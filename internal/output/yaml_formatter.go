package output

import (
	"fmt"
	"io"

	"go.yaml.in/yaml/v3"
)

// YAMLFormatter serialises the response body as YAML. On TTY it also prints a
// brief status/header preamble (matching the readable formatter convention) so
// the output is still identifiable as an HTTP response.
type YAMLFormatter struct{}

func (f *YAMLFormatter) Format(w io.Writer, resp *Response, color bool) error {
	if color {
		// Print a minimal preamble so the user can see the status code.
		if _, err := fmt.Fprintf(w, "%s %d\n\n", resp.Proto, resp.Status); err != nil {
			return err
		}
	}

	data, err := yaml.Marshal(resp.Body)
	if err != nil {
		return fmt.Errorf("yaml: %w", err)
	}
	_, err = w.Write(data)
	return err
}
