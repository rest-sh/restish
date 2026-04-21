package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/rest-sh/restish/v2/internal/config"
	"github.com/rest-sh/restish/v2/internal/filter"
	"github.com/rest-sh/restish/v2/internal/output"
	"github.com/rest-sh/restish/v2/internal/request"
	"github.com/spf13/cobra"
)

// tryPaginate checks if auto-pagination should run for this response.
// Returns (true, err) when pagination handled output (caller should not format).
// Returns (false, nil) when pagination is disabled or no next link found.
func (c *CLI) tryPaginate(
	cmd *cobra.Command,
	firstResp *output.Response,
	firstURL string,
	opts request.Options,
	pagCfg *config.PaginationConfig,
) (bool, error) {
	noPaginate := globalFlagsFromContext(requestContext(cmd)).NoPaginate
	if noPaginate {
		return false, nil
	}

	nextURL := resolveNextURL(firstResp, pagCfg)
	if nextURL == "" {
		return false, nil
	}

	gfPag := globalFlagsFromContext(requestContext(cmd))
	collect := gfPag.Collect
	maxPages := gfPag.MaxPages
	maxItems := gfPag.MaxItems

	return true, c.runPagination(cmd, firstResp, firstURL, nextURL, opts, pagCfg, collect, maxPages, maxItems)
}

// runPagination drives the pagination loop starting from firstResp.
func (c *CLI) runPagination(
	cmd *cobra.Command,
	firstResp *output.Response,
	firstURL string,
	firstNextURL string,
	opts request.Options,
	pagCfg *config.PaginationConfig,
	collect bool,
	maxPages, maxItems int,
) (retErr error) {
	ctx := requestContext(cmd)
	if !output.IsTerminal(c.Stdout) {
		origStdout := c.Stdout
		c.Stdout = contextWriter{ctx: ctx, writer: origStdout}
		defer func() { c.Stdout = origStdout }()
	}

	var allItems []any
	var renderer valueRenderer
	streamItems, err := c.paginationStreamsItems(cmd, firstResp)
	if err != nil {
		return err
	}

	if !collect && streamItems {
		renderer, err = c.newPaginatedValueRenderer(cmd, firstResp, pagCfg)
		if err != nil {
			return err
		}
		defer func() {
			if err := renderer.Close(); retErr == nil && err != nil {
				retErr = err
			}
		}()
	}

	// Process first page.
	items, filterErr := pageItems(firstResp.Body, pagCfg)
	if filterErr != nil {
		fmt.Fprintf(c.Stderr, "warning: pagination items_path: %v\n", filterErr)
	}
	if collect || !streamItems {
		allItems = make([]any, 0, paginationItemCapacity(len(items), maxPages, maxItems))
	}
	items, done := applyItemLimits(items, allItems, maxItems)
	if collect || !streamItems {
		allItems = append(allItems, items...)
	} else {
		if err := c.streamItems(ctx, cmd, renderer, items); err != nil {
			return err
		}
	}

	nextURL := firstNextURL
	page := 1
	visited := map[string]int{firstURL: 1}
	stderrIsTTY := output.IsTerminal(c.Stderr)

	for !done && nextURL != "" {
		if err := ctx.Err(); err != nil {
			return err
		}
		// Safety: max pages check (page is 1-indexed, firstResp is page 1).
		if maxPages > 0 && page >= maxPages {
			fmt.Fprintf(c.Stderr, "warning: reached --rsh-max-pages limit (%d); stopping pagination\n", maxPages)
			break
		}
		if seenPage, ok := visited[nextURL]; ok {
			fmt.Fprintf(c.Stderr, "warning: pagination cycle detected at page %d URL %q; stopping\n", seenPage, nextURL)
			break
		}
		page++
		visited[nextURL] = page

		// Progress feedback on TTY stderr.
		if stderrIsTTY {
			fmt.Fprintf(c.Stderr, "\rfetching page %d...\x1b[K", page)
		}

		httpResp, err := request.Do(ctx, "GET", nextURL, nil, opts)
		if err != nil {
			return fmt.Errorf("paginate page %d: %w", page, err)
		}

		resp, err := c.normalizeHTTPResponse(httpResp, maxBodyBytes(cmd))
		if err != nil {
			return fmt.Errorf("paginate page %d normalize: %w", page, err)
		}

		items, filterErr = pageItems(resp.Body, pagCfg)
		if filterErr != nil {
			fmt.Fprintf(c.Stderr, "warning: pagination items_path: %v\n", filterErr)
		}
		items, done = applyItemLimits(items, allItems, maxItems)
		if collect || !streamItems {
			allItems = append(allItems, items...)
		} else {
			if err := c.streamItems(ctx, cmd, renderer, items); err != nil {
				return err
			}
		}
		nextURL = resolveNextURL(resp, pagCfg)
	}

	// Erase progress line on TTY.
	if stderrIsTTY {
		fmt.Fprintf(c.Stderr, "\r\x1b[K")
	}

	if done && maxItems > 0 {
		fmt.Fprintf(c.Stderr, "warning: reached --rsh-max-items limit (%d); stopping pagination\n", maxItems)
	}

	if collect || !streamItems {
		synthetic := buildPaginatedResponse(firstResp, pagCfg, allItems)
		if err := ctx.Err(); err != nil {
			return err
		}
		return c.formatResponse(cmd, synthetic)
	}
	return nil
}

