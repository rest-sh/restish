package spec

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel"
	"github.com/rest-sh/restish/v2/internal/request"
	"go.yaml.in/yaml/v3"
)

// OpenAPILoader handles OpenAPI 3.0 and 3.1 specifications.
type OpenAPILoader struct{}

func (OpenAPILoader) Priority() int { return 10 }

// Detect returns true when the content type or body look like an OpenAPI spec.
// It accepts JSON, YAML, text-ish raw files, and the official OpenAPI MIME
// types, then confirms by sniffing for an "openapi:" / `"openapi"` key.
func (OpenAPILoader) Detect(contentType string, body []byte) bool {
	ct := strings.ToLower(contentType)
	if strings.Contains(ct, "openapi") {
		return true
	}
	// Accept OpenAPI-specific MIME types and common JSON/YAML types.
	if !strings.Contains(ct, "json") &&
		!strings.Contains(ct, "yaml") &&
		!strings.HasPrefix(ct, "text/") &&
		ct != "" {
		return false
	}

	// Body sniff: look for the "openapi" field. Some generated specs write
	// the top-level openapi field late in the document, so generic JSON/YAML
	// cannot rely on a tiny prefix sniff.
	sniff := body
	low := bytes.ToLower(sniff)
	return bytes.Contains(low, []byte(`"openapi"`)) ||
		bytes.Contains(low, []byte("openapi:")) ||
		bytes.Contains(low, []byte(`"swagger"`)) ||
		bytes.Contains(low, []byte("swagger:"))
}

// Load parses body as an OpenAPI 3.x document.
func (OpenAPILoader) Load(body []byte) (*APISpec, error) {
	return OpenAPILoader{}.LoadWithOptions(body, LoadOptions{})
}

// LoadWithOptions parses body as an OpenAPI 3.x document, using source
// metadata to resolve supported external references.
func (OpenAPILoader) LoadWithOptions(body []byte, opts LoadOptions) (*APISpec, error) {
	if isSwagger2Document(body) {
		return nil, &LoadError{Errors: []string{"Swagger/OpenAPI 2.0 is not supported; Restish requires OpenAPI 3.x"}}
	}
	resolvedBody, err := resolveOpenAPIExternalRefs(body, opts)
	if err != nil {
		return nil, &LoadError{Errors: []string{err.Error()}}
	}
	doc, err := libopenapi.NewDocumentWithConfiguration(resolvedBody, openAPIConfig(opts))
	if err != nil {
		return nil, &LoadError{Errors: []string{err.Error()}}
	}
	return &APISpec{Raw: body, Document: doc}, nil
}

func isSwagger2Document(body []byte) bool {
	var doc yaml.Node
	if err := yaml.Unmarshal(body, &doc); err != nil {
		return false
	}
	root := &doc
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		root = doc.Content[0]
	}
	if root.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "swagger" && strings.TrimSpace(root.Content[i+1].Value) == "2.0" {
			return true
		}
	}
	return false
}

func openAPIConfig(opts LoadOptions) *datamodel.DocumentConfiguration {
	cfg := datamodel.NewDocumentConfiguration()
	cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg.ExtractRefsSequentially = true
	cfg.ResolveNestedRefsWithDocumentContext = true
	cfg.SkipCircularReferenceCheck = true

	if opts.LocalPath != "" {
		localPath := filepath.Clean(opts.LocalPath)
		cfg.BasePath = filepath.Dir(localPath)
		cfg.SpecFilePath = localPath
		cfg.FileFilter = []string{localPath}
		cfg.AllowFileReferences = true
	}

	if opts.SourceURL != "" {
		baseURL, err := openAPIRefBaseURL(opts.SourceURL)
		if err == nil {
			cfg.BaseURL = baseURL
			cfg.AllowRemoteReferences = true
			cfg.RemoteURLHandler = openAPIRemoteURLHandler(opts)
		}
	}

	return cfg
}

func openAPIRefBaseURL(rawURL string) (*url.URL, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported OpenAPI ref base URL scheme %q", u.Scheme)
	}
	if u.Path == "" || strings.HasSuffix(u.Path, "/") {
		return u, nil
	}
	u.Path = u.Path[:strings.LastIndex(u.Path, "/")+1]
	u.RawQuery = ""
	u.Fragment = ""
	return u, nil
}

