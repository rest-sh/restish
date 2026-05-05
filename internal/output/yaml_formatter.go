package output

import (
	"fmt"
	"io"

	"go.yaml.in/yaml/v3"
)

// YAMLFormatter serialises the response body as YAML.
type YAMLFormatter struct{}

func (f *YAMLFormatter) Format(w io.Writer, resp *Response, color bool) error {
	data, err := yaml.Marshal(resp.Body)
	if err != nil {
		return fmt.Errorf("yaml: %w", err)
	}
	_, err = w.Write(data)
	return err
}

// FormatValue writes a body/sub-value as YAML without the HTTP response
// preamble used by the full-response formatter path.
func (f *YAMLFormatter) FormatValue(w io.Writer, value any, color bool) error {
	data, err := yaml.Marshal(value)
	if err != nil {
		return fmt.Errorf("yaml: %w", err)
	}
	_, err = w.Write(data)
	return err
}
