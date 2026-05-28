package content

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/amazon-ion/ion-go/ion"
	"github.com/fxamacker/cbor/v2"
	"github.com/shamaton/msgpack/v3"
	"go.yaml.in/yaml/v3"
)

// MultipartBody carries a structured multipart/form-data body plus optional
// per-field Content-Type metadata from an OpenAPI encoding object.
type MultipartBody struct {
	Value        any
	ContentTypes map[string]string
}

// Default returns a Registry pre-loaded with JSON, YAML, CBOR, msgpack, Ion,
// plain text, and gzip/deflate/brotli encodings.
func Default() *Registry {
	r := New()

	r.AddContentType(&ContentType{
		Name:      "json",
		MIMETypes: []string{"application/json"},
		Suffixes:  []string{"+json"},
		Quality:   0.9,
		Marshal: func(v any) ([]byte, error) {
			return json.Marshal(v)
		},
		Unmarshal: func(data []byte) (any, error) {
			var v any
			if err := json.Unmarshal(data, &v); err != nil {
				if seq, seqErr := unmarshalJSONSequence(data); seqErr == nil {
					return seq, nil
				}
				return nil, err
			}
			return v, nil
		},
	})

	r.AddContentType(&ContentType{
		Name:      "ndjson",
		MIMETypes: []string{"application/x-ndjson", "application/ndjson", "application/jsonl", "application/jsonlines"},
		Quality:   0.8,
		Marshal:   marshalNDJSON,
		Unmarshal: func(data []byte) (any, error) {
			lines := bytes.Split(data, []byte{'\n'})
			out := make([]any, 0, len(lines))
			for _, line := range lines {
				line = bytes.TrimSpace(line)
				if len(line) == 0 {
					continue
				}
				var v any
				if err := json.Unmarshal(line, &v); err != nil {
					return nil, err
				}
				out = append(out, v)
			}
			return out, nil
		},
	})

	r.AddContentType(&ContentType{
		Name:      "xml",
		MIMETypes: []string{"application/xml", "text/xml"},
		Suffixes:  []string{"+xml"},
		Quality:   0.2,
		Marshal:   marshalRawText("XML"),
		Unmarshal: func(data []byte) (any, error) {
			return string(data), nil
		},
	})

	r.AddContentType(&ContentType{
		Name:      "yaml",
		MIMETypes: []string{"application/yaml", "application/x-yaml", "text/yaml", "text/x-yaml"},
		Suffixes:  []string{"+yaml"},
		Quality:   0.8,
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
		Quality:   0.6,
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
		Quality:   0.6,
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
		Name:      "binary",
		MIMETypes: []string{"application/octet-stream"},
		Quality:   0.1,
		Marshal: func(v any) ([]byte, error) {
			switch t := v.(type) {
			case nil:
				return nil, nil
			case []byte:
				return t, nil
			case string:
				return []byte(t), nil
			default:
				return json.Marshal(v)
			}
		},
		Unmarshal: func(data []byte) (any, error) {
			if b, ok := Printable(data); ok {
				return string(b), nil
			}
			return data, nil
		},
	})

	r.AddContentType(&ContentType{
		Name:      "ion",
		MIMETypes: []string{"application/ion", "text/ion"},
		Suffixes:  []string{"+ion"},
		Quality:   0.6,
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
		Name:      "sse",
		MIMETypes: []string{"text/event-stream"},
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

	// text/* is a catch-all for plain text responses. Keep it below explicit
	// structured types so it does not win when a more specific format is known.
	r.AddContentType(&ContentType{
		Name:      "text",
		MIMETypes: []string{"text/plain", "text/*"},
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

func unmarshalJSONSequence(data []byte) ([]any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	out := make([]any, 0)
	for {
		var v any
		if err := dec.Decode(&v); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		out = append(out, v)
	}
	if len(out) < 2 {
		return nil, fmt.Errorf("JSON sequence must contain at least two values")
	}
	return out, nil
}

func marshalNDJSON(v any) ([]byte, error) {
	if data, ok := rawTextBytes(v); ok {
		return data, nil
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	rv := reflect.ValueOf(v)
	if rv.IsValid() && (rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array) {
		if _, ok := v.([]byte); !ok {
			for i := 0; i < rv.Len(); i++ {
				if err := enc.Encode(rv.Index(i).Interface()); err != nil {
					return nil, err
				}
			}
			return buf.Bytes(), nil
		}
	}
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func marshalRawText(name string) func(any) ([]byte, error) {
	return func(v any) ([]byte, error) {
		if data, ok := rawTextBytes(v); ok {
			return data, nil
		}
		return nil, fmt.Errorf("%s request bodies must be supplied as raw text or @file input", name)
	}
}

func rawTextBytes(v any) ([]byte, bool) {
	switch t := v.(type) {
	case nil:
		return nil, true
	case []byte:
		return t, true
	case string:
		return []byte(t), true
	default:
		return nil, false
	}
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
	opts := multipartOptions{}
	if body, ok := v.(MultipartBody); ok {
		v = body.Value
		opts.contentTypes = body.ContentTypes
	} else if body, ok := v.(*MultipartBody); ok && body != nil {
		v = body.Value
		opts.contentTypes = body.ContentTypes
	}
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := addMultipartParts(writer, "", v, opts); err != nil {
		writer.Close()
		return nil, "", err
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return body.Bytes(), writer.FormDataContentType(), nil
}

type multipartOptions struct {
	contentTypes map[string]string
}

func addMultipartParts(writer *multipart.Writer, prefix string, v any, opts multipartOptions) error {
	switch typed := v.(type) {
	case map[string]any:
		if prefix != "" && opts.contentTypes[prefix] != "" {
			return writeMultipartField(writer, prefix, typed, opts.contentTypes[prefix])
		}
		keys := sortedKeys(typed)
		for _, key := range keys {
			name := key
			if prefix != "" {
				name = prefix + "[" + key + "]"
			}
			if err := addMultipartParts(writer, name, typed[key], opts); err != nil {
				return err
			}
		}
	case []any:
		for _, item := range typed {
			name := prefix
			if name == "" {
				return fmt.Errorf("multipart bodies must be an object")
			}
			if err := addMultipartParts(writer, name, item, opts); err != nil {
				return err
			}
		}
	case nil:
		if prefix == "" {
			return fmt.Errorf("multipart bodies must be an object")
		}
		return writeMultipartField(writer, prefix, nil, opts.contentTypes[prefix])
	default:
		if prefix == "" {
			return fmt.Errorf("multipart bodies must be an object")
		}
		if literal, ok := multipartEscapedAtLiteral(v); ok {
			return writeMultipartField(writer, prefix, literal, opts.contentTypes[prefix])
		}
		if filePath, ok := multipartFilePath(v); ok {
			return addMultipartFile(writer, prefix, filePath, opts.contentTypes[prefix])
		} else if err := multipartFileReferenceError(v); err != nil {
			return err
		}
		return writeMultipartField(writer, prefix, v, opts.contentTypes[prefix])
	}
	return nil
}

func writeMultipartField(writer *multipart.Writer, fieldName string, value any, contentType string) error {
	data, err := multipartFieldBytes(value, contentType)
	if err != nil {
		return err
	}
	if contentType == "" {
		return writer.WriteField(fieldName, string(data))
	}
	part, err := writer.CreatePart(multipartPartHeader(fieldName, "", contentType))
	if err != nil {
		return err
	}
	_, err = part.Write(data)
	return err
}

func multipartFieldBytes(value any, contentType string) ([]byte, error) {
	if value == nil {
		return nil, nil
	}
	if strings.EqualFold(strings.TrimSpace(strings.Split(contentType, ";")[0]), "application/json") ||
		strings.HasSuffix(strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0])), "+json") {
		return json.Marshal(value)
	}
	switch typed := value.(type) {
	case []byte:
		return typed, nil
	case string:
		return []byte(typed), nil
	default:
		return []byte(fmt.Sprint(value)), nil
	}
}

func multipartEscapedAtLiteral(v any) (string, bool) {
	s, ok := v.(string)
	if !ok || !strings.HasPrefix(s, "@@") {
		return "", false
	}
	return s[1:], true
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

func multipartFileReferenceError(v any) error {
	s, ok := v.(string)
	if !ok || len(s) < 2 || !strings.HasPrefix(s, "@") || strings.HasPrefix(s, "@@") {
		return nil
	}
	path := s[1:]
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("unable to read multipart file %q: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("unable to read multipart file %q: is a directory", path)
	}
	return nil
}

func addMultipartFile(writer *multipart.Writer, fieldName, path, contentType string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	var part io.Writer
	if contentType == "" {
		part, err = writer.CreateFormFile(fieldName, filepath.Base(path))
	} else {
		part, err = writer.CreatePart(multipartPartHeader(fieldName, filepath.Base(path), contentType))
	}
	if err != nil {
		return err
	}
	_, err = io.Copy(part, file)
	return err
}

func multipartPartHeader(fieldName, fileName, contentType string) textproto.MIMEHeader {
	header := make(textproto.MIMEHeader)
	disposition := fmt.Sprintf(`form-data; name="%s"`, escapeMultipartQuote(fieldName))
	if fileName != "" {
		disposition += fmt.Sprintf(`; filename="%s"`, escapeMultipartQuote(fileName))
	}
	header.Set("Content-Disposition", disposition)
	if contentType != "" {
		header.Set("Content-Type", contentType)
	}
	return header
}

func escapeMultipartQuote(s string) string {
	return strings.NewReplacer("\\", "\\\\", `"`, "\\\"").Replace(s)
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
