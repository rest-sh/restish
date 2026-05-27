package openapi

import (
	"fmt"
	"net/url"
	"strings"
)

type Param struct {
	Name          string
	In            string
	Type          string
	Style         string
	Explode       *bool
	AllowReserved bool
}

type Field struct {
	Key   string
	Value string
}

type Value struct {
	Scalar string
	Array  []string
	Object []Field
}

type QueryParam struct {
	Name          string
	Value         string
	AllowReserved bool
}

func ScalarValue(value string) Value {
	return Value{Scalar: value}
}

func ArrayValue(values []string) Value {
	return Value{Array: values}
}

func ObjectValue(fields []Field) Value {
	return Value{Object: fields}
}

func DefaultParamStyle(p Param) string {
	if p.Style != "" {
		return p.Style
	}
	switch p.In {
	case "query", "cookie":
		return "form"
	case "path", "header":
		return "simple"
	default:
		return "form"
	}
}

func ParamExplode(p Param) bool {
	if p.Explode != nil {
		return *p.Explode
	}
	return DefaultParamStyle(p) == "form"
}

func SerializePathParam(p Param, value Value) (string, error) {
	style := DefaultParamStyle(p)
	explode := ParamExplode(p)
	switch style {
	case "label":
		delimiter := ","
		if explode {
			delimiter = "."
		}
		return "." + pathDelimitedParamValue(p, value, delimiter, explode), nil
	case "matrix":
		switch {
		case p.Type == "array" && explode:
			var b strings.Builder
			for _, item := range value.Array {
				b.WriteString(";")
				b.WriteString(url.PathEscape(p.Name))
				b.WriteString("=")
				b.WriteString(url.PathEscape(item))
			}
			return b.String(), nil
		case p.Type == "object" && explode:
			var b strings.Builder
			for _, field := range value.Object {
				b.WriteString(";")
				b.WriteString(url.PathEscape(field.Key))
				b.WriteString("=")
				b.WriteString(url.PathEscape(field.Value))
			}
			return b.String(), nil
		default:
			return ";" + url.PathEscape(p.Name) + "=" + pathDelimitedParamValue(p, value, ",", false), nil
		}
	default:
		return pathDelimitedParamValue(p, value, ",", explode), nil
	}
}

func SerializeQueryParam(p Param, value Value) ([]QueryParam, error) {
	switch p.Type {
	case "array":
		switch DefaultParamStyle(p) {
		case "spaceDelimited":
			return []QueryParam{{Name: p.Name, Value: strings.Join(value.Array, " "), AllowReserved: p.AllowReserved}}, nil
		case "pipeDelimited":
			return []QueryParam{{Name: p.Name, Value: strings.Join(value.Array, "|"), AllowReserved: p.AllowReserved}}, nil
		case "deepObject":
			if ParamExplode(p) {
				out := make([]QueryParam, 0, len(value.Array))
				for _, item := range value.Array {
					out = append(out, QueryParam{Name: p.Name + "[]", Value: item, AllowReserved: p.AllowReserved})
				}
				return out, nil
			}
			return []QueryParam{{Name: p.Name, Value: strings.Join(value.Array, ","), AllowReserved: p.AllowReserved}}, nil
		default:
			if ParamExplode(p) {
				out := make([]QueryParam, 0, len(value.Array))
				for _, item := range value.Array {
					out = append(out, QueryParam{Name: p.Name, Value: item, AllowReserved: p.AllowReserved})
				}
				return out, nil
			}
			return []QueryParam{{Name: p.Name, Value: strings.Join(value.Array, ","), AllowReserved: p.AllowReserved}}, nil
		}
	case "object":
		switch {
		case DefaultParamStyle(p) == "deepObject":
			out := make([]QueryParam, 0, len(value.Object))
			for _, field := range value.Object {
				out = append(out, QueryParam{Name: p.Name + "[" + field.Key + "]", Value: field.Value, AllowReserved: p.AllowReserved})
			}
			return out, nil
		case ParamExplode(p):
			out := make([]QueryParam, 0, len(value.Object))
			for _, field := range value.Object {
				out = append(out, QueryParam{Name: field.Key, Value: field.Value, AllowReserved: p.AllowReserved})
			}
			return out, nil
		default:
			return []QueryParam{{Name: p.Name, Value: CommaDelimitedObject(value.Object), AllowReserved: p.AllowReserved}}, nil
		}
	default:
		return []QueryParam{{Name: p.Name, Value: value.Scalar, AllowReserved: p.AllowReserved}}, nil
	}
}

