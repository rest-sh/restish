package cli

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rest-sh/restish/v2/internal/output"
	pluginwire "github.com/rest-sh/restish/v2/plugin"
	"github.com/spf13/cobra"
)

type commandProgressRenderer struct {
	formatter  output.ValueStreamFormatter
	stream     output.ValueStream
	cmd        *cobra.Command
	ctx        context.Context
	cancel     context.CancelFunc
	subprocess bool
	disabled   bool
	fallback   []string
	bytes      int
}

func (c *CLI) newCommandProgressRenderer(cmd *cobra.Command) *commandProgressRenderer {
	formatter, ok := c.formatters["progress"].(output.ValueStreamFormatter)
	if !ok {
		return nil
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	subprocess := false
	if pluginFormatter, ok := formatter.(*output.PluginFormatter); ok {
		clone := *pluginFormatter
		clone.Context = ctx
		formatter = &clone
		subprocess = true
	}
	return &commandProgressRenderer{
		formatter:  formatter,
		cmd:        cmd,
		ctx:        ctx,
		cancel:     cancel,
		subprocess: subprocess,
	}
}

func (r *commandProgressRenderer) Write(msg pluginwire.ProgressMsg) (bool, error) {
	if r.disabled {
		return false, nil
	}
	if (msg.Current == nil) != (msg.Total == nil) || msg.Current != nil && (*msg.Current < 0 || *msg.Total <= 0 || *msg.Current > *msg.Total) {
		return false, nil
	}
	if msg.Text != "" && r.bytes+len(msg.Text)+1 > maxCommandPluginDataBytes {
		err := r.closeStream()
		r.disabled = true
		return false, err
	}
	if r.stream == nil {
		// Command stderr is not coordinated with formatter redraws yet, so keep
		// this session line-oriented even when stderr is a terminal.
		stream, err := r.formatter.StartValueStream(r.cmd.ErrOrStderr(), &output.Response{}, false)
		if err != nil {
			r.disabled = true
			return false, err
		}
		r.stream = stream
	}
	record := map[string]any{
		"id":      msg.ID,
		"label":   msg.Label,
		"state":   msg.State,
		"current": msg.Current,
		"total":   msg.Total,
		"unit":    msg.Unit,
		"message": msg.Message,
	}
	stream := r.stream
	if err := r.run(func() error { return stream.WriteValue(record) }); err != nil {
		var closeErr error
		if r.ctx.Err() != nil {
			if r.subprocess {
				closeErr = stream.Close()
			}
			r.stream = nil
			r.replayFallback()
		} else {
			closeErr = r.closeStream()
		}
		r.disabled = true
		return false, errors.Join(err, closeErr)
	}
	if msg.Text != "" {
		r.fallback = append(r.fallback, msg.Text)
		r.bytes += len(msg.Text) + 1
	}
	return true, nil
}

func (r *commandProgressRenderer) Close() error {
	defer r.cancel()
	return r.closeStream()
}

func (r *commandProgressRenderer) closeStream() error {
	if r.stream == nil {
		return nil
	}
	err := r.run(r.stream.Close)
	r.stream = nil
	if err != nil {
		r.replayFallback()
	} else {
		r.fallback = nil
		r.bytes = 0
	}
	return err
}

func (r *commandProgressRenderer) replayFallback() {
	for _, text := range r.fallback {
		fmt.Fprintln(r.cmd.ErrOrStderr(), text)
	}
	r.fallback = nil
	r.bytes = 0
}

func (r *commandProgressRenderer) run(fn func() error) error {
	done := make(chan error, 1)
	go func() { done <- fn() }()
	timeout := commandPluginShutdownGrace()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case err := <-done:
		return err
	case <-r.ctx.Done():
		r.cancel()
		if r.subprocess {
			return errors.Join(r.ctx.Err(), <-done)
		}
		return r.ctx.Err()
	case <-timer.C:
		r.cancel()
		if r.subprocess {
			return errors.Join(fmt.Errorf("timed out after %s", timeout), <-done)
		}
		return fmt.Errorf("timed out after %s", timeout)
	}
}
