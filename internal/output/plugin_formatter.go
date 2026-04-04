package output

import (
	"fmt"
	"io"

	"github.com/danielgtaylor/restish/v2/internal/plugin"
)

// PluginFormatter is an output.Formatter backed by a hook plugin. The plugin
// receives the normalized response as a CBOR message on stdin and writes its
// formatted output directly to stdout (raw bytes, no CBOR framing).
type PluginFormatter struct {
	PluginPath string
	FormatName string
}

// Format sends the response to the plugin and copies the plugin's raw output
// to w.
func (f *PluginFormatter) Format(w io.Writer, resp *Response, color bool) error {
	in := map[string]any{
		"type":   "formatter",
		"format": f.FormatName,
		"color":  color,
		"response": map[string]any{
			"proto":   resp.Proto,
			"status":  resp.Status,
			"headers": resp.Headers,
			"links":   resp.Links,
			"body":    resp.Body,
		},
	}
	data, err := plugin.CallFormatterHook(f.PluginPath, in)
	if err != nil {
		return fmt.Errorf("formatter plugin %s: %w", f.FormatName, err)
	}
	_, err = w.Write(data)
	return err
}