func SerializeHeaderParam(p Param, value Value) ([]string, error) {
	switch p.Type {
	case "object":
		if ParamExplode(p) {
			parts := make([]string, 0, len(value.Object))
			for _, field := range value.Object {
				parts = append(parts, field.Key+"="+field.Value)
			}
			return []string{strings.Join(parts, ",")}, nil
		}
		return []string{CommaDelimitedObject(value.Object)}, nil
	case "array":
		return []string{strings.Join(value.Array, ",")}, nil
	default:
		return []string{value.Scalar}, nil
	}
}

func SerializeCookieParam(p Param, value Value) ([]string, error) {
	switch p.Type {
	case "object":
		if ParamExplode(p) {
			out := make([]string, 0, len(value.Object))
			for _, field := range value.Object {
				out = append(out, url.QueryEscape(field.Key)+"="+url.QueryEscape(field.Value))
			}
			return out, nil
		}
		return []string{url.QueryEscape(p.Name) + "=" + url.QueryEscape(CommaDelimitedObject(value.Object))}, nil
	case "array":
		if ParamExplode(p) {
			out := make([]string, 0, len(value.Array))
			for _, item := range value.Array {
				out = append(out, url.QueryEscape(p.Name)+"="+url.QueryEscape(item))
			}
			return out, nil
		}
		return []string{url.QueryEscape(p.Name) + "=" + url.QueryEscape(strings.Join(value.Array, ","))}, nil
	default:
		return []string{url.QueryEscape(p.Name) + "=" + url.QueryEscape(value.Scalar)}, nil
	}
}

func NormalizeArrayValues(values []string) []string {
	if len(values) == 1 {
		parts := splitEscapedComma(values[0])
		if len(parts) > 1 {
			return parts
		}
	}
	return values
}

func CommaDelimitedObject(fields []Field) string {
	parts := make([]string, 0, len(fields)*2)
	for _, field := range fields {
		parts = append(parts, field.Key, field.Value)
	}
	return strings.Join(parts, ",")
}

func UnsupportedArrayStyleError(p Param) error {
	return fmt.Errorf("unsupported %s array style %q", p.In, DefaultParamStyle(p))
}

func pathDelimitedParamValue(p Param, value Value, delimiter string, explode bool) string {
	switch p.Type {
	case "array":
		escaped := make([]string, 0, len(value.Array))
		for _, item := range value.Array {
			escaped = append(escaped, url.PathEscape(item))
		}
		return strings.Join(escaped, delimiter)
	case "object":
		parts := make([]string, 0, len(value.Object)*2)
		for _, field := range value.Object {
			if explode {
				parts = append(parts, url.PathEscape(field.Key)+"="+url.PathEscape(field.Value))
				continue
			}
			parts = append(parts, url.PathEscape(field.Key), url.PathEscape(field.Value))
		}
		return strings.Join(parts, delimiter)
	default:
		return url.PathEscape(value.Scalar)
	}
}

func splitEscapedComma(value string) []string {
	var parts []string
	var b strings.Builder
	escaped := false
	for _, r := range value {
		if escaped {
			b.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == ',' {
			parts = append(parts, strings.TrimSpace(b.String()))
			b.Reset()
			continue
		}
		b.WriteRune(r)
	}
	if escaped {
		b.WriteRune('\\')
	}
	parts = append(parts, strings.TrimSpace(b.String()))
	return parts
}
