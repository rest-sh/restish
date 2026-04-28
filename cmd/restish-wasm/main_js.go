//go:build js && wasm

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"syscall/js"

	"github.com/google/shlex"
	"github.com/rest-sh/restish/v2/internal/content"
	"github.com/rest-sh/restish/v2/internal/filter"
	"github.com/rest-sh/restish/v2/internal/hypermedia"
	"github.com/rest-sh/restish/v2/internal/input"
)

type wasmResult struct {
	Output string `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}

type plan struct {
	Method       string
	URL          string
	BodyArgs     []string
	Headers      []string
	Query        []string
	ContentType  string
	OutputFormat string
	Filter       string
	Raw          bool
	HeadersOnly  bool
	NoPaginate   bool
	MaxPages     int
	MaxItems     int
}

var (
	reg     = content.Default()
	parsers = hypermedia.DefaultParsers()
)

type responseDoc struct {
	Proto   string
	Status  int
	Headers map[string]string
	URL     string
	Links   map[string]any
	Body    any
	Raw     []byte
}

func main() {
	js.Global().Set("restishWasmRun", js.FuncOf(runJS))
	select {}
}

func runJS(_ js.Value, args []js.Value) any {
	command := ""
	if len(args) > 0 {
		command = args[0].String()
	}
	return promise(func() wasmResult {
		out, err := run(command)
		if err != nil {
			return wasmResult{Error: err.Error()}
		}
		return wasmResult{Output: out}
	})
}

func promise(fn func() wasmResult) js.Value {
	promiseCtor := js.Global().Get("Promise")
	return promiseCtor.New(js.FuncOf(func(_ js.Value, args []js.Value) any {
		resolve := args[0]
		go func() {
			result := fn()
			data, _ := json.Marshal(result)
			resolve.Invoke(string(data))
		}()
		return nil
	}))
}

func run(command string) (string, error) {
	p, err := parse(command)
	if err != nil {
		return "", err
	}
	return execute(p)
}

func parse(command string) (plan, error) {
	words, err := shlex.Split(strings.TrimSpace(strings.TrimPrefix(command, "$")))
	if err != nil {
		return plan{}, err
	}
	if len(words) == 0 || words[0] != "restish" {
		return plan{}, fmt.Errorf("command must start with restish")
	}

	p := plan{Method: "GET", MaxPages: 25}
	var args []string
	for i := 1; i < len(words); i++ {
		word := words[i]
		name, value, hasInline := strings.Cut(word, "=")
		if strings.HasPrefix(word, "--") && hasInline {
			word = name
		}
		nextValue := func() (string, error) {
			if hasInline {
				return value, nil
			}
			if i+1 >= len(words) {
				return "", fmt.Errorf("%s needs a value", word)
			}
			i++
			return words[i], nil
		}

		switch word {
		case "-H", "--rsh-header":
			v, err := nextValue()
			if err != nil {
				return plan{}, err
			}
			p.Headers = append(p.Headers, v)
		case "-q", "--rsh-query":
			v, err := nextValue()
			if err != nil {
				return plan{}, err
			}
			p.Query = append(p.Query, v)
		case "-c", "--rsh-content-type":
			p.ContentType, err = nextValue()
			if err != nil {
				return plan{}, err
			}
		case "-o", "--rsh-output-format":
			p.OutputFormat, err = nextValue()
			if err != nil {
				return plan{}, err
			}
		case "-f", "--rsh-filter":
			p.Filter, err = nextValue()
			if err != nil {
				return plan{}, err
			}
		case "--rsh-max-pages":
			v, err := nextValue()
			if err != nil {
				return plan{}, err
			}
			_, _ = fmt.Sscan(v, &p.MaxPages)
		case "--rsh-max-items":
			v, err := nextValue()
			if err != nil {
				return plan{}, err
			}
			_, _ = fmt.Sscan(v, &p.MaxItems)
		case "-r", "--rsh-raw":
			p.Raw = true
		case "--rsh-headers":
			p.HeadersOnly = true
		case "--rsh-no-paginate":
			p.NoPaginate = true
		case "--rsh-collect", "--rsh-ignore-status-code":
			// Accepted for docs examples; behavior is already compatible enough.
		default:
			if strings.HasPrefix(word, "-") {
				return plan{}, fmt.Errorf("%s is not supported in the WASM prototype", word)
			}
			args = append(args, word)
		}
	}

	if len(args) == 0 {
		return plan{}, fmt.Errorf("missing URL or command")
	}
	switch strings.ToLower(args[0]) {
	case "get", "head", "options", "post", "put", "patch", "delete":
		p.Method = strings.ToUpper(args[0])
		if len(args) < 2 {
			return plan{}, fmt.Errorf("missing URL")
		}
		p.URL = args[1]
		p.BodyArgs = args[2:]
	case "example":
		if len(args) < 2 {
			return plan{}, fmt.Errorf("missing example operation")
		}
		switch args[1] {
		case "list-images":
			p.URL = "https://api.rest.sh/images"
			p.BodyArgs = args[2:]
		case "get-image":
			format := "jpeg"
			if len(args) > 2 {
				format = args[2]
			}
			p.URL = "https://api.rest.sh/images/" + url.PathEscape(format)
			if len(args) > 3 {
				p.BodyArgs = args[3:]
			}
		default:
			return plan{}, fmt.Errorf("example %s is not mapped in the WASM prototype", args[1])
		}
	default:
		p.URL = args[0]
		p.BodyArgs = args[1:]
	}
	if _, err := url.ParseRequestURI(p.URL); err != nil {
		return plan{}, fmt.Errorf("invalid URL: %w", err)
	}
	return p, nil
}

func execute(p plan) (string, error) {
	body, err := requestBody(p)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(context.Background(), p.Method, p.URL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json, application/yaml;q=0.5, text/*;q=0.2")
	for _, h := range p.Headers {
		name, value, ok := strings.Cut(h, ":")
		if !ok {
			return "", fmt.Errorf("invalid header %q: expected Name: Value", h)
		}
		req.Header.Set(strings.TrimSpace(name), strings.TrimSpace(value))
	}
	if len(body) > 0 {
		ct := p.ContentType
		if ct == "" {
			ct = "json"
		}
		mime := reg.MIMETypeForName(ct)
		if mime == "" {
			mime = ct
		}
		req.Header.Set("Content-Type", mime)
	}
	if len(p.Query) > 0 {
		q := req.URL.Query()
		for _, item := range p.Query {
			key, value, ok := strings.Cut(item, "=")
			if !ok {
				return "", fmt.Errorf("invalid query param %q: expected key=value", item)
			}
			q.Add(key, value)
		}
		req.URL.RawQuery = q.Encode()
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	normalized, err := normalizeBrowserResponse(resp)
	if err != nil {
		return "", err
	}
	if err := addLinks(normalized); err != nil {
		return "", err
	}
	if p.Method == "GET" && !p.NoPaginate {
		if err := paginate(p, normalized); err != nil {
			return "", err
		}
	}
	return render(p, normalized)
}

func requestBody(p plan) ([]byte, error) {
	bodyValue, err := input.Body(strings.NewReader(""), true, p.BodyArgs, p.ContentType)
	if err != nil || bodyValue == nil {
		return nil, err
	}
	ct := p.ContentType
	if ct == "" {
		ct = "json"
	}
	mime := reg.MIMETypeForName(ct)
	if mime == "" {
		mime = ct
	}
	data, _, err := reg.EncodeWithType(mime, bodyValue)
	return data, err
}

func normalizeBrowserResponse(resp *http.Response) (*responseDoc, error) {
	// Browser fetch exposes decompressed bytes. Drop Content-Encoding so the
	// normal Restish decoder does not try to decompress them a second time.
	resp.Header.Del("Content-Encoding")
	defer resp.Body.Close()
	headers := make(map[string]string, len(resp.Header))
	for k, vals := range resp.Header {
		if len(vals) > 0 {
			headers[k] = vals[0]
		}
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var body any
	if len(raw) > 0 {
		body, err = reg.Decode(resp.Header.Get("Content-Type"), raw)
		if err != nil {
			return nil, err
		}
	}
	url := ""
	if resp.Request != nil && resp.Request.URL != nil {
		url = resp.Request.URL.String()
	}
	return &responseDoc{
		Proto:   resp.Proto,
		Status:  resp.StatusCode,
		Headers: headers,
		URL:     url,
		Body:    body,
		Raw:     raw,
	}, nil
}

func addLinks(resp *responseDoc) error {
	if resp == nil || resp.URL == "" {
		return nil
	}
	base, err := url.Parse(resp.URL)
	if err != nil {
		return err
	}
	headers := make(http.Header, len(resp.Headers))
	for k, v := range resp.Headers {
		headers.Set(k, v)
	}
	resp.Links = map[string]any{}
	for rel, uri := range hypermedia.Parse(base, headers, resp.Body, parsers) {
		resp.Links[rel] = uri
	}
	if len(resp.Links) == 0 {
		resp.Links = nil
	}
	return nil
}

func paginate(p plan, first *responseDoc) error {
	items, ok := first.Body.([]any)
	if !ok || first.Links == nil {
		return nil
	}
	pages := 1
	for {
		next, _ := first.Links["next"].(string)
		if next == "" || (p.MaxPages > 0 && pages >= p.MaxPages) || (p.MaxItems > 0 && len(items) >= p.MaxItems) {
			break
		}
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, next, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "application/json, application/yaml;q=0.5, text/*;q=0.2")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		page, err := normalizeBrowserResponse(resp)
		if err != nil {
			return err
		}
		if err := addLinks(page); err != nil {
			return err
		}
		pageItems, ok := page.Body.([]any)
		if !ok {
			break
		}
		items = append(items, pageItems...)
		first.Links = page.Links
		pages++
	}
	if p.MaxItems > 0 && len(items) > p.MaxItems {
		items = items[:p.MaxItems]
	}
	first.Body = items
	return nil
}

func render(p plan, resp *responseDoc) (string, error) {
	if p.HeadersOnly {
		p.Filter = "headers"
	}
	value := any(nil)
	filtered := false
	if p.Filter != "" {
		doc := map[string]any{
			"proto":   resp.Proto,
			"status":  resp.Status,
			"headers": resp.Headers,
			"links":   resp.Links,
			"body":    resp.Body,
		}
		v, err := filter.Apply(p.Filter, doc, filter.LangAuto)
		if err != nil {
			return "", err
		}
		value = v
		filtered = true
	}
	if p.Raw {
		if filtered {
			return rawOutput(value), nil
		}
		return string(resp.Raw), nil
	}

	var buf bytes.Buffer
	if filtered {
		data, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return "", err
		}
		buf.Write(data)
		buf.WriteByte('\n')
		return buf.String(), nil
	}

	name := p.OutputFormat
	if name == "" {
		name = "readable"
	}
	switch name {
	case "readable":
		writeReadable(&buf, resp)
	case "json":
		data, err := json.MarshalIndent(resp.Body, "", "  ")
		if err != nil {
			return "", err
		}
		buf.Write(data)
		buf.WriteByte('\n')
	case "ndjson":
		items, ok := resp.Body.([]any)
		if !ok {
			items = []any{resp.Body}
		}
		for _, item := range items {
			data, err := json.Marshal(item)
			if err != nil {
				return "", err
			}
			buf.Write(data)
			buf.WriteByte('\n')
		}
	default:
		return "", fmt.Errorf("unknown output format %q in WASM prototype", name)
	}
	return buf.String(), nil
}

func writeReadable(buf *bytes.Buffer, resp *responseDoc) {
	fmt.Fprintf(buf, "%s %d %s\n", proto(resp.Proto), resp.Status, http.StatusText(resp.Status))
	for _, key := range sortedKeys(resp.Headers) {
		fmt.Fprintf(buf, "%s: %s\n", key, resp.Headers[key])
	}
	buf.WriteByte('\n')
	if resp.Body == nil {
		return
	}
	data, err := json.MarshalIndent(resp.Body, "", "  ")
	if err != nil {
		fmt.Fprintln(buf, resp.Body)
		return
	}
	buf.Write(data)
	buf.WriteByte('\n')
}

func proto(value string) string {
	if value == "" {
		return "HTTP/2.0"
	}
	return value
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && strings.ToLower(keys[j]) < strings.ToLower(keys[j-1]); j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

func rawOutput(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case []any:
		var b strings.Builder
		for _, item := range v {
			b.WriteString(rawScalar(item))
			b.WriteByte('\n')
		}
		return b.String()
	default:
		return rawScalar(v) + "\n"
	}
}

func rawScalar(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprint(v)
		}
		return string(data)
	}
}