type contextWriter struct {
	ctx    context.Context
	writer io.Writer
}

func (w contextWriter) Write(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := w.writer.Write(p)
	if err != nil {
		return n, err
	}
	if err := w.ctx.Err(); err != nil {
		return n, err
	}
	return n, nil
}

func paginationItemCapacity(firstPageItems, maxPages, maxItems int) int {
	if maxItems > 0 && firstPageItems >= maxItems {
		return maxItems
	}
	capacity := firstPageItems
	if maxPages > 0 && firstPageItems > 0 {
		capacity = firstPageItems * maxPages
	}
	if maxItems > 0 && (capacity == 0 || capacity > maxItems) {
		return maxItems
	}
	return capacity
}

func (c *CLI) newPaginatedValueRenderer(cmd *cobra.Command, base *output.Response, pagCfg *config.PaginationConfig) (valueRenderer, error) {
	fmtName := globalFlagsFromContext(requestContext(cmd)).OutputFormat
	tty := output.IsTerminal(c.Stdout)
	if !tty || (fmtName != "" && fmtName != "readable") {
		return c.newValueRenderer(cmd, base)
	}

	formatter, err := c.selectFormatter(cmd, fmtName, tty, "")
	if err != nil {
		return nil, err
	}
	framed, ok := formatter.(output.FramedValueStreamFormatter)
	if !ok {
		return c.newValueRenderer(cmd, base)
	}

	frame, ok := paginatedReadableFrame(base.Body, pagCfg)
	if !ok {
		return c.newValueRenderer(cmd, base)
	}

	stream, err := framed.StartFramedValueStream(c.Stdout, base, output.ColorEnabled(c.Stdout), frame)
	if err != nil {
		return nil, err
	}
	return valueStreamRenderer{stream: stream}, nil
}

func (c *CLI) paginationStreamsItems(cmd *cobra.Command, base *output.Response) (bool, error) {
	gf := globalFlagsFromContext(requestContext(cmd))
	if gf.Silent {
		return true, nil
	}
	if gf.Raw {
		return true, nil
	}

	fmtName := gf.OutputFormat
	tty := output.IsTerminal(c.Stdout)

	// Default non-TTY pagination should preserve a single valid JSON document.
	if !tty && fmtName == "" {
		return false, nil
	}
	// JSON is always a document format and therefore never streams items.
	if fmtName == "json" {
		return false, nil
	}
	// Explicit readable only streams incrementally on an actual TTY.
	if fmtName == "readable" && !tty {
		return false, nil
	}

	formatter, err := c.selectFormatter(cmd, fmtName, tty, "")
	if err != nil {
		return false, err
	}
	_, ok := formatter.(output.ValueStreamFormatter)
	return ok, nil
}

// streamItems renders each item using the shared streamed-item filter/render
// path used by pagination and event streams.
func (c *CLI) streamItems(ctx context.Context, cmd *cobra.Command, renderer valueRenderer, items []any) error {
	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := c.renderStreamValue(cmd, renderer, item, true); err != nil {
			return err
		}
	}
	return nil
}

func buildPaginatedResponse(firstResp *output.Response, pagCfg *config.PaginationConfig, items []any) *output.Response {
	return &output.Response{
		Proto:   firstResp.Proto,
		Status:  firstResp.Status,
		Headers: firstResp.Headers,
		Links:   firstResp.Links,
		Body:    mergePaginatedBody(firstResp.Body, pagCfg, items),
	}
}

const paginatedItemsPlaceholder = "__restish_paginated_items_placeholder__"

func paginatedReadableFrame(firstBody any, pagCfg *config.PaginationConfig) (output.FramedValueTemplate, bool) {
	templateBody, ok := paginatedFrameTemplateBody(firstBody, pagCfg)
	if !ok {
		return output.FramedValueTemplate{}, false
	}

	data, err := json.MarshalIndent(templateBody, "", "  ")
	if err != nil {
		return output.FramedValueTemplate{}, false
	}

	token := `"` + paginatedItemsPlaceholder + `"`
	rendered := string(data)
	idx := strings.Index(rendered, token)
	if idx < 0 {
		return output.FramedValueTemplate{}, false
	}

	lineStart := strings.LastIndex(rendered[:idx], "\n") + 1
	closeIndent := leadingWhitespace(rendered[lineStart:idx])
	return output.FramedValueTemplate{
		Prefix:      rendered[:idx],
		Suffix:      rendered[idx+len(token):],
		ItemIndent:  closeIndent + "  ",
		CloseIndent: closeIndent,
	}, true
}

