package spec

import (
	"fmt"

	"github.com/pb33f/libopenapi"
	"github.com/rest-sh/restish/v2/internal/plugin"
	pluginwire "github.com/rest-sh/restish/v2/plugin"
)

// PluginLoader is a spec.Loader backed by a hook plugin. The plugin receives a
// CBOR message on stdin with the URL, content type, and raw body; it returns a
// CBOR message containing an OpenAPI spec in JSON or YAML form.
type PluginLoader struct {
	PluginPath   string
	PluginName   string
	ContentTypes []string
}

// Detect returns true when contentType matches one of the plugin's declared
// loader content types.
func (l PluginLoader) Detect(contentType string, _ []byte) bool {
	for _, ct := range l.ContentTypes {
		if ct == contentType {
			return true
		}
	}
	return false
}

// Load calls the plugin and parses the returned OpenAPI spec.
// The returned APISpec has ContentType and Raw set from the plugin's response,
// allowing the plugin to produce a normalized form different from the input.
func (l PluginLoader) Load(body []byte) (*APISpec, error) {
	return l.LoadWithOptions(body, LoadOptions{})
}

// LoadWithOptions calls the plugin with source metadata from discovery or cache
// when it is available.
func (l PluginLoader) LoadWithOptions(body []byte, opts LoadOptions) (*APISpec, error) {
	in := pluginwire.LoaderRequest{
		Type:        "loader",
		Body:        body,
		ContentType: opts.ContentType,
		SourceURL:   opts.SourceURL,
		LocalPath:   opts.LocalPath,
	}
	var out pluginwire.LoaderResponse
	if err := plugin.CallHookContext(opts.Context, l.PluginPath, in, &out); err != nil {
		return nil, fmt.Errorf("loader plugin %s: %w", l.PluginName, err)
	}

	var outBody []byte
	switch v := out.Body.(type) {
	case []byte:
		outBody = v
	case string:
		outBody = []byte(v)
	}
	if len(outBody) == 0 {
		return nil, fmt.Errorf("loader plugin %s: empty body in response", l.PluginName)
	}

	doc, err := libopenapi.NewDocument(outBody)
	if err != nil {
		return nil, fmt.Errorf("loader plugin %s: parse OpenAPI: %w", l.PluginName, err)
	}
	return &APISpec{
		ContentType: out.ContentType,
		Raw:         outBody,
		Document:    doc,
	}, nil
}

// Priority returns a high priority so plugin loaders are tried before built-in
// loaders for the same content type.
func (l PluginLoader) Priority() int { return 100 }