func openAPIRemoteURLHandler(opts LoadOptions) func(string) (*http.Response, error) {
	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}
	tr := opts.Transport
	if tr == nil {
		tr = http.DefaultTransport
	}
	source, _ := url.Parse(opts.SourceURL)

	return func(rawURL string) (*http.Response, error) {
		u, err := url.Parse(rawURL)
		if err != nil {
			return nil, err
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return nil, fmt.Errorf("OpenAPI external ref %q uses unsupported scheme %q", rawURL, u.Scheme)
		}
		if source != nil && !opts.AllowCrossOrigin && !sameOrigin(source, u) {
			return nil, fmt.Errorf("OpenAPI external ref %q is not same-origin with %q", rawURL, opts.SourceURL)
		}
		if source != nil && opts.AllowCrossOrigin && isDisallowedCrossOriginHost(source.Hostname(), u.Hostname()) {
			return nil, fmt.Errorf("OpenAPI external ref %q targets a non-public host from public origin %q", rawURL, opts.SourceURL)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, err
		}
		resp, err := tr.RoundTrip(req)
		if err != nil {
			return nil, err
		}
		if resp.Body == nil {
			return resp, nil
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(io.LimitReader(resp.Body, maxSpecBytes+1))
		if err != nil {
			return nil, err
		}
		if int64(len(data)) > maxSpecBytes {
			return nil, fmt.Errorf("OpenAPI external ref %q exceeds limit of %d bytes", rawURL, maxSpecBytes)
		}
		resp.Body = io.NopCloser(bytes.NewReader(data))
		resp.ContentLength = int64(len(data))
		return resp, nil
	}
}

func sameOrigin(a, b *url.URL) bool {
	return request.SameOrigin(a, b)
}

type openAPIRefSource struct {
	url       string
	localPath string
}

type openAPIRefResolver struct {
	opts  LoadOptions
	docs  map[string]*yaml.Node
	depth int
}

func resolveOpenAPIExternalRefs(body []byte, opts LoadOptions) ([]byte, error) {
	if opts.SourceURL == "" && opts.LocalPath == "" {
		return body, nil
	}
	if opts.SourceURL != "" {
		tracef(opts.Trace, "Resolving OpenAPI external refs from %s", opts.SourceURL)
	} else {
		tracef(opts.Trace, "Resolving OpenAPI external refs from %s", opts.LocalPath)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	root := &doc
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		root = doc.Content[0]
	}
	resolver := &openAPIRefResolver{opts: opts, docs: map[string]*yaml.Node{}}
	source := openAPIRefSource{url: opts.SourceURL, localPath: opts.LocalPath}
	changed, err := resolver.resolveNode(root, source)
	if err != nil {
		return nil, err
	}
	if !changed {
		return body, nil
	}
	return yaml.Marshal(&doc)
}

func (r *openAPIRefResolver) resolveNode(n *yaml.Node, source openAPIRefSource) (bool, error) {
	if n == nil {
		return false, nil
	}
	if r.depth > 100 {
		return false, fmt.Errorf("OpenAPI external ref resolution exceeded maximum depth")
	}
	if ref := mappingRefValue(n); ref != "" && !strings.HasPrefix(ref, "#") {
		originalSiblings := cloneRefSiblings(n)
		target, targetSource, err := r.resolveRef(ref, source)
		if err != nil {
			return false, err
		}
		*n = *cloneYAMLNode(target)
		mergeRefSiblings(n, originalSiblings)
		r.depth++
		defer func() { r.depth-- }()
		if _, err := r.resolveNode(n, targetSource); err != nil {
			return false, err
		}
		return true, nil
	}

	changed := false
	for _, child := range n.Content {
		childChanged, err := r.resolveNode(child, source)
		if err != nil {
			return false, err
		}
		changed = changed || childChanged
	}
	return changed, nil
}

func mappingRefValue(n *yaml.Node) string {
	if n == nil || n.Kind != yaml.MappingNode {
		return ""
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == "$ref" && n.Content[i+1].Kind == yaml.ScalarNode {
			return n.Content[i+1].Value
		}
	}
	return ""
}

func cloneRefSiblings(n *yaml.Node) []*yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	var out []*yaml.Node
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == "$ref" {
			continue
		}
		out = append(out, cloneYAMLNode(n.Content[i]), cloneYAMLNode(n.Content[i+1]))
	}
	return out
}

func mergeRefSiblings(n *yaml.Node, siblings []*yaml.Node) {
	if len(siblings) == 0 {
		return
	}
	if n.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(siblings); i += 2 {
		n.Content = removeMappingKey(n.Content, siblings[i].Value)
		n.Content = append(n.Content, siblings[i], siblings[i+1])
	}
}

func removeMappingKey(content []*yaml.Node, key string) []*yaml.Node {
	out := content[:0]
	for i := 0; i+1 < len(content); i += 2 {
		if content[i].Value == key {
			continue
		}
		out = append(out, content[i], content[i+1])
	}
	return out
}

func (r *openAPIRefResolver) resolveRef(ref string, source openAPIRefSource) (*yaml.Node, openAPIRefSource, error) {
	root, fragment, _ := strings.Cut(ref, "#")
	if root == "" {
		return nil, source, fmt.Errorf("OpenAPI external ref %q has no external document", ref)
	}
	key, targetSource, err := r.resolveRefRoot(root, source)
	if err != nil {
		return nil, source, err
	}
	doc, err := r.loadExternalDoc(key, targetSource)
	if err != nil {
		return nil, source, err
	}
	target, err := yamlPointer(doc, fragment)
	if err != nil {
		return nil, source, fmt.Errorf("OpenAPI external ref %q: %w", ref, err)
	}
	return target, targetSource, nil
}

