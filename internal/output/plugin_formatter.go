package output

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/rest-sh/restish/v2/internal/plugin"
	pluginwire "github.com/rest-sh/restish/v2/plugin"
)

// PluginFormatter is an output.Formatter backed by a hook plugin. The plugin
// receives a short formatter session over CBOR on stdin and writes its
// formatted output directly to stdout (raw bytes, no CBOR reply framing).
type PluginFormatter struct {
	PluginPath string
	FormatName string
	Context    context.Context
}

var startPluginFormatterStream = func(ctx context.Context, path string, w io.Writer, in any) (formatterStream, error) {
	return plugin.StartFormatterStream(ctx, path, w, in)
}

// Format sends the response to the plugin using the formatter session protocol
// and copies the plugin's raw output to w.
func (f *PluginFormatter) Format(w io.Writer, resp *Response, color bool) error {
	stream, err := startPluginFormatterStream(f.context(), f.PluginPath, w, pluginwire.FormatterRequest{
		Type:   "formatter",
		Format: f.FormatName,
		Color:  color,
		Event:  "start",
		Response: pluginwire.FormatterResponse{
			Proto:   resp.Proto,
			Status:  resp.Status,
			Headers: resp.Headers,
			Links:   resp.Links,
			Body:    resp.Body,
		},
	})
	if err != nil {
		return fmt.Errorf("formatter plugin %s: %w", f.FormatName, err)
	}
	if err := finishPluginFormatterSession(f.FormatName, stream, pluginwire.FormatterRequest{
		Type:   "formatter",
		Format: f.FormatName,
		Event:  "end",
	}); err != nil {
		return err
	}
	return nil
}

// FormatValue renders a body/sub-value through a short formatter session,
// without implying that the value is a full HTTP response.
func (f *PluginFormatter) FormatValue(w io.Writer, value any, color bool) error {
	stream, err := startPluginFormatterStream(f.context(), f.PluginPath, w, pluginwire.FormatterRequest{
		Type:     "formatter",
		Format:   f.FormatName,
		Color:    color,
		Event:    "start",
		Response: pluginwire.FormatterResponse{},
	})
	if err != nil {
		return fmt.Errorf("formatter plugin %s: %w", f.FormatName, err)
	}
	if err := stream.Send(pluginwire.FormatterRequest{
		Type:   "formatter",
		Format: f.FormatName,
		Event:  "item",
		Response: pluginwire.FormatterResponse{
			Body: value,
		},
	}); err != nil {
		return finishPluginFormatterSession(f.FormatName, stream, nil, err)
	}
	if err := finishPluginFormatterSession(f.FormatName, stream, pluginwire.FormatterRequest{
		Type:   "formatter",
		Format: f.FormatName,
		Event:  "end",
	}); err != nil {
		return err
	}
	return nil
}

// StartValueStream starts a long-lived formatter plugin session.
func (f *PluginFormatter) StartValueStream(w io.Writer, base *Response, color bool) (ValueStream, error) {
	stream, err := startPluginFormatterStream(f.context(), f.PluginPath, w, pluginwire.FormatterRequest{
		Type:   "formatter",
		Format: f.FormatName,
		Color:  color,
		Event:  "start",
		Response: pluginwire.FormatterResponse{
			Proto:   base.Proto,
			Status:  base.Status,
			Headers: base.Headers,
			Links:   base.Links,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("formatter plugin %s: %w", f.FormatName, err)
	}
	return &pluginFormatterStream{
		pluginPath: f.PluginPath,
		formatName: f.FormatName,
		stream:     stream,
	}, nil
}

func finishPluginFormatterSession(formatName string, stream formatterStream, final any, priorErr ...error) error {
	var sendErr error
	if final != nil {
		sendErr = stream.Send(final)
	}
	closeErr := stream.Close()
	if joined := errors.Join(append(priorErr, sendErr, closeErr)...); joined != nil {
		return fmt.Errorf("formatter plugin %s: %w", formatName, joined)
	}
	return nil
}

func (f *PluginFormatter) context() context.Context {
	if f.Context != nil {
		return f.Context
	}
	return context.Background()
}

type pluginFormatterStream struct {
	pluginPath string
	formatName string
	stream     formatterStream
}

type formatterStream interface {
	Send(any) error
	Close() error
}

func (s *pluginFormatterStream) WriteValue(value any) error {
	if err := s.stream.Send(pluginwire.FormatterRequest{
		Type:   "formatter",
		Format: s.formatName,
		Event:  "item",
		Response: pluginwire.FormatterResponse{
			Body: value,
		},
	}); err != nil {
		return fmt.Errorf("formatter plugin %s: %w", s.formatName, err)
	}
	return nil
}

func (s *pluginFormatterStream) Close() error {
	return finishPluginFormatterSession(s.formatName, s.stream, pluginwire.FormatterRequest{
		Type:   "formatter",
		Format: s.formatName,
		Event:  "end",
	})
}
