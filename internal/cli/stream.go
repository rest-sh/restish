package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/rest-sh/restish/v2/internal/filter"
	"github.com/rest-sh/restish/v2/internal/output"
	"github.com/spf13/cobra"
)

const (
	mimeSSE       = "text/event-stream"
	mimeNDJSON    = "application/x-ndjson"
	mimeNDJSONAlt = "application/ndjson"
	mimeJSONLAlt  = "application/jsonl"
	mimeJSONL     = "application/jsonlines"
)

// streamingContentType returns the stream kind ("sse" or "ndjson") when ct
// identifies a streaming format, and "" otherwise.
func streamingContentType(ct string) string {
	base, _, _ := mime.ParseMediaType(ct)
	switch base {
	case mimeSSE:
		return "sse"
	case mimeNDJSON, mimeNDJSONAlt, mimeJSONLAlt, mimeJSONL:
		return "ndjson"
	}
	return ""
}

func (c *CLI) handleMislabeledJSONLines(cmd *cobra.Command, resp *http.Response) (bool, error) {
	gf := globalFlagsFromContext(requestContext(cmd))
	if gf.MaxItems <= 0 {
		return false, nil
	}
	base, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if base != "application/json" || resp.Body == nil {
		return false, nil
	}

	original := resp.Body
	reader := bufio.NewReader(original)
	line, err := reader.ReadSlice('\n')
	if err != nil {
		resp.Body = &readCloser{Reader: io.MultiReader(bytes.NewReader(line), reader), Closer: original}
		return false, nil
	}
	lineCopy := append([]byte(nil), line...)
	trimmed := strings.TrimSpace(string(lineCopy))
	if !strings.HasPrefix(trimmed, "{") {
		resp.Body = &readCloser{Reader: io.MultiReader(bytes.NewReader(lineCopy), reader), Closer: original}
		return false, nil
	}
	if _, parsedJSON := parseJSONOrString(trimmed); !parsedJSON {
		resp.Body = &readCloser{Reader: io.MultiReader(bytes.NewReader(lineCopy), reader), Closer: original}
		return false, nil
	}
	resp.Body = &readCloser{Reader: io.MultiReader(bytes.NewReader(lineCopy), reader), Closer: original}
	if err := c.statusError(cmd, resp.StatusCode); err != nil {
		_ = resp.Body.Close()
		return true, err
	}
	return true, c.handleNDJSON(cmd, resp)
}

type readCloser struct {
	io.Reader
	io.Closer
}

