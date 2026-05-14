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

func (c *CLI) handleMislabeledJSONLines(cmd *cobra.Command, resp *http.Response, prepared *preparedRequest, spec printSpec) (bool, error) {
	gf := globalFlagsFromContext(requestContext(cmd))
	if gf.MaxItems <= 0 {
		return false, nil
	}
	base, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if base != "application/json" || resp.Body == nil {
		return false, nil
	}
	if contentEncodingApplied(resp.Header.Get("Content-Encoding")) {
		body, err := c.decompressedResponseBody(resp)
		if err != nil {
			_ = resp.Body.Close()
			return true, fmt.Errorf("decompressing response: %w", err)
		}
		resp.Body = body
		resp.Header.Del("Content-Encoding")
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
	return true, c.handleNDJSON(cmd, resp, prepared, spec)
}

func contentEncodingApplied(encoding string) bool {
	encoding = strings.TrimSpace(encoding)
	return encoding != "" && !strings.EqualFold(encoding, "identity")
}

type readCloser struct {
	io.Reader
	io.Closer
}

// handleSSE reads a text/event-stream response body, emitting each event's
// data to stdout as it arrives. Filter and output flags are applied per event.
func (c *CLI) handleSSE(cmd *cobra.Command, resp *http.Response, prepared *preparedRequest, spec printSpec) (retErr error) {
	defer resp.Body.Close()

	if !spec.includesResponseBody() {
		return c.runPrintSpec(cmd, streamBaseResponse(resp), prepared, spec, nil)
	}

	if err := validateStreamingOutputMode(cmd); err != nil {
		return err
	}

	gf := globalFlagsFromContext(requestContext(cmd))
	base := streamBaseResponse(resp)
	return c.runPrintSpec(cmd, base, prepared, spec, func() (err error) {
		if collectStreamingJSON(gf) {
			return c.collectSSE(cmd, resp)
		}
		renderer, err := c.newValueRendererWithPrint(cmd, base, gf.Filter != "", spec)
		if err != nil {
			return err
		}
		defer func() {
			if closeErr := renderer.Close(); err == nil && closeErr != nil {
				err = closeErr
			}
		}()

		return c.readSSEItems(cmd, resp.Body, gf.Filter != "", gf.MaxItems, func(item streamItem) error {
			return c.renderStreamValue(cmd, renderer, item.value, item.parsedJSON)
		})
	})
}

func parseJSONOrString(data string) (any, bool) {
	var parsed any
	if err := json.Unmarshal([]byte(data), &parsed); err == nil {
		return parsed, true
	}
	return data, false
}

type streamItem struct {
	value      any
	parsedJSON bool
}

func sseStreamItem(data, eventName, eventID string, retryMs int, wrapData bool) streamItem {
	parsed, parsedJSON := parseJSONOrString(data)
	item := parsed
	itemParsedJSON := parsedJSON
	if wrapData || eventName != "" || eventID != "" || retryMs != 0 {
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
	return streamItem{value: item, parsedJSON: itemParsedJSON}
}

func (c *CLI) readSSEItems(cmd *cobra.Command, r io.Reader, wrapData bool, maxItems int, emit func(streamItem) error) error {
	scanner := bufio.NewScanner(r)
	lineLimit := maxStreamLineBytes(cmd)
	eventLimit := maxSSEEventBytes(cmd)
	scanner.Buffer(make([]byte, 64*1024), lineLimit)

	var eventName string
	var eventID string
	var retryMs int
	var data strings.Builder
	count := 0
	stoppedByMax := false

	flush := func() error {
		if data.Len() == 0 && eventName == "" && eventID == "" && retryMs == 0 {
			return nil
		}
		item := sseStreamItem(data.String(), eventName, eventID, retryMs, wrapData)
		if err := emit(item); err != nil {
			return err
		}
		count++
		eventName = ""
		eventID = ""
		retryMs = 0
		data.Reset()
		if maxItems > 0 && count >= maxItems {
			stoppedByMax = true
		}
		return nil
	}

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if line == "" {
			if err := flush(); err != nil {
				return err
			}
			if stoppedByMax {
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

	if err := scanner.Err(); err != nil && !stoppedByMax {
		if strings.Contains(err.Error(), "token too long") {
			return fmt.Errorf("SSE stream line exceeds %d bytes", lineLimit)
		}
		return fmt.Errorf("SSE stream error: %w", err)
	}
	if !stoppedByMax {
		if err := flush(); err != nil {
			return err
		}
	}
	if stoppedByMax {
		c.warnf("streaming stopped at --rsh-max-items=%d; pass 0 for unlimited", maxItems)
	}
	return nil
}

func (c *CLI) readNDJSONItems(cmd *cobra.Command, r io.Reader, maxItems int, emit func(streamItem) error) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), maxStreamLineBytes(cmd))
	count := 0
	stoppedByMax := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		item, parsedJSON := parseJSONOrString(line)
		if err := emit(streamItem{value: item, parsedJSON: parsedJSON}); err != nil {
			return err
		}
		count++
		if maxItems > 0 && count >= maxItems {
			stoppedByMax = true
			break
		}
	}

	if err := scanner.Err(); err != nil && !stoppedByMax {
		return fmt.Errorf("NDJSON stream error: %w", err)
	}
	if stoppedByMax {
		c.warnf("streaming stopped at --rsh-max-items=%d; pass 0 for unlimited", maxItems)
	}
	return nil
}

