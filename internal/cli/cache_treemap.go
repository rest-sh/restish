package cli

import (
	"fmt"
	"io"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/rest-sh/restish/v2/internal/cache"
	"github.com/rest-sh/restish/v2/config"
	"golang.org/x/term"
)

type cacheTreemapItem struct {
	label string
	size  int64
	fill  string
	token string
}

type cacheTreemapRect struct {
	item       cacheTreemapItem
	x, y, w, h int
}

func printCacheTreemap(w io.Writer, style humanTextStyle, title string, items []cache.Breakdown, cfg *config.Config, project *projectConfigState, width, height, limit int) {
	treemapItems := cacheTreemapItems(items, cfg, project, limit)
	if len(treemapItems) == 0 {
		return
	}
	if width < 32 {
		width = 32
	}
	if width > 104 {
		width = 104
	}
	if height < 9 {
		height = 9
	}
	if height > 18 {
		height = 18
	}
	innerWidth := width - 2
	if innerWidth < 30 {
		innerWidth = 30
	}
	rects := layoutCacheTreemap(treemapItems, cacheTreemapRect{x: 0, y: 0, w: innerWidth, h: height})
	grid := make([][]cacheTreemapCell, height)
	for y := range grid {
		grid[y] = make([]cacheTreemapCell, innerWidth)
		for x := range grid[y] {
			grid[y][x] = cacheTreemapCell{text: " "}
		}
	}
	total := int64(0)
	for _, item := range treemapItems {
		total += item.size
	}
	for _, rect := range rects {
		drawCacheTreemapRect(grid, rect, total)
	}
	fmt.Fprintln(w, style.key(title))
	fmt.Fprintf(w, "  %s%s%s\n", style.hint("╭"), style.hint(strings.Repeat("─", innerWidth)), style.hint("╮"))
	for _, row := range grid {
		fmt.Fprintf(w, "  %s", style.hint("│"))
		for _, cell := range row {
			fmt.Fprint(w, style.style(cell.token, cell.text))
		}
		fmt.Fprintf(w, "%s\n", style.hint("│"))
	}
	fmt.Fprintf(w, "  %s%s%s\n", style.hint("╰"), style.hint(strings.Repeat("─", innerWidth)), style.hint("╯"))
}

func cacheTreemapItems(items []cache.Breakdown, cfg *config.Config, project *projectConfigState, limit int) []cacheTreemapItem {
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	fills := []string{"█", "▓", "▒", "░", "■", "▪", "◆", "●", "▲", "◇"}
	tokens := []string{"status_2xx", "diagnostic_info", "diagnostic_hint", "diagnostic_warn", "key", "url", "number", "string", "type", "operator"}
	out := make([]cacheTreemapItem, 0, len(items))
	for i, item := range items {
		if item.SizeBytes <= 0 {
			continue
		}
		details := cacheNamespaceInfo(item.Name, cfg, project)
		out = append(out, cacheTreemapItem{
			label: details.name,
			size:  item.SizeBytes,
			fill:  fills[i%len(fills)],
			token: tokens[i%len(tokens)],
		})
	}
	return out
}

func cacheTreemapSize(w io.Writer) (int, int) {
	width := 60
	height := 10
	if f, ok := w.(interface{ Fd() uintptr }); ok {
		if terminalWidth, terminalHeight, err := term.GetSize(int(f.Fd())); err == nil {
			if terminalWidth > 0 {
				width = terminalWidth - 4
			}
			if terminalHeight >= 42 {
				height = 18
			} else if terminalHeight >= 34 {
				height = 15
			} else if terminalHeight >= 26 {
				height = 12
			}
		}
	}
	if width < 32 {
		width = 32
	}
	if width > 104 {
		width = 104
	}
	return width, height
}

func layoutCacheTreemap(items []cacheTreemapItem, bounds cacheTreemapRect) []cacheTreemapRect {
	if len(items) == 0 || bounds.w <= 0 || bounds.h <= 0 {
		return nil
	}
	if len(items) == 1 || bounds.w == 1 || bounds.h == 1 {
		bounds.item = items[0]
		return []cacheTreemapRect{bounds}
	}

	total := cacheTreemapTotal(items)
	leftTotal, split := int64(0), 0
	for split < len(items)-1 {
		nextTotal := leftTotal + items[split].size
		if leftTotal > 0 && math.Abs(float64(total/2-leftTotal)) <= math.Abs(float64(total/2-nextTotal)) {
			break
		}
		leftTotal = nextTotal
		split++
	}
	if split == 0 {
		split = 1
		leftTotal = items[0].size
	}

	var first, second cacheTreemapRect
	if bounds.w >= bounds.h {
		firstW := int(math.Round(float64(bounds.w) * float64(leftTotal) / float64(total)))
		if firstW < 1 {
			firstW = 1
		}
		if firstW > bounds.w-1 {
			firstW = bounds.w - 1
		}
		first = cacheTreemapRect{x: bounds.x, y: bounds.y, w: firstW, h: bounds.h}
		second = cacheTreemapRect{x: bounds.x + firstW, y: bounds.y, w: bounds.w - firstW, h: bounds.h}
	} else {
		firstH := int(math.Round(float64(bounds.h) * float64(leftTotal) / float64(total)))
		if firstH < 1 {
			firstH = 1
		}
		if firstH > bounds.h-1 {
			firstH = bounds.h - 1
		}
		first = cacheTreemapRect{x: bounds.x, y: bounds.y, w: bounds.w, h: firstH}
		second = cacheTreemapRect{x: bounds.x, y: bounds.y + firstH, w: bounds.w, h: bounds.h - firstH}
	}

	rects := layoutCacheTreemap(items[:split], first)
	rects = append(rects, layoutCacheTreemap(items[split:], second)...)
	return rects
}

func cacheTreemapTotal(items []cacheTreemapItem) int64 {
	var total int64
	for _, item := range items {
		total += item.size
	}
	if total <= 0 {
		return 1
	}
	return total
}

type cacheTreemapCell struct {
	text  string
	token string
}

func drawCacheTreemapRect(grid [][]cacheTreemapCell, rect cacheTreemapRect, total int64) {
	for y := rect.y; y < rect.y+rect.h && y < len(grid); y++ {
		for x := rect.x; x < rect.x+rect.w && x < len(grid[y]); x++ {
			grid[y][x] = cacheTreemapCell{text: rect.item.fill, token: rect.item.token}
		}
	}
	labelWidth := rect.w - 2
	if labelWidth < 4 || rect.h < 2 {
		return
	}
	percent := int(math.Round(float64(rect.item.size) * 100 / float64(total)))
	lines := []string{truncateCacheTreemapLabel(rect.item.label, labelWidth)}
	if rect.h >= 3 {
		lines = append(lines, truncateCacheTreemapLabel(fmt.Sprintf("%d%%", percent), labelWidth))
	}
	for i, line := range lines {
		y := rect.y + 1 + i
		if y >= len(grid) {
			break
		}
		x := rect.x + 1
		for _, r := range line {
			if x >= rect.x+rect.w-1 || x >= len(grid[y]) {
				break
			}
			grid[y][x] = cacheTreemapCell{text: string(r), token: "text"}
			x++
		}
	}
}

func truncateCacheTreemapLabel(label string, width int) string {
	if width <= 0 {
		return ""
	}
	if utf8.RuneCountInString(label) <= width {
		return label
	}
	if width <= 1 {
		r, _ := utf8.DecodeRuneInString(label)
		return string(r)
	}
	runes := []rune(label)
	return string(runes[:width-1]) + "."
}
