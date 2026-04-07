package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/danielgtaylor/restish/v2/internal/config"
	"github.com/danielgtaylor/restish/v2/internal/filter"
	"github.com/danielgtaylor/restish/v2/internal/hypermedia"
	"github.com/danielgtaylor/restish/v2/internal/output"
	"github.com/danielgtaylor/restish/v2/internal/request"
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
	noPaginate, _ := cmd.Flags().GetBool("rsh-no-paginate")
	if noPaginate {
		return false, nil
	}

	nextURL := resolveNextURL(firstResp, pagCfg)
	if nextURL == "" {
		return false, nil
	}

	collect, _ := cmd.Flags().GetBool("rsh-collect")
	maxPages, _ := cmd.Flags().GetInt("rsh-max-pages")
	maxItems, _ := cmd.Flags().GetInt("rsh-max-items")

	return true, c.runPagination(cmd, firstResp, nextURL, opts, pagCfg, collect, maxPages, maxItems)
}

// runPagination drives the pagination loop starting from firstResp.
func (c *CLI) runPagination(
	cmd *cobra.Command,
	firstResp *output.Response,
	firstNextURL string,
	opts request.Options,
	pagCfg *config.PaginationConfig,
	collect bool,
	maxPages, maxItems int,
) error {
	var allItems []any

	// Process first page.
	items, filterErr := pageItems(firstResp.Body, pagCfg)
	if filterErr != nil {
		fmt.Fprintf(c.Stderr, "warning: pagination items_path: %v\n", filterErr)
	}
	items, done := applyItemLimits(items, allItems, maxItems)
	if collect {
		allItems = append(allItems, items...)
	} else {
		if err := c.streamItems(items); err != nil {
			return err
		}
	}

	nextURL := firstNextURL
	page := 1
	stderrIsTTY := output.IsTerminal(c.Stderr)

	for !done && nextURL != "" {
		// Safety: max pages check (page is 1-indexed, firstResp is page 1).
		if maxPages > 0 && page >= maxPages {
			fmt.Fprintf(c.Stderr, "warning: reached --rsh-max-pages limit (%d); stopping pagination\n", maxPages)
			break
		}
		page++

		// Progress feedback on TTY stderr.
		if stderrIsTTY {
			fmt.Fprintf(c.Stderr, "\rfetching page %d...", page)
		}

		httpResp, err := request.Do(context.Background(), "GET", nextURL, nil, opts)
		if err != nil {
			return fmt.Errorf("paginate page %d: %w", page, err)
		}

		resp, err := output.Normalize(httpResp, c.content)
		if err != nil {
			return fmt.Errorf("paginate page %d normalize: %w", page, err)
		}

		// Parse links for this page.
		if httpResp.Request != nil {
			if links := hypermedia.Parse(httpResp.Request.URL, httpResp.Header, resp.Body, c.linkParsers); len(links) > 0 {
				resp.Links = make(map[string]any, len(links))
				for k, v := range links {
					resp.Links[k] = v
				}
			}
		}

		items, filterErr = pageItems(resp.Body, pagCfg)
		if filterErr != nil {
			fmt.Fprintf(c.Stderr, "warning: pagination items_path: %v\n", filterErr)
		}
		items, done = applyItemLimits(items, allItems, maxItems)
		if collect {
			allItems = append(allItems, items...)
		} else {
			if err := c.streamItems(items); err != nil {
				return err
			}
		}
		nextURL = resolveNextURL(resp, pagCfg)
	}

	// Erase progress line on TTY.
	if stderrIsTTY {
		fmt.Fprintf(c.Stderr, "\r")
	}

	if done && maxItems > 0 {
		fmt.Fprintf(c.Stderr, "warning: reached --rsh-max-items limit (%d); stopping pagination\n", maxItems)
	}

	if collect {
		// Format the full collection through the normal pipeline.
		synthetic := &output.Response{
			Proto:  firstResp.Proto,
			Status: firstResp.Status,
			Body:   allItems,
		}
		return c.formatResponse(cmd, synthetic)
	}
	return nil
}

// streamItems JSON-encodes each item and writes it to stdout (one per line).
func (c *CLI) streamItems(items []any) error {
	for _, item := range items {
		b, err := json.Marshal(item)
		if err != nil {
			return err
		}
		fmt.Fprintf(c.Stdout, "%s\n", b)
	}
	return nil
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
			if result != nil {
				return []any{result}, nil
			}
		}
		return nil, nil
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