func (c *CLI) collectNDJSON(cmd *cobra.Command, resp *http.Response) error {
	gf := globalFlagsFromContext(requestContext(cmd))
	items := make([]any, 0, gf.MaxItems)
	if err := c.readNDJSONItems(cmd, resp.Body, gf.MaxItems, func(item streamItem) error {
		value, err := c.filterBodyValue(cmd, item.value)
		if err != nil {
			return err
		}
		items = append(items, value)
		return nil
	}); err != nil {
		return err
	}
	return c.renderValue(cmd, items, false)
}

func (c *CLI) collectSSE(cmd *cobra.Command, resp *http.Response) error {
	gf := globalFlagsFromContext(requestContext(cmd))
	items := make([]any, 0, gf.MaxItems)
	if err := c.readSSEItems(cmd, resp.Body, gf.Filter != "", gf.MaxItems, func(item streamItem) error {
		value, err := c.filterBodyValue(cmd, item.value)
		if err != nil {
			return err
		}
		items = append(items, value)
		return nil
	}); err != nil {
		return err
	}
	return c.renderValue(cmd, items, false)
}

// handleNDJSON reads a newline-delimited JSON response body, emitting each
// line to stdout as it arrives. Filter and output flags are applied per line.
func (c *CLI) handleNDJSON(cmd *cobra.Command, resp *http.Response, prepared *preparedRequest, spec printSpec) (retErr error) {
	defer resp.Body.Close()

	if !spec.includesResponseBody() {
		return c.runPrintSpec(cmd, streamBaseResponse(resp), prepared, spec, nil)
	}

	if err := validateStreamingOutputMode(cmd); err != nil {
		return err
	}

	gf := globalFlagsFromContext(requestContext(cmd))
	base := streamBaseResponse(resp)
	return c.runPrintSpec(cmd, base, prepared, spec, func() (err error) {
		if collectStreamingJSON(gf) {
			return c.collectNDJSON(cmd, resp)
		}
		renderer, err := c.newValueRendererWithPrint(cmd, base, gf.Filter != "", spec)
		if err != nil {
			return err
		}
		defer func() {
			if closeErr := renderer.Close(); err == nil && closeErr != nil {
				err = closeErr
			}
		}()

		return c.readNDJSONItems(cmd, resp.Body, gf.MaxItems, func(item streamItem) error {
			return c.renderStreamValue(cmd, renderer, item.value, item.parsedJSON)
		})
	})
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

	result, err := c.filterBodyValue(cmd, item)
	if err != nil {
		return err
	}

	tty := c.stdoutIsTerminal()
	if fmtName == "" && tty {
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
	gf := globalFlagsFromContext(requestContext(cmd))
	if gf.OutputFormat == "json" && !collectStreamingJSON(gf) {
		return fmt.Errorf("-o json for stream responses requires --rsh-collect and --rsh-max-items N. Try -o ndjson for record-by-record JSON output.")
	}
	return nil
}

func collectStreamingJSON(gf GlobalFlags) bool {
	return gf.OutputFormat == "json" && gf.Collect && gf.MaxItems > 0
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
