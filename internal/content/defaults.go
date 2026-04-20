package content

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/amazon-ion/ion-go/ion"
	"github.com/fxamacker/cbor/v2"
	"github.com/shamaton/msgpack/v2"
	"go.yaml.in/yaml/v3"
)

// Default returns a Registry pre-loaded with JSON, YAML, CBOR, msgpack, Ion,
// plain text, and gzip/deflate/brotli encodings.
func Default() *Registry {
	r := New()

	r.AddContentType(&ContentType{
		Name:      "json",
		MIMETypes: []string{"application/json"},
		Suffixes:  []string{"+json"},
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
		Suffixes:  []string{"+yaml"},
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
		Suffixes:  []string{"+cbor"},
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
		Suffixes:  []string{"+msgpack"},
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

	r.AddContentType(&ContentType{
		Name:      "ion",
		MIMETypes: []string{"application/ion", "text/ion"},
		Suffixes:  []string{"+ion"},
		Quality:   0.8,
		Marshal: func(v any) ([]byte, error) {
			return ion.MarshalText(v)
		},
		Unmarshal: func(data []byte) (any, error) {
			var v any
			if err := ion.Unmarshal(data, &v); err != nil {
				return nil, err
			}
			return v, nil
		},
	})

	// text/* is a catch-all for plain text responses — lowest quality so
	// structured formats are always preferred.
	r.AddContentType(&ContentType{
		Name:      "form",
		MIMETypes: []string{"application/x-www-form-urlencoded"},
		Quality:   0.3,
		Marshal:   marshalForm,
		Unmarshal: func(data []byte) (any, error) {
			values, err := url.ParseQuery(string(data))
			if err != nil {
				return nil, err
			}
			out := make(map[string]any, len(values))
			for key, items := range values {
				if len(items) == 1 {
					out[key] = items[0]
				} else {
					vals := make([]any, len(items))
					for i, item := range items {
						vals[i] = item
					}
					out[key] = vals
				}
			}
			return out, nil
		},
	})

	r.AddContentType(&ContentType{
		Name:      "multipart",
		MIMETypes: []string{"multipart/form-data"},
		Quality:   0.3,
		MarshalContentType: func(v any) ([]byte, string, error) {
			data, contentType, err := marshalMultipart(v)
			return data, contentType, err
		},
		Marshal: func(v any) ([]byte, error) {
			data, _, err := marshalMultipart(v)
			return data, err
		},
		Unmarshal: func(data []byte) (any, error) {
			return string(data), nil
		},
	})

	r.AddContentType(&ContentType{
		Name:      "text",
		MIMETypes: []string{"text/event-stream", "text/*"},
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

func marshalForm(v any) ([]byte, error) {
	values := url.Values{}
	if err := addFormValues(values, "", v); err != nil {
		return nil, err
	}
	return []byte(values.Encode()), nil
}

func addFormValues(values url.Values, prefix string, v any) error {
	switch typed := v.(type) {
	case map[string]any:
		if prefix == "" && len(typed) == 0 {
			return nil
		}
		keys := sortedKeys(typed)
		for _, key := range keys {
			name := key
			if prefix != "" {
				name = prefix + "[" + key + "]"
			}
			if err := addFormValues(values, name, typed[key]); err != nil {
				return err
			}
		}
	case []any:
		for _, item := range typed {
			name := prefix
			if name == "" {
				return fmt.Errorf("form bodies must be an object")
			}
			if err := addFormValues(values, name+"[]", item); err != nil {
				return err
			}
		}
	case nil:
		if prefix == "" {
			return fmt.Errorf("form bodies must be an object")
		}
		values.Add(prefix, "")
	default:
		if prefix == "" {
			return fmt.Errorf("form bodies must be an object")
		}
		values.Add(prefix, fmt.Sprint(v))
	}
	return nil
}

func marshalMultipart(v any) ([]byte, string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := addMultipartParts(writer, "", v); err != nil {
		writer.Close()
		return nil, "", err
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return body.Bytes(), writer.FormDataContentType(), nil
}

func addMultipartParts(writer *multipart.Writer, prefix string, v any) error {
	switch typed := v.(type) {
	case map[string]any:
		keys := sortedKeys(typed)
		for _, key := range keys {
			name := key
			if prefix != "" {
				name = prefix + "[" + key + "]"
			}
			if err := addMultipartParts(writer, name, typed[key]); err != nil {
				return err
			}
		}
	case []any:
		for _, item := range typed {
			name := prefix
			if name == "" {
				return fmt.Errorf("multipart bodies must be an object")
			}
			if err := addMultipartParts(writer, name+"[]", item); err != nil {
				return err
			}
		}
	case nil:
		if prefix == "" {
			return fmt.Errorf("multipart bodies must be an object")
		}
		return writer.WriteField(prefix, "")
	default:
		if prefix == "" {
			return fmt.Errorf("multipart bodies must be an object")
		}
		if filePath, ok := multipartFilePath(v); ok {
			return addMultipartFile(writer, prefix, filePath)
		}
		return writer.WriteField(prefix, fmt.Sprint(v))
	}
	return nil
}

func multipartFilePath(v any) (string, bool) {
	s, ok := v.(string)
	if !ok || len(s) < 2 || !strings.HasPrefix(s, "@") {
		return "", false
	}
	path := s[1:]
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return "", false
	}
	return path, true
}

func addMultipartFile(writer *multipart.Writer, fieldName, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	part, err := writer.CreateFormFile(fieldName, filepath.Base(path))
	if err != nil {
		return err
	}
	_, err = io.Copy(part, file)
	return err
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