func paginatedFrameTemplateBody(firstBody any, pagCfg *config.PaginationConfig) (any, bool) {
	if pagCfg == nil || pagCfg.ItemsPath == "" {
		return paginatedItemsPlaceholder, true
	}

	clone, ok := cloneJSONPath(firstBody, pagCfg.ItemsPath)
	if !ok {
		return nil, false
	}
	updated, ok := setSimplePath(clone, pagCfg.ItemsPath, paginatedItemsPlaceholder)
	if !ok {
		return nil, false
	}
	return updated, true
}

func leadingWhitespace(s string) string {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return s[:i]
}

func mergePaginatedBody(firstBody any, pagCfg *config.PaginationConfig, items []any) any {
	if pagCfg == nil || pagCfg.ItemsPath == "" {
		return items
	}

	clone, ok := cloneJSONPath(firstBody, pagCfg.ItemsPath)
	if !ok {
		return items
	}

	if updated, ok := setSimplePath(clone, pagCfg.ItemsPath, items); ok {
		return updated
	}
	return items
}

func cloneJSONPath(value any, targetPath string) (any, bool) {
	trimmed := strings.TrimPrefix(targetPath, ".")
	if trimmed == "" {
		return value, true
	}
	parts := strings.Split(trimmed, ".")
	root, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	for _, part := range parts {
		if part == "" || strings.ContainsAny(part, "[]{}()|=<>!@") {
			return nil, false
		}
	}

	clone := cloneMap(root)
	currentClone := clone
	currentOriginal := root
	for _, part := range parts[:len(parts)-1] {
		nextOriginal, ok := currentOriginal[part].(map[string]any)
		if !ok {
			return nil, false
		}
		nextClone := cloneMap(nextOriginal)
		currentClone[part] = nextClone
		currentClone = nextClone
		currentOriginal = nextOriginal
	}
	return clone, true
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func setSimplePath(value any, path string, replacement any) (any, bool) {
	trimmed := strings.TrimPrefix(path, ".")
	if trimmed == "" {
		return replacement, true
	}
	parts := strings.Split(trimmed, ".")
	current, ok := value.(map[string]any)
	if !ok {
		return value, false
	}
	for i, part := range parts {
		if part == "" || strings.ContainsAny(part, "[]{}()|=<>!@") {
			return value, false
		}
		if i == len(parts)-1 {
			current[part] = replacement
			return value, true
		}
		next, ok := current[part].(map[string]any)
		if !ok {
			return value, false
		}
		current = next
	}
	return value, false
}

// pageItems extracts the items array from a page body using pagCfg.ItemsPath.
// Falls back to treating the body as an array, or wrapping it as a single item.
// A non-nil filterErr is returned when ItemsPath is set but the filter fails,
// so callers can surface the problem rather than silently returning no items.
func pageItems(body any, pagCfg *config.PaginationConfig) ([]any, error) {
	if pagCfg != nil && pagCfg.ItemsPath != "" {
		if m, ok := body.(map[string]any); ok {
			result, err := filter.Apply(pagCfg.ItemsPath, m, filter.LangAuto)
			if err != nil {
				return nil, fmt.Errorf("items_path filter %q: %w", pagCfg.ItemsPath, err)
			}
			if arr, ok := result.([]any); ok {
				return arr, nil
			}
			if result == nil {
				return nil, fmt.Errorf("items_path %q returned no items", pagCfg.ItemsPath)
			}
			if result != nil {
				return []any{result}, fmt.Errorf("items_path %q returned %T instead of an array", pagCfg.ItemsPath, result)
			}
		} else {
			return nil, fmt.Errorf("items_path %q requires an object response body", pagCfg.ItemsPath)
		}
		return nil, fmt.Errorf("items_path %q returned no items", pagCfg.ItemsPath)
	}
	if arr, ok := body.([]any); ok {
		return arr, nil
	}
	if body != nil {
		return []any{body}, nil
	}
	return nil, nil
}

// resolveNextURL returns the next-page URL from resp.Links or pagCfg.NextPath.
func resolveNextURL(resp *output.Response, pagCfg *config.PaginationConfig) string {
	// 1. Standard link relation "next".
	if resp.Links != nil {
		if next, ok := resp.Links["next"].(string); ok && next != "" {
			return next
		}
	}
	// 2. Per-API next_path override (extracts URL directly from body).
	if pagCfg != nil && pagCfg.NextPath != "" {
		if m, ok := resp.Body.(map[string]any); ok {
			result, err := filter.Apply(pagCfg.NextPath, m, filter.LangAuto)
			if err == nil {
				if s, ok := result.(string); ok && s != "" {
					return s
				}
			}
		}
	}
	return ""
}

// applyItemLimits truncates items to stay within maxItems. Returns the
// (possibly truncated) items and a done flag indicating the limit was hit.
func applyItemLimits(newItems, existing []any, maxItems int) ([]any, bool) {
	if maxItems <= 0 {
		return newItems, false
	}
	remaining := maxItems - len(existing)
	if remaining <= 0 {
		return nil, true
	}
	if len(newItems) > remaining {
		return newItems[:remaining], true
	}
	return newItems, false
}