// handleSSE reads a text/event-stream response body, emitting each event's
// data to stdout as it arrives. Filter and output flags are applied per event.
func (c *CLI) handleSSE(cmd *cobra.Command, resp *http.Response) (retErr error) {
	defer resp.Body.Close()

	if err := validateStreamingOutputMode(cmd); err != nil {
		return err
	}

	gfSSE := globalFlagsFromContext(requestContext(cmd))
	maxItems := gfSSE.MaxItems
	filterExpr := gfSSE.Filter
	renderer, err := c.newValueRenderer(cmd, streamBaseResponse(resp), filterExpr != "")
	if err != nil {
		return err
	}
	defer func() {
		if err := renderer.Close(); retErr == nil && err != nil {
			retErr = err
		}
	}()

	scanner := bufio.NewScanner(resp.Body)
	lineLimit := maxStreamLineBytes(cmd)
	eventLimit := maxSSEEventBytes(cmd)
	scanner.Buffer(make([]byte, 64*1024), lineLimit)
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

	stoppedByMax := false
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if line == "" {
			// Blank line terminates an event.
			if err := flush(); err != nil {
				return err
			}
			if maxItems > 0 && count >= maxItems {
				stoppedByMax = true
				break
			}
			continue
		}

		if strings.HasPrefix(line, ":") {
			continue
		}
		field, value, hasColon := strings.Cut(line, ":")
		if hasColon {
			value = strings.TrimPrefix(value, " ")
		} else {
			value = ""
		}
		switch field {
		case "data":
			nextLen := data.Len() + len(value)
			if data.Len() > 0 {
				nextLen++
			}
			if nextLen > eventLimit {
				return fmt.Errorf("SSE event data exceeds %d bytes", eventLimit)
			}
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(value)
		case "event":
			eventName = value
		case "id":
			eventID = value
		case "retry":
			if retry, err := strconv.Atoi(value); err == nil {
				retryMs = retry
			}
		}
	}

	if err := scanner.Err(); err != nil {
		if stoppedByMax {
			c.warnf("streaming stopped at --rsh-max-items=%d; pass 0 for unlimited", maxItems)
			return nil
		}
		if strings.Contains(err.Error(), "token too long") {
			return fmt.Errorf("SSE stream line exceeds %d bytes", lineLimit)
		}
		return fmt.Errorf("SSE stream error: %w", err)
	}
	if !stoppedByMax {
		if err := flush(); err != nil {
			return err
		}
	} else {
		c.warnf("streaming stopped at --rsh-max-items=%d; pass 0 for unlimited", maxItems)
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

	gf := globalFlagsFromContext(requestContext(cmd))
	maxItems := gf.MaxItems
	renderer, err := c.newValueRenderer(cmd, streamBaseResponse(resp), gf.Filter != "")
	if err != nil {
		return err
	}
	defer func() {
		if err := renderer.Close(); retErr == nil && err != nil {
			retErr = err
		}
	}()

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), maxStreamLineBytes(cmd))
	count := 0
	stoppedByMax := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if err := c.formatStreamItem(cmd, renderer, line); err != nil {
			return err
		}
		count++
		if maxItems > 0 && count >= maxItems {
			stoppedByMax = true
			break
		}
	}

	if err := scanner.Err(); err != nil {
		if stoppedByMax {
			c.warnf("streaming stopped at --rsh-max-items=%d; pass 0 for unlimited", maxItems)
			return nil
		}
		return fmt.Errorf("NDJSON stream error: %w", err)
	}
	if stoppedByMax {
		c.warnf("streaming stopped at --rsh-max-items=%d; pass 0 for unlimited", maxItems)
	}
	return nil
}

func maxStreamLineBytes(cmd *cobra.Command) int {
	maxBytes := maxBodyBytes(cmd)
	if maxBytes <= 0 {
		maxBytes = output.DefaultMaxBodyBytes
	}
	maxInt := int64(int(^uint(0) >> 1))
	if maxBytes > maxInt {
		return int(maxInt)
	}
	return int(maxBytes)
}

func maxSSEEventBytes(cmd *cobra.Command) int {
	return maxStreamLineBytes(cmd)
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
		lang := resolveFilterLang(gf.FilterLang)
		filterResult, err := filter.ApplyWithInfo(filterExpr, doc, lang)
		if err != nil {
			return fmt.Errorf("filter: %w", err)
		}
		traceFilter(requestTraceFromContext(requestContext(cmd)), lang, filterResult.Lang)
		result = filterResult.Value
	}

	tty := output.IsTerminal(c.Stdout)
	if fmtName == "readable" || (fmtName == "" && tty) {
		if !parsedJSON {
			c.traceValueOutput(cmd, result, false)
			if trace := requestTraceFromContext(requestContext(cmd)); trace != nil {
				trace.RenderAfter(c.Stderr, gf.Verbose)
			}
			if err := c.writePlainValue(result); err != nil {
				return err
			}
			return c.flushStdout()
		}
	}

	c.traceValueOutput(cmd, result, filterExpr != "")
	if trace := requestTraceFromContext(requestContext(cmd)); trace != nil {
		trace.RenderAfter(c.Stderr, gf.Verbose)
	}
	if err := renderer.Render(result); err != nil {
		return err
	}
	return c.flushStdout()
}

func validateStreamingOutputMode(cmd *cobra.Command) error {
	fmtName := globalFlagsFromContext(requestContext(cmd)).OutputFormat
	if fmtName == "json" {
		return fmt.Errorf("-o json cannot be used with an unbounded stream response. Try -o ndjson for record-by-record JSON output.")
	}
	return nil
}

func streamBaseResponse(resp *http.Response) *output.Response {
	headers := make(map[string][]string, len(resp.Header))
	for k, vals := range resp.Header {
		if len(vals) > 0 {
			headers[k] = append([]string(nil), vals...)
		}
	}
	return &output.Response{
		Proto:   resp.Proto,
		Status:  resp.StatusCode,
		Headers: headers,
	}
}
