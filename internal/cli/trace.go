package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

type traceLevel int

const (
	traceInfo traceLevel = iota + 1
	traceDebug
)

type tracePhase int

const (
	traceBefore tracePhase = iota
	traceAfter
)

type requestTraceContextKey struct{}

type requestTrace struct {
	beforeRendered bool
	afterRendered  bool
	entries        []traceEntry
	steps          []string
}

type traceEntry struct {
	phase tracePhase
	level traceLevel
	key   string
	value string
}

func newRequestTrace() *requestTrace {
	return &requestTrace{}
}

func withRequestTrace(ctx context.Context, trace *requestTrace) context.Context {
	if trace == nil {
		return ctx
	}
	return context.WithValue(ctx, requestTraceContextKey{}, trace)
}

func requestTraceFromContext(ctx context.Context) *requestTrace {
	if trace, ok := ctx.Value(requestTraceContextKey{}).(*requestTrace); ok {
		return trace
	}
	return nil
}

func ensureRequestTrace(cmd *cobra.Command) *requestTrace {
	if cmd == nil {
		return nil
	}
	ctx := requestContext(cmd)
	if trace := requestTraceFromContext(ctx); trace != nil {
		return trace
	}
	trace := newRequestTrace()
	cmd.SetContext(withRequestTrace(ctx, trace))
	return trace
}

func (t *requestTrace) InfoBefore(key, value string) {
	t.set(traceBefore, traceInfo, key, value)
}

func (t *requestTrace) DebugBefore(key, value string) {
	t.set(traceBefore, traceDebug, key, value)
}

func (t *requestTrace) Info(key, value string) {
	t.set(traceAfter, traceInfo, key, value)
}

func (t *requestTrace) Debug(key, value string) {
	t.set(traceAfter, traceDebug, key, value)
}

func (t *requestTrace) AddInfo(key, value string) {
	t.add(traceAfter, traceInfo, key, value)
}

func (t *requestTrace) AddDebug(key, value string) {
	t.add(traceAfter, traceDebug, key, value)
}

func (t *requestTrace) Step(value string) {
	if t == nil || strings.TrimSpace(value) == "" {
		return
	}
	t.steps = append(t.steps, strings.TrimSpace(value))
}

func (t *requestTrace) set(phase tracePhase, level traceLevel, key, value string) {
	if t == nil || strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
		return
	}
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	for i := range t.entries {
		entry := &t.entries[i]
		if entry.phase == phase && entry.level == level && entry.key == key {
			entry.value = value
			return
		}
	}
	t.entries = append(t.entries, traceEntry{phase: phase, level: level, key: key, value: value})
}

func (t *requestTrace) add(phase tracePhase, level traceLevel, key, value string) {
	if t == nil || strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
		return
	}
	t.entries = append(t.entries, traceEntry{
		phase: phase,
		level: level,
		key:   strings.TrimSpace(key),
		value: strings.TrimSpace(value),
	})
}

func (t *requestTrace) RenderBefore(w io.Writer, verbosity int) {
	if t == nil || t.beforeRendered || verbosity <= 0 {
		return
	}
	t.beforeRendered = true
	t.render(w, verbosity, traceBefore)
}

func (t *requestTrace) RenderAfter(w io.Writer, verbosity int) {
	if t == nil || t.afterRendered || verbosity <= 0 {
		return
	}
	t.afterRendered = true
	t.render(w, verbosity, traceAfter)
	if pipeline := t.pipeline(); pipeline != "" {
		fmt.Fprintf(w, "* Pipeline: %s\n", pipeline)
	}
}

func (t *requestTrace) render(w io.Writer, verbosity int, phase tracePhase) {
	for _, entry := range t.entries {
		if entry.phase != phase || !entry.visibleAt(verbosity) {
			continue
		}
		fmt.Fprintf(w, "* %s: %s\n", entry.key, entry.value)
	}
}

func (t *requestTrace) pipeline() string {
	if t == nil || len(t.steps) == 0 {
		return ""
	}
	steps := make([]string, 0, len(t.steps))
	var prev string
	for _, step := range t.steps {
		step = strings.TrimSpace(step)
		if step == "" || step == prev {
			continue
		}
		steps = append(steps, step)
		prev = step
	}
	return strings.Join(steps, " -> ")
}

func (e traceEntry) visibleAt(verbosity int) bool {
	if verbosity <= 0 {
		return false
	}
	return e.level == traceInfo || verbosity >= 2
}
