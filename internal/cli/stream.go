package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"strings"

	"github.com/danielgtaylor/restish/v2/internal/filter"
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
func (c *CLI) handleSSE(cmd *cobra.Command, resp *http.Response) error {
	defer resp.Body.Close()

	maxEvents, _ := cmd.Flags().GetInt("rsh-max-events")
	scanner := bufio.NewScanner(resp.Body)
	// Increase the max token size to 1 MiB to accommodate large SSE payloads.
	// The default 64 KiB limit causes silent data loss on large events.
	scanner.Buffer(make([]byte, 64*1024), 1*1024*1024)

	var dataLines []string
	count := 0

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			// Blank line terminates an event.
			if len(dataLines) > 0 {
				data := strings.Join(dataLines, "\n")
				if err := c.formatStreamItem(cmd, data); err != nil {
					return err
				}
				count++
				dataLines = dataLines[:0]
				if maxEvents > 0 && count >= maxEvents {
					break
				}
			}
			continue
		}

		field, value, _ := strings.Cut(line, ":")
		value = strings.TrimPrefix(value, " ")
		switch field {
		case "data":
			dataLines = append(dataLines, value)
		case "retry":
			// Reconnect delay hint; parsed but reconnect is not implemented.
		case "id", "event":
			// Ignored for now.
		}
	}

	// Flush a final event if the stream ended without a trailing blank line.
	if len(dataLines) > 0 && (maxEvents <= 0 || count < maxEvents) {
		data := strings.Join(dataLines, "\n")
		if err := c.formatStreamItem(cmd, data); err != nil {
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		// Surface unexpected I/O errors on stderr so users can distinguish a
		// network interruption from a clean end-of-stream.
		fmt.Fprintf(c.Stderr, "warning: SSE stream error: %v\n", err)
	}
	return nil
}

// handleNDJSON reads a newline-delimited JSON response body, emitting each
// line to stdout as it arrives. Filter and output flags are applied per line.
func (c *CLI) handleNDJSON(cmd *cobra.Command, resp *http.Response) error {
	defer resp.Body.Close()

	maxEvents, _ := cmd.Flags().GetInt("rsh-max-events")
	scanner := bufio.NewScanner(resp.Body)
	// Increase the max token size to 1 MiB to accommodate large NDJSON lines.
	scanner.Buffer(make([]byte, 64*1024), 1*1024*1024)
	count := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if err := c.formatStreamItem(cmd, line); err != nil {
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

// formatStreamItem parses data as JSON (if possible), applies the -f filter,
// and writes the result to stdout.
func (c *CLI) formatStreamItem(cmd *cobra.Command, data string) error {
	filterExpr, _ := cmd.Flags().GetString("rsh-filter")
	rawMode, _ := cmd.Flags().GetBool("rsh-raw")

	// Try to parse as JSON; fall back to string.
	var item any = data
	var parsed any
	if err := json.Unmarshal([]byte(data), &parsed); err == nil {
		item = parsed
	}

	// Apply filter.
	result := item
	if filterExpr != "" {
		doc := map[string]any{"body": item}
		filtered, err := filter.Apply(filterExpr, doc, filter.LangAuto)
		if err != nil {
			return fmt.Errorf("filter: %w", err)
		}
		result = filtered
	}

	if rawMode {
		return c.writeRaw(result)
	}

	b, err := json.Marshal(result)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.Stdout, "%s\n", b)
	return nil
}
