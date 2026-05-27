package main

import (
	"fmt"
	"net/url"
)

func schemaURL(resp *httpResponse) string {
	if resp == nil {
		return ""
	}
	if href, ok := resp.Links["describedby"].(string); ok {
		return href
	}
	body, ok := resp.Body.(map[string]any)
	if !ok {
		return ""
	}
	raw, ok := body["$schema"].(string)
	if !ok || raw == "" {
		return ""
	}
	if resp.URL == "" {
		return raw
	}
	base, err := url.Parse(resp.URL)
	if err != nil {
		return raw
	}
	ref, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	return base.ResolveReference(ref).String()
}

func (a *app) schemaExample(f *File) any {
	if f == nil || f.Schema == "" || a == nil || a.client == nil {
		return nil
	}
	if a.schemaCache == nil {
		a.schemaCache = map[string]any{}
	}
	if a.schemaMisses == nil {
		a.schemaMisses = map[string]bool{}
	}
	if example, ok := a.schemaCache[f.Schema]; ok {
		return example
	}
	if a.schemaMisses[f.Schema] {
		return nil
	}
	resp, err := a.client.request("GET", f.Schema, nil, nil)
	if err != nil || resp.Error != "" || resp.Status >= 400 {
		a.schemaMisses[f.Schema] = true
		return nil
	}
	example := jsonSchemaExample(resp.Body)
	if example == nil {
		a.schemaMisses[f.Schema] = true
		return nil
	}
	a.schemaCache[f.Schema] = example
	return example
}

func jsonSchemaExample(schema any) any {
	m, ok := schema.(map[string]any)
	if !ok {
		return nil
	}
	if props, ok := m["properties"].(map[string]any); ok {
		obj := map[string]any{}
		for name, raw := range props {
			if example := jsonSchemaExample(raw); example != nil {
				obj[name] = example
			}
		}
		return obj
	}
	switch fmt.Sprintf("%v", m["type"]) {
	case "object":
		return map[string]any{}
	case "array":
		if items, ok := m["items"]; ok {
			if example := jsonSchemaExample(items); example != nil {
				return []any{example}
			}
		}
		return []any{}
	case "string":
		return "string"
	case "integer":
		return 1
	case "number":
		return 1.0
	case "boolean":
		return true
	case "null":
		return nil
	default:
		if enum, ok := m["enum"].([]any); ok && len(enum) > 0 {
			return enum[0]
		}
	}
	return nil
}
