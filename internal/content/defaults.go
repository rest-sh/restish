package content

import (
	"encoding/json"

	"github.com/fxamacker/cbor/v2"
	"github.com/shamaton/msgpack/v2"
	"go.yaml.in/yaml/v3"
)

// Default returns a Registry pre-loaded with JSON, YAML, CBOR, msgpack,
// plain text, and gzip/deflate/brotli encodings.
func Default() *Registry {
	r := New()

	r.AddContentType(&ContentType{
		Name:      "json",
		MIMETypes: []string{"application/json"},
		Quality:   0.5,
		Marshal: func(v any) ([]byte, error) {
			return json.Marshal(v)
		},
		Unmarshal: func(data []byte) (any, error) {
			var v any
			if err := json.Unmarshal(data, &v); err != nil {
				return nil, err
			}
			return v, nil
		},
	})

	r.AddContentType(&ContentType{
		Name:      "yaml",
		MIMETypes: []string{"application/yaml", "application/x-yaml", "text/yaml", "text/x-yaml"},
		Quality:   0.5,
		Marshal: func(v any) ([]byte, error) {
			return yaml.Marshal(v)
		},
		Unmarshal: func(data []byte) (any, error) {
			var v any
			if err := yaml.Unmarshal(data, &v); err != nil {
				return nil, err
			}
			return v, nil
		},
	})

	r.AddContentType(&ContentType{
		Name:      "cbor",
		MIMETypes: []string{"application/cbor"},
		Quality:   0.9,
		Marshal: func(v any) ([]byte, error) {
			return cbor.Marshal(v)
		},
		Unmarshal: func(data []byte) (any, error) {
			var v any
			if err := cbor.Unmarshal(data, &v); err != nil {
				return nil, err
			}
			return v, nil
		},
	})

	r.AddContentType(&ContentType{
		Name:      "msgpack",
		MIMETypes: []string{"application/msgpack", "application/x-msgpack", "application/vnd.msgpack"},
		Quality:   0.8,
		Marshal: func(v any) ([]byte, error) {
			return msgpack.Marshal(v)
		},
		Unmarshal: func(data []byte) (any, error) {
			var v any
			if err := msgpack.Unmarshal(data, &v); err != nil {
				return nil, err
			}
			return v, nil
		},
	})

	// text/* is a catch-all for plain text responses — lowest quality so
	// structured formats are always preferred.
	r.AddContentType(&ContentType{
		Name:      "text",
		MIMETypes: []string{"text/*"},
		Quality:   0.2,
		Marshal: func(v any) ([]byte, error) {
			if s, ok := v.(string); ok {
				return []byte(s), nil
			}
			return json.Marshal(v)
		},
		Unmarshal: func(data []byte) (any, error) {
			return string(data), nil
		},
	})

	r.AddEncoding(&Encoding{
		Name:       "br",
		Quality:    1.0,
		Decompress: defaultBrotliDecompress,
	})

	r.AddEncoding(&Encoding{
		Name:       "gzip",
		Quality:    1.0,
		Decompress: defaultGzipDecompress,
	})

	r.AddEncoding(&Encoding{
		Name:       "deflate",
		Quality:    1.0,
		Decompress: defaultDeflateDecompress,
	})

	return r
}
