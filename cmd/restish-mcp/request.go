package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	openapiparam "github.com/rest-sh/restish/v2/internal/openapi"
)

type HTTPRequest struct {
	Method      string
	URI         string
	Headers     map[string]string
	Body        any
	ContentType string
	Timeout     int
}

type HTTPResponse struct {
	Status  int
	Headers map[string][]string
	Body    any
	Error   string
}

func (t *Tool) Request(args map[string]any) (*HTTPRequest, error) {
	path := t.Path
	query := url.Values{}
	headers := map[string]string{}
	var cookies []string

	for _, param := range t.Params {
		value, ok := args[param.Name]
		if !ok || value == nil {
			if param.Required {
				return nil, fmt.Errorf("missing required parameter %q", param.Name)
			}
			continue
		}
		switch param.In {
		case "path":
			text, err := serializeMCPPathParam(param, value)
			if err != nil {
				return nil, err
			}
			path = strings.ReplaceAll(path, "{"+param.Name+"}", text)
		case "query":
			parts, err := serializeMCPQueryParam(param, value)
			if err != nil {
				return nil, err
			}
			for _, part := range parts {
				query.Add(part.name, part.value)
			}
		case "header":
			text, err := serializeMCPHeaderParam(param, value)
			if err != nil {
				return nil, err
			}
			headers[param.Name] = text
		case "cookie":
			text, err := serializeMCPCookieParam(param, value)
			if err != nil {
				return nil, err
			}
			cookies = append(cookies, text)
		}
	}
	if len(cookies) > 0 {
		headers["Cookie"] = strings.Join(cookies, "; ")
	}
	rawURL := t.APIName + path
	if qs := query.Encode(); qs != "" {
		rawURL += "?" + qs
	}

	var body any
	if value, ok := args["body"]; ok {
		body = value
	} else if t.BodyRequired {
		return nil, errors.New("missing required parameter \"body\"")
	}

	return &HTTPRequest{
		Method:      t.Method,
		URI:         rawURL,
		Headers:     headers,
		Body:        body,
		ContentType: t.BodyContentType,
	}, nil
}

type mcpQueryParam struct {
	name  string
	value string
}

func mcpParamDescriptor(p Param) openapiparam.Param {
	return openapiparam.Param{
		Name:          p.Name,
		In:            p.In,
		Type:          p.Type,
		Style:         p.Style,
		Explode:       p.Explode,
		AllowReserved: p.AllowReserved,
	}
}

func serializeMCPPathParam(p Param, value any) (string, error) {
	if isObjectValue(value) {
		return "", fmt.Errorf("parameter %q: object values are not supported", p.Name)
	}
	paramValue, err := mcpParamValue(p, value)
	if err != nil {
		return "", fmt.Errorf("parameter %q: %w", p.Name, err)
	}
	if p.Type == "array" && openapiparam.DefaultParamStyle(mcpParamDescriptor(p)) != "simple" {
		return "", fmt.Errorf("parameter %q: %w", p.Name, openapiparam.UnsupportedArrayStyleError(mcpParamDescriptor(p)))
	}
	return openapiparam.SerializePathParam(mcpParamDescriptor(p), paramValue)
}

func serializeMCPQueryParam(p Param, value any) ([]mcpQueryParam, error) {
	if isObjectValue(value) {
		return nil, fmt.Errorf("parameter %q: object values are not supported", p.Name)
	}
	paramValue, err := mcpParamValue(p, value)
	if err != nil {
		return nil, fmt.Errorf("parameter %q: %w", p.Name, err)
	}
	if p.Type == "array" {
		switch openapiparam.DefaultParamStyle(mcpParamDescriptor(p)) {
		case "form", "spaceDelimited", "pipeDelimited":
		default:
			return nil, fmt.Errorf("parameter %q: %w", p.Name, openapiparam.UnsupportedArrayStyleError(mcpParamDescriptor(p)))
		}
	}
	parts, err := openapiparam.SerializeQueryParam(mcpParamDescriptor(p), paramValue)
	if err != nil {
		return nil, err
	}
	out := make([]mcpQueryParam, 0, len(parts))
	for _, part := range parts {
		out = append(out, mcpQueryParam{name: part.Name, value: part.Value})
	}
	return out, nil
}

func serializeMCPHeaderParam(p Param, value any) (string, error) {
	if isObjectValue(value) {
		return "", fmt.Errorf("parameter %q: object values are not supported", p.Name)
	}
	paramValue, err := mcpParamValue(p, value)
	if err != nil {
		return "", fmt.Errorf("parameter %q: %w", p.Name, err)
	}
	if p.Type == "array" && openapiparam.DefaultParamStyle(mcpParamDescriptor(p)) != "simple" {
		return "", fmt.Errorf("parameter %q: %w", p.Name, openapiparam.UnsupportedArrayStyleError(mcpParamDescriptor(p)))
	}
	values, err := openapiparam.SerializeHeaderParam(mcpParamDescriptor(p), paramValue)
	if err != nil {
		return "", err
	}
	return strings.Join(values, ","), nil
}

func serializeMCPCookieParam(p Param, value any) (string, error) {
	if isObjectValue(value) {
		return "", fmt.Errorf("parameter %q: object values are not supported", p.Name)
	}
	paramValue, err := mcpParamValue(p, value)
	if err != nil {
		return "", fmt.Errorf("parameter %q: %w", p.Name, err)
	}
	if p.Type == "array" && openapiparam.DefaultParamStyle(mcpParamDescriptor(p)) != "form" {
		return "", fmt.Errorf("parameter %q: %w", p.Name, openapiparam.UnsupportedArrayStyleError(mcpParamDescriptor(p)))
	}
	values, err := openapiparam.SerializeCookieParam(mcpParamDescriptor(p), paramValue)
	if err != nil {
		return "", err
	}
	return strings.Join(values, "; "), nil
}

func mcpParamValue(p Param, value any) (openapiparam.Value, error) {
	if p.Type != "array" {
		text, ok := scalarValueString(value)
		if !ok {
			return openapiparam.Value{}, errors.New("unsupported value type")
		}
		return openapiparam.ScalarValue(text), nil
	}
	items, ok := value.([]any)
	if !ok {
		text, ok := scalarValueString(value)
		if !ok {
			return openapiparam.Value{}, errors.New("unsupported value type")
		}
		return openapiparam.ArrayValue(openapiparam.NormalizeArrayValues([]string{text})), nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := scalarValueString(item)
		if !ok {
			return openapiparam.Value{}, errors.New("array items must be scalar values")
		}
		out = append(out, text)
	}
	return openapiparam.ArrayValue(out), nil
}

func isObjectValue(v any) bool {
	_, ok := v.(map[string]any)
	return ok
}

func scalarValueString(v any) (string, bool) {
	switch t := v.(type) {
	case string:
		return t, true
	case json.Number:
		return t.String(), true
	case bool:
		return strconv.FormatBool(t), true
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64), true
	case float32:
		return strconv.FormatFloat(float64(t), 'f', -1, 32), true
	case int:
		return strconv.Itoa(t), true
	case int64:
		return strconv.FormatInt(t, 10), true
	case int32:
		return strconv.FormatInt(int64(t), 10), true
	case uint:
		return strconv.FormatUint(uint64(t), 10), true
	case uint64:
		return strconv.FormatUint(t, 10), true
	case uint32:
		return strconv.FormatUint(uint64(t), 10), true
	default:
		return "", false
	}
}
