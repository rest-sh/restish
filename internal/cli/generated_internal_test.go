package cli

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel"
	v2high "github.com/pb33f/libopenapi/datamodel/high/v2"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/index"

	"github.com/rest-sh/restish/v2/internal/config"
	"github.com/rest-sh/restish/v2/internal/spec"
)

type failingDocument struct {
	err error
}

func (d failingDocument) GetVersion() string                                 { return "3.1.0" }
func (d failingDocument) GetRolodex() *index.Rolodex                         { return nil }
func (d failingDocument) GetSpecInfo() *datamodel.SpecInfo                   { return nil }
func (d failingDocument) SetConfiguration(*datamodel.DocumentConfiguration)  {}
func (d failingDocument) GetConfiguration() *datamodel.DocumentConfiguration { return nil }
func (d failingDocument) BuildV2Model() (*libopenapi.DocumentModel[v2high.Swagger], error) {
	return nil, errors.New("unsupported")
}
func (d failingDocument) BuildV3Model() (*libopenapi.DocumentModel[v3high.Document], error) {
	return nil, d.err
}
func (d failingDocument) RenderAndReload() ([]byte, libopenapi.Document, *libopenapi.DocumentModel[v3high.Document], error) {
	return nil, nil, nil, nil
}
func (d failingDocument) Render() ([]byte, error)    { return nil, nil }
func (d failingDocument) Serialize() ([]byte, error) { return nil, nil }
func (d failingDocument) Release()                   {}

func TestBuildAPICommandWarnsOnModelFailure(t *testing.T) {
	c := New()
	var errOut strings.Builder
	c.Stderr = &errOut

	apiCmd := c.buildAPICommand("broken", &config.APIConfig{SpecURL: "https://api.example.com/openapi.json"}, &spec.APISpec{
		Document: failingDocument{err: errors.New("model boom")},
	})
	if apiCmd != nil {
		t.Fatal("expected nil API command when V3 model build fails")
	}
	if !strings.Contains(errOut.String(), "warning: skipping generated commands for API \"broken\"") {
		t.Fatalf("expected warning about skipped generated commands, got: %q", errOut.String())
	}
	if !strings.Contains(errOut.String(), "model boom") {
		t.Fatalf("expected underlying model error in warning, got: %q", errOut.String())
	}
}

