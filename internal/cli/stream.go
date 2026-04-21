package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/rest-sh/restish/v2/internal/filter"
	"github.com/rest-sh/restish/v2/internal/output"
	"github.com/spf13/cobra"
)

const (
	mimeSSE    = "text/event-stream"
	mimeNDJSON = "application/x-ndjson"
	mimeJSONL  = "application/jsonlines"
)

// streamingContentType returns the stream kind ("sse" or "ndjson") when ct
// identifies a streaming format, and "" otherwise.
func streamingContentType(ct string) string {
	base, _, _ := mime.ParseMediaType(ct)
	switch base {
	case mimeSSE:
		return "sse"
	case mimeNDJSON, mimeJSONL:
		return "ndjson"
	}
	return ""
}

// handleSSE reads a text/event-stream response body, emitting each event's
// data to stdout as it arrives. Filter and output flags are applied per event.
func (c *CLI) handleSSE(cmd *cobra.Command, resp *http.Response) (retErr error) {
	defer resp.Body.Close()

	if err := validateStreamingOutputMode(cmd); err != nil {
		return err
	}

	gfSSE := globalFlagsFromContext(requestContext(cmd))
	maxEvents := gfSSE.MaxEvents
	filterExpr := gfSSE.Filter
	renderer, err := c.newValueRenderer(cmd, streamBaseResponse(resp))
	if err != nil {
		return err
	}
	defer func() {
		if err := renderer.Close(); retErr == nil && err != nil {
			retErr = err
		}
	}()

	reader := bufio.NewReader(resp.Body)
	var eventName string
	var eventID string
	var retryMs int
	var data strings.Builder
	count := 0

	flush := func() error {
		if data.Len() == 0 && eventName == "" && eventID == "" && retryMs == 0 {
			return nil
		}
		parsed, parsedJSON := parseJSONOrString(data.String())
		item := parsed
		itemParsedJSON := parsedJSON
		if filterExpr != "" || eventName != "" || eventID != "" || retryMs != 0 {
			event := map[string]any{"data": parsed}
			if eventName != "" {
				event["event"] = eventName
			}
			if eventID != "" {
				event["id"] = eventID
			}
			if retryMs != 0 {
				event["retry"] = retryMs
			}
			item = event
			itemParsedJSON = true
		}
		if err := c.renderStreamValue(cmd, renderer, item, itemParsedJSON); err != nil {
			return err
		}
		count++
		eventName = ""
		eventID = ""
		retryMs = 0
		data.Reset()
		return nil
	}

	for {
		line, readErr := reader.ReadString('\n')
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			fmt.Fprintf(c.Stderr, "warning: SSE stream error: %v\n", readErr)
			return nil
		}
		line = strings.TrimRight(line, "\r\n")

		if line == "" {
			// Blank line terminates an event.
			if err := flush(); err != nil {
				return err
			}
			if maxEvents > 0 && count >= maxEvents {
				break
			}
			if errors.Is(readErr, io.EOF) {
				break
			}
			continue
		}

		field, value, hasColon := strings.Cut(line, ":")
		if hasColon {
			value = strings.TrimPrefix(value, " ")
		}
		switch field {
		case "data":
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(value)
		case "event":
			eventName = value
		case "id":
			eventID = value
		case "retry":
			var retry int
			if _, err := fmt.Sscanf(value, "%d", &retry); err == nil {
				retryMs = retry
			}
		}

		if errors.Is(readErr, io.EOF) {
			if err := flush(); err != nil {
				return err
			}
			break
		}
	}

	return nil
}

func parseJSONOrString(data string) (any, bool) {
	var parsed any
	if err := json.Unmarshal([]byte(data), &parsed); err == nil {
		return parsed, true
	}
	return data, false
}

func (c *CLI) formatStreamItem(cmd *cobra.Command, renderer valueRenderer, data string) error {
	item, parsedJSON := parseJSONOrString(data)
	return c.renderStreamValue(cmd, renderer, item, parsedJSON)
}

// handleNDJSON reads a newline-delimited JSON response body, emitting each
// line to stdout as it arrives. Filter and output flags are applied per line.
func (c *CLI) handleNDJSON(cmd *cobra.Command, resp *http.Response) (retErr error) {
	defer resp.Body.Close()

	if err := validateStreamingOutputMode(cmd); err != nil {
		return err
	}

	maxEvents := globalFlagsFromContext(requestContext(cmd)).MaxEvents
	renderer, err := c.newValueRenderer(cmd, streamBaseResponse(resp))
	if err != nil {
		return err
	}
	defer func() {
		if err := renderer.Close(); retErr == nil && err != nil {
			retErr = err
		}
	}()

	scanner := bufio.NewScanner(resp.Body)
	// Increase the max token size to 1 MiB to accommodate large NDJSON lines.
	scanner.Buffer(make([]byte, 64*1024), 1*1024*1024)
	count := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if err := c.formatStreamItem(cmd, renderer, line); err != nil {
			return err
		}
		count++
		if maxEvents > 0 && count >= maxEvents {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(c.Stderr, "warning: NDJSON stream error: %v\n", err)
	}
	return nil
}

// renderStreamValue applies the active filter to one streamed item and renders
// the result using the shared body/sub-value output path.
func (c *CLI) renderStreamValue(cmd *cobra.Command, renderer valueRenderer, item any, parsedJSON bool) error {
	gf := globalFlagsFromContext(requestContext(cmd))
	filterExpr := gf.Filter
	fmtName := gf.OutputFormat

	result := item
	if filterExpr != "" {
		doc := map[string]any{"body": item}
		filtered, handled, err := c.filterOutput(cmd, filterExpr, doc, filter.LangAuto)
		if err != nil {
			return err
		}
		if handled {
			return nil
		}
		result = filtered
	}

	tty := output.IsTerminal(c.Stdout)
	if fmtName == "readable" || (fmtName == "" && tty) {
		if !parsedJSON {
			return c.writeRaw(result)
		}
	}

	return renderer.Render(result)
}

func validateStreamingOutputMode(cmd *cobra.Command) error {
	fmtName := globalFlagsFromContext(requestContext(cmd)).OutputFormat
	if fmtName == "json" {
		return fmt.Errorf("output format %q requires a complete bounded result; use -o ndjson for streaming records", fmtName)
	}
	return nil
}

func streamBaseResponse(resp *http.Response) *output.Response {
	headers := make(map[string]string, len(resp.Header))
	for k, vals := range resp.Header {
		if len(vals) > 0 {
			headers[k] = vals[0]
		}
	}
	return &output.Response{
		Proto:   resp.Proto,
		Status:  resp.StatusCode,
		Headers: headers,
	}
}
