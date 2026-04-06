package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/danielgtaylor/restish/v2/plugin"
	"github.com/fxamacker/cbor/v2"
)

type formatterRequest struct {
	Type     string            `cbor:"type"`
	Format   string            `cbor:"format"`
	Response formatterResponse `cbor:"response"`
}

type formatterResponse struct {
	Body any `cbor:"body"`
}

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "--rsh-plugin-manifest" {
			writeManifest()
			return
		}
	}

	var req formatterRequest
	if err := plugin.ReadMessage(os.Stdin, &req); err != nil {
		fail(fmt.Errorf("read request: %w", err))
	}
	if req.Type != "formatter" {
		fail(fmt.Errorf("expected formatter request, got %q", req.Type))
	}
	if req.Format != "csv" {
		fail(fmt.Errorf("unsupported formatter %q", req.Format))
	}
	if err := FormatCSV(os.Stdout, req.Response.Body); err != nil {
		fail(err)
	}
}

func writeManifest() {
	data, err := cbor.Marshal(map[string]any{
		"name":                "csv",
		"version":             "1.0.0",
		"description":         "Formatter plugin that renders array-shaped responses as CSV",
		"restish_api_version": 1,
		"hooks":               []string{"formatter"},
		"formatter_names":     []string{"csv"},
	})
	if err != nil {
		fail(fmt.Errorf("marshal manifest: %w", err))
	}
	if _, err := os.Stdout.Write(data); err != nil {
		fail(fmt.Errorf("write manifest: %w", err))
	}
}

func FormatCSV(w io.Writer, body any) error {
	rows, err := csvRows(body)
	if err != nil {
		return err
	}

	headers := collectHeaders(rows)
	writer := csv.NewWriter(w)

	if len(headers) > 0 {
		if err := writer.Write(headers); err != nil {
			return fmt.Errorf("write header: %w", err)
		}
	}

	for i, row := range rows {
		record := make([]string, len(headers))
		for j, header := range headers {
			record[j] = csvCell(row[header])
		}
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("write row %d: %w", i, err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("flush csv: %w", err)
	}
	return nil
}

func csvRows(body any) ([]map[string]any, error) {
	items, ok := body.([]any)
	if !ok {
		return nil, fmt.Errorf("csv formatter expects response body to be an array of objects")
	}

	rows := make([]map[string]any, 0, len(items))
	for i, item := range items {
		row, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("csv formatter expects each array item to be an object (row %d)", i)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func collectHeaders(rows []map[string]any) []string {
	seen := map[string]struct{}{}
	for _, row := range rows {
		for key := range row {
			seen[key] = struct{}{}
		}
	}

	headers := make([]string, 0, len(seen))
	for key := range seen {
		headers = append(headers, key)
	}
	sort.Strings(headers)
	return headers
}

func csvCell(v any) string {
	switch data := v.(type) {
	case nil:
		return ""
	case string:
		return data
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(encoded)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