func TestBuildOperationCommandRejectsInvalidTypedDefault(t *testing.T) {
	c := New()
	_, err := c.buildOperationCommand("myapi", "", spec.Operation{
		ID:     "search",
		Method: "GET",
		Path:   "/search",
		Parameters: []spec.Param{
			{Name: "enabled", In: "query", Type: "boolean", Default: "definitely", HasDefault: true},
		},
	})
	if err == nil {
		t.Fatal("expected invalid default error")
	}
	if !strings.Contains(err.Error(), "invalid boolean default") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildOperationCommandDisambiguatesOperatorFlagNames(t *testing.T) {
	c := New()
	cmd, err := c.buildOperationCommand("myapi", "", spec.Operation{
		ID:     "search",
		Method: "GET",
		Path:   "/search",
		Parameters: []spec.Param{
			{Name: "StartTime", In: "query", Type: "string"},
			{Name: "StartTime<", In: "query", Type: "string"},
			{Name: "StartTime<=", In: "query", Type: "string"},
			{Name: "StartTime>", In: "query", Type: "string"},
			{Name: "StartTime>=", In: "query", Type: "string"},
			{Name: "StartTime=", In: "query", Type: "string"},
			{Name: "StartTime!=", In: "query", Type: "string"},
		},
	})
	if err != nil {
		t.Fatalf("build operation command: %v", err)
	}
	for _, name := range []string{
		"start-time",
		"start-time-lt",
		"start-time-lte",
		"start-time-gt",
		"start-time-gte",
		"start-time-eq",
		"start-time-ne",
	} {
		if cmd.Flags().Lookup(name) == nil {
			t.Fatalf("expected generated flag --%s", name)
		}
	}
}

func TestBuildOperationCommandDisambiguatesNonOperatorFlagNames(t *testing.T) {
	c := New()
	var errOut strings.Builder
	c.Stderr = &errOut

	cmd, err := c.buildOperationCommand("myapi", "", spec.Operation{
		ID:     "search",
		Method: "GET",
		Path:   "/search",
		Parameters: []spec.Param{
			{Name: "start_time", In: "query", Type: "string"},
			{Name: "start-time", In: "query", Type: "string"},
		},
	})
	if err != nil {
		t.Fatalf("build operation command: %v", err)
	}
	if cmd.Flags().Lookup("start-time") == nil {
		t.Fatal("expected generated flag --start-time")
	}
	if cmd.Flags().Lookup("start-time-start-dash-time") == nil {
		t.Fatal("expected disambiguated generated flag --start-time-start-dash-time")
	}
	if !strings.Contains(errOut.String(), "parameter flag collision") {
		t.Fatalf("expected fallback warning, got: %q", errOut.String())
	}
}

func TestToKebabCaseKnownAcronyms(t *testing.T) {
	tests := map[string]string{
		"listAPIs":      "list-apis",
		"getURL":        "get-url",
		"parseJSONBody": "parse-json-body",
		"OAuthToken":    "oauth-token",
		"listItems":     "list-items",
	}
	for input, want := range tests {
		if got := toKebabCase(input); got != want {
			t.Fatalf("toKebabCase(%q) = %q, want %q", input, got, want)
		}
	}
}

// ---- Name-collision handling -----------------------------------------------

func buildSpecWithPaths(t *testing.T, yamlBody string) *spec.APISpec {
	t.Helper()
	loaders := spec.DefaultLoaders()
	s, err := loaders[0].LoadWithOptions([]byte(yamlBody), spec.LoadOptions{})
	if err != nil {
		t.Fatalf("build spec: %v", err)
	}
	return s
}

func TestBuildAPICommand_DuplicateOperationIDDisambiguated(t *testing.T) {
	// Two operations that kebab to the same name: one GET and one POST.
	specBody := `
openapi: "3.1.0"
info:
  title: DupTest
  version: "1.0.0"
paths:
  /items:
    get:
      operationId: listItems
      summary: List items
      responses:
        "200": {}
    post:
      operationId: listItems
      summary: Also list items (duplicate operationId)
      responses:
        "200": {}
`
	s := buildSpecWithPaths(t, specBody)
	c := New()
	var errOut strings.Builder
	c.Stderr = &errOut

	apiCmd := c.buildAPICommand("myapi", &config.APIConfig{BaseURL: "https://api.example.com"}, s)
	if apiCmd == nil {
		t.Fatal("expected non-nil API command")
	}

	// One of the two commands should have been disambiguated.
	names := make(map[string]bool)
	for _, sub := range apiCmd.Commands() {
		names[sub.Name()] = true
	}
	if len(names) != 2 {
		t.Errorf("expected 2 distinct commands, got %d: %v", len(names), names)
	}
	// A warning should have been emitted.
	if !strings.Contains(errOut.String(), "collision") {
		t.Errorf("expected collision warning, got: %q", errOut.String())
	}
}

func TestBuildAPICommand_AliasCollisionWarnsAndDropsAlias(t *testing.T) {
	specBody := `
openapi: "3.1.0"
info:
  title: AliasCollision
  version: "1.0.0"
paths:
  /items:
    get:
      operationId: listItems
      x-cli-aliases: [shared]
      responses:
        "200": {}
  /widgets:
    get:
      operationId: listWidgets
      x-cli-aliases: [shared]
      responses:
        "200": {}
`
	s := buildSpecWithPaths(t, specBody)
	c := New()
	var errOut strings.Builder
	c.Stderr = &errOut

	apiCmd := c.buildAPICommand("myapi", &config.APIConfig{BaseURL: "https://api.example.com"}, s)
	if apiCmd == nil {
		t.Fatal("expected non-nil API command")
	}
	sharedCount := 0
	for _, sub := range apiCmd.Commands() {
		for _, alias := range sub.Aliases {
			if alias == "shared" {
				sharedCount++
			}
		}
	}
	if sharedCount != 1 {
		t.Fatalf("shared alias count = %d, want 1", sharedCount)
	}
	if !strings.Contains(errOut.String(), "alias collision") {
		t.Errorf("expected alias collision warning, got: %q", errOut.String())
	}
}

func TestBuildAPICommand_MissingPathParamErrors(t *testing.T) {
	// Path references {petId} but no parameter is declared.
	specBody := `
openapi: "3.1.0"
info:
  title: MissingParam
  version: "1.0.0"
paths:
  /pets/{petId}:
    get:
      operationId: getPet
      summary: Get a pet
      responses:
        "200": {}
`
	s := buildSpecWithPaths(t, specBody)
	c := New()
	var errOut strings.Builder
	c.Stderr = &errOut

	apiCmd := c.buildAPICommand("myapi", &config.APIConfig{BaseURL: "https://api.example.com"}, s)
	if apiCmd == nil {
		t.Fatal("expected non-nil API command group (even with skipped ops)")
	}
	// The operation should have been skipped with a warning.
	if !strings.Contains(errOut.String(), "warning: skipping") {
		t.Errorf("expected skipping warning for missing path param, got: %q", errOut.String())
	}
}

func TestBuildAPICommand_DuplicatePathTemplateParamErrors(t *testing.T) {
	specBody := `
openapi: "3.1.0"
info:
  title: DuplicatePathParam
  version: "1.0.0"
paths:
  /pets/{id}/aliases/{id}:
    get:
      operationId: getPetAlias
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
      responses:
        "200": {}
`
	s := buildSpecWithPaths(t, specBody)
	c := New()
	var errOut strings.Builder
	c.Stderr = &errOut

	apiCmd := c.buildAPICommand("myapi", &config.APIConfig{BaseURL: "https://api.example.com"}, s)
	if apiCmd == nil {
		t.Fatal("expected non-nil API command group")
	}
	if !strings.Contains(errOut.String(), "repeats path parameter") {
		t.Errorf("expected duplicate path parameter warning, got: %q", errOut.String())
	}
}

func TestBuildAPICommand_PathParamXCLIIgnoreErrors(t *testing.T) {
	specBody := `
openapi: "3.1.0"
info:
  title: IgnoredPathParam
  version: "1.0.0"
paths:
  /pets/{id}:
    get:
      operationId: getPet
      parameters:
        - name: id
          in: path
          required: true
          x-cli-ignore: true
          schema:
            type: string
      responses:
        "200": {}
`
	s := buildSpecWithPaths(t, specBody)
	c := New()
	var errOut strings.Builder
	c.Stderr = &errOut

	apiCmd := c.buildAPICommand("myapi", &config.APIConfig{BaseURL: "https://api.example.com"}, s)
	if apiCmd == nil {
		t.Fatal("expected non-nil API command group")
	}
	if !strings.Contains(errOut.String(), "references missing path parameter") {
		t.Errorf("expected ignored path parameter warning, got: %q", errOut.String())
	}
}

func TestGeneratedQueryParamSerialization(t *testing.T) {
	t.Run("form arrays and reserved values", func(t *testing.T) {
		p := &paramInfo{name: "tag", in: "query", typ: "array", style: "form", explode: boolPtr(true), allowReserved: true}
		parts, err := serializeGeneratedQueryParam(p, []string{"red/blue", "green"})
		if err != nil {
			t.Fatalf("serialize: %v", err)
		}
		got := encodeGeneratedQuery(parts)
		if got != "tag=red/blue&tag=green" {
			t.Fatalf("query = %q, want repeated reserved values", got)
		}

		p.explode = boolPtr(false)
		parts, err = serializeGeneratedQueryParam(p, []string{"red", "blue"})
		if err != nil {
			t.Fatalf("serialize: %v", err)
		}
		got = encodeGeneratedQuery(parts)
		if got != "tag=red,blue" {
			t.Fatalf("query = %q, want comma-joined reserved array", got)
		}
	})

	t.Run("space pipe and deep object", func(t *testing.T) {
		space := &paramInfo{name: "ids", in: "query", typ: "array", style: "spaceDelimited"}
		parts, err := serializeGeneratedQueryParam(space, []string{"a", "b"})
		if err != nil {
			t.Fatalf("space serialize: %v", err)
		}
		if got := encodeGeneratedQuery(parts); got != "ids=a%20b" {
			t.Fatalf("space query = %q, want ids=a%%20b", got)
		}

		pipe := &paramInfo{name: "ids", in: "query", typ: "array", style: "pipeDelimited"}
		parts, err = serializeGeneratedQueryParam(pipe, []string{"a", "b"})
		if err != nil {
			t.Fatalf("pipe serialize: %v", err)
		}
		if got := encodeGeneratedQuery(parts); got != "ids=a%7Cb" {
			t.Fatalf("pipe query = %q, want ids=a%%7Cb", got)
		}

		obj := &paramInfo{name: "filter", in: "query", typ: "object", style: "deepObject"}
		parts, err = serializeGeneratedQueryParam(obj, []string{"limit:", "10,", "q:", "cats"})
		if err != nil {
			t.Fatalf("object serialize: %v", err)
		}
		got := encodeGeneratedQuery(parts)
		if got != "filter%5Blimit%5D=10&filter%5Bq%5D=cats" {
			t.Fatalf("deep object query = %q", got)
		}
	})
}

func TestEncodeGeneratedQueryValueReservedBytes(t *testing.T) {
	tests := []struct {
		name          string
		value         string
		allowReserved bool
		want          string
	}{
		{name: "literal plus stays encoded", value: "a+b", allowReserved: true, want: "a%2Bb"},
		{name: "space uses percent encoding", value: "a b", allowReserved: true, want: "a%20b"},
		{name: "encoded plus input preserves percent", value: "a%2Bb", allowReserved: true, want: "a%252Bb"},
		{name: "slash allowed", value: "a/b", allowReserved: true, want: "a/b"},
		{name: "comma allowed", value: "a,b", allowReserved: true, want: "a,b"},
		{name: "reserved set allowed", value: ":/?#[]@!$&'()*+,;=", allowReserved: true, want: ":/?#[]@!$&'()*%2B,;="},
		{name: "slash encoded without allow reserved", value: "a/b", allowReserved: false, want: "a%2Fb"},
		{name: "comma encoded without allow reserved", value: "a,b", allowReserved: false, want: "a%2Cb"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := encodeGeneratedQueryValue(tc.value, tc.allowReserved); got != tc.want {
				t.Fatalf("encodeGeneratedQueryValue(%q, %v) = %q, want %q", tc.value, tc.allowReserved, got, tc.want)
			}
		})
	}
}