func (r *openAPIRefResolver) resolveRefRoot(root string, source openAPIRefSource) (string, openAPIRefSource, error) {
	if u, err := url.Parse(root); err == nil && u.Scheme != "" {
		switch u.Scheme {
		case "file":
			if source.localPath == "" {
				return "", openAPIRefSource{}, fmt.Errorf("OpenAPI external ref %q uses local file access from remote source %q", root, source.url)
			}
			path, err := localPathFromSource(u.String())
			if err != nil {
				return "", openAPIRefSource{}, err
			}
			path = filepath.Clean(path)
			return "file:" + path, openAPIRefSource{localPath: path}, nil
		case "http", "https":
			return u.String(), openAPIRefSource{url: u.String()}, nil
		default:
			return "", openAPIRefSource{}, fmt.Errorf("OpenAPI external ref %q uses unsupported scheme %q", root, u.Scheme)
		}
	}

	if source.localPath != "" {
		path := filepath.Clean(filepath.Join(filepath.Dir(source.localPath), root))
		return "file:" + path, openAPIRefSource{localPath: path}, nil
	}

	if source.url != "" {
		base, err := openAPIRefBaseURL(source.url)
		if err != nil {
			return "", openAPIRefSource{}, err
		}
		rel, err := url.Parse(root)
		if err != nil {
			return "", openAPIRefSource{}, err
		}
		resolved := base.ResolveReference(rel).String()
		return resolved, openAPIRefSource{url: resolved}, nil
	}

	return "", openAPIRefSource{}, fmt.Errorf("OpenAPI external ref %q has no source to resolve against", root)
}

func (r *openAPIRefResolver) loadExternalDoc(key string, source openAPIRefSource) (*yaml.Node, error) {
	if doc := r.docs[key]; doc != nil {
		return doc, nil
	}

	var data []byte
	var err error
	switch {
	case source.localPath != "":
		data, err = os.ReadFile(source.localPath)
	case source.url != "":
		tracef(r.opts.Trace, "GET OpenAPI external ref %s", source.url)
		data, err = r.fetchRemoteDoc(source.url)
	default:
		err = fmt.Errorf("OpenAPI external ref has no resolved source")
	}
	if err != nil {
		return nil, err
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	root := &doc
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		root = doc.Content[0]
	}
	r.docs[key] = root
	return root, nil
}

func (r *openAPIRefResolver) fetchRemoteDoc(rawURL string) ([]byte, error) {
	opts := r.opts
	opts.SourceURL = r.opts.SourceURL
	if opts.SourceURL == "" {
		opts.SourceURL = rawURL
	}
	handler := openAPIRemoteURLHandler(opts)
	resp, err := handler(rawURL)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", rawURL, err)
	}
	if resp == nil {
		return nil, fmt.Errorf("GET %s: no response", rawURL)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GET %s: %s", rawURL, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxSpecBytes+1))
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", rawURL, err)
	}
	if int64(len(data)) > maxSpecBytes {
		return nil, fmt.Errorf("OpenAPI external ref %q exceeds limit of %d bytes", rawURL, maxSpecBytes)
	}
	return data, nil
}

func yamlPointer(root *yaml.Node, fragment string) (*yaml.Node, error) {
	if fragment == "" {
		return root, nil
	}
	pointer, err := url.PathUnescape(fragment)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(pointer, "/") {
		return nil, fmt.Errorf("unsupported fragment %q", fragment)
	}
	current := root
	for _, token := range strings.Split(pointer[1:], "/") {
		token = strings.ReplaceAll(strings.ReplaceAll(token, "~1", "/"), "~0", "~")
		switch current.Kind {
		case yaml.MappingNode:
			var next *yaml.Node
			for i := 0; i+1 < len(current.Content); i += 2 {
				if current.Content[i].Value == token {
					next = current.Content[i+1]
					break
				}
			}
			if next == nil {
				return nil, fmt.Errorf("JSON pointer token %q not found", token)
			}
			current = next
		case yaml.SequenceNode:
			idx, err := strconv.Atoi(token)
			if err != nil || idx < 0 || idx >= len(current.Content) {
				return nil, fmt.Errorf("JSON pointer index %q not found", token)
			}
			current = current.Content[idx]
		default:
			return nil, fmt.Errorf("JSON pointer token %q cannot descend into %s", token, current.ShortTag())
		}
	}
	return current, nil
}

func cloneYAMLNode(n *yaml.Node) *yaml.Node {
	if n == nil {
		return nil
	}
	clone := *n
	clone.Content = make([]*yaml.Node, len(n.Content))
	for i, child := range n.Content {
		clone.Content[i] = cloneYAMLNode(child)
	}
	return &clone
}

// LoadError wraps one or more errors returned by the libopenapi parser.
type LoadError struct {
	Errors []string
}

func (e *LoadError) Error() string {
	return "openapi: " + strings.Join(e.Errors, "; ")
}
