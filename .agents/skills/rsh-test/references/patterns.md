# Restish Test Patterns

Load this when writing a new Restish test and you need a concrete shape. Prefer nearby existing tests over these snippets when the package already has a clearer local pattern.

## Helper Vocabulary

When several tests repeat the same OpenAPI, config, mock server, CLI command, and assertion setup, prefer a small Go helper vocabulary over an external scenario DSL. The helper should make the scenario easier to scan while keeping important behavior visible in the test.

For example, repeated generated-command tests may benefit from helpers shaped roughly like this:

```go
api := newMockAPI(t).
	WithOpenAPI(openAPISpec(
		serverURL(),
		getOperation("/items", "listItems", queryParam("limit", "integer")),
	)).
	WithJSONRoute("GET", "/items", `[{"id":"a"},{"id":"b"}]`)

app := newTestApp(t)
app.WithAPIConfig("tapi", api.URL)
app.Run("api", "sync", "tapi")
app.Run("tapi", "list-items", "--limit", "2", "-o", "json")

app.RequireStdoutContains(`"id": "a"`, `"id": "b"`)
api.RequireLastRequest(wantRequest{
	Method: "GET",
	Path:   "/items",
	Query:  map[string]string{"limit": "2"},
})
```

Do not add exactly these helpers unless they fit the package and existing style. The useful pattern is setup, run command, assert observable output/request/state. Good candidates are tiny OpenAPI operation builders, mock API servers that serve both specs and endpoint routes, config/profile/auth constructors, CLI run helpers, JSON stdout assertions, and request assertions for method/path/query/header/body.

## Generated CLI Command

Use this for OpenAPI-generated command behavior: spec discovery, command arguments, request construction, response rendering, and cache interactions.

```go
func TestGeneratedCommandListsItems(t *testing.T) {
	mux := http.NewServeMux()
	var spec string
	mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, spec)
	})
	mux.HandleFunc("/items", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("limit"); got != "2" {
			t.Fatalf("limit = %q, want 2", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"id":"a"},{"id":"b"}]`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	spec = fmt.Sprintf(`{
			"openapi": "3.1.0",
			"info": {"title": "Test API", "version": "1.0"},
			"servers": [{"url": %q}],
			"paths": {
				"/items": {"get": {
					"operationId": "listItems",
					"parameters": [{"name": "limit", "in": "query", "schema": {"type": "integer"}}],
					"responses": {"200": {"description": "OK"}}
				}}
			}
		}`, srv.URL)

	app := newTestApp(t)
	app.WriteConfig(fmt.Sprintf(`{"apis":{"tapi":{"base_url":%q}}}`, srv.URL))
	app.Run("api", "sync", "tapi")
	app.Run("tapi", "list-items", "--limit", "2", "-o", "json")

	requireContains(t, app.Stdout.String(), `"id": "a"`, `"id": "b"`)
}
```

If the spec needs its own server URL in `servers`, generate the spec after `httptest.NewServer` or use existing helpers such as `setupGeneratedEnv`.

If multiple tests grow copies of this shape, extract the repeated setup into narrow helpers rather than introducing Gherkin/Cucumber step definitions. Keep route behavior and key assertions in the test body unless hiding them makes failures clearer.

## Generic HTTP Command

Use fake transport when the request shape matters more than server behavior.

```go
func TestProfileHeaderIsSent(t *testing.T) {
	cfg := `{"apis":{"myapi":{"base_url":"https://api.example.com","headers":["X-Token: secret"]}}}`
	app, rr := newAPIRecorderApp(t, cfg)

	app.Run("get", "myapi/items")

	if got := rr.Last().Header.Get("X-Token"); got != "secret" {
		t.Fatalf("X-Token = %q, want secret", got)
	}
}
```

Use `httptest.Server` when redirects, compression, cookies, streaming, TLS, pagination links, or response timing are part of the behavior.

## Failure Path

Use `RunErr` or direct `CLI.Run` when the exit/error behavior is the result.

```go
func TestBadFlagReportsHelpfulError(t *testing.T) {
	app := newTestApp(t)

	err := app.RunErr("get", "--rsh-timeout", "nope", "https://api.example.com")
	if err == nil {
		t.Fatal("expected error")
	}
	requireContains(t, app.Stderr.String(), "invalid timeout")
}
```

Prefer checking the exit code type when the code is the contract. Prefer one or two message substrings when the exact wording around them may change.

## Output Assertions

Decode JSON when layout is incidental:

```go
var got map[string]any
if err := json.Unmarshal(app.Stdout.Bytes(), &got); err != nil {
	t.Fatalf("decode output: %v\n%s", err, app.Stdout.String())
}
if got["status"] != float64(200) {
	t.Fatalf("status = %v, want 200", got["status"])
}
```

Use exact strings for raw output, lines, and protocol framing:

```go
if got, want := out.String(), "data: {\"n\":1}\n\n"; got != want {
	t.Fatalf("stream output = %q, want %q", got, want)
}
```

Use `testdata/` fixture files for large stable formatter/help output. There is
no automatic `-update` flag; regenerate fixtures deliberately when output
changes intentionally:

```go
want := readFile(t, "testdata/list-items.txt")
if got := out.String(); got != want {
	t.Fatalf("output mismatch\nwant:\n%s\n\ngot:\n%s", want, got)
}
```

## Table-Driven Unit Test

Use a table when the body is nearly identical across cases.

```go
func TestStatusToExitCode(t *testing.T) {
	tests := []struct {
		name   string
		status int
		want   int
	}{
		{name: "success", status: 200, want: 0},
		{name: "redirect", status: 302, want: 3},
		{name: "client error", status: 404, want: 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StatusToExitCode(tt.status); got != tt.want {
				t.Fatalf("StatusToExitCode(%d) = %d, want %d", tt.status, got, tt.want)
			}
		})
	}
}
```

Split into separate tests when rows need different setup, different helpers, or different failure interpretation.

## Fuzz Harness

Use fuzzing for parsers and other input-heavy code. Keep assertions about invariants, panics, round-trips, and accepted/rejected behavior.

```go
func FuzzParseShorthand(f *testing.F) {
	for _, seed := range []string{
		"name: Alice",
		"tags[]: docs, active: true",
		"payload: @file.json",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		_, _ = parseShorthand(input)
	})
}
```

When a fuzz input exposes a real bug, keep the saved corpus or convert the case into a small named regression test if that makes the behavior easier to understand.