func TestGeneratedPathHeaderCookieAndContentParamSerialization(t *testing.T) {
	matrix := &paramInfo{name: "id", in: "path", typ: "array", style: "matrix", explode: boolPtr(true)}
	gotPath, err := serializeGeneratedPathParam(matrix, []string{"a", "b"})
	if err != nil {
		t.Fatalf("matrix serialize: %v", err)
	}
	if gotPath != ";id=a;id=b" {
		t.Fatalf("matrix path = %q, want ;id=a;id=b", gotPath)
	}

	label := &paramInfo{name: "filter", in: "path", typ: "object", style: "label", explode: boolPtr(true)}
	gotPath, err = serializeGeneratedPathParam(label, []string{"limit:", "10,", "q:", "cats"})
	if err != nil {
		t.Fatalf("label serialize: %v", err)
	}
	if gotPath != ".limit=10.q=cats" {
		t.Fatalf("label path = %q, want .limit=10.q=cats", gotPath)
	}

	header := &paramInfo{name: "X-IDs", in: "header", typ: "array"}
	gotHeaders, err := serializeGeneratedHeaderParam(header, []string{"a", "b"})
	if err != nil {
		t.Fatalf("header serialize: %v", err)
	}
	if !reflect.DeepEqual(gotHeaders, []string{"a,b"}) {
		t.Fatalf("headers = %#v, want a,b", gotHeaders)
	}

	cookie := &paramInfo{name: "session", in: "cookie", typ: "array", style: "form", explode: boolPtr(true)}
	gotCookies, err := serializeGeneratedCookieParam(cookie, []string{"a", "b"})
	if err != nil {
		t.Fatalf("cookie serialize: %v", err)
	}
	if !reflect.DeepEqual(gotCookies, []string{"session=a", "session=b"}) {
		t.Fatalf("cookies = %#v", gotCookies)
	}

	content := &paramInfo{name: "filter", in: "query", contentMediaType: "application/json"}
	parts, err := serializeGeneratedQueryParam(content, []string{"limit:", "10,", "q:", "cats"})
	if err != nil {
		t.Fatalf("content serialize: %v", err)
	}
	if got := encodeGeneratedQuery(parts); got != "filter=%7B%22limit%22%3A10%2C%22q%22%3A%22cats%22%7D" {
		t.Fatalf("content query = %q", got)
	}
}

func TestAppendGeneratedParamSupportNote(t *testing.T) {
	got := appendGeneratedParamSupportNote("Filter value", spec.Param{In: "query", Style: "deepSpaceObject"})
	if !strings.Contains(got, `OpenAPI style "deepSpaceObject" is not fully supported`) {
		t.Fatalf("expected unsupported style note, got %q", got)
	}

	got = appendGeneratedParamSupportNote("", spec.Param{ContentMediaType: "text/plain"})
	if got != "parameter content type text/plain is sent as raw text" {
		t.Fatalf("content note = %q", got)
	}
}

func boolPtr(v bool) *bool {
	return &v
}
