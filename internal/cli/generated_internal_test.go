package cli

import (
	"errors"
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

// ---- Name-collision handling -----------------------------------------------

func buildSpecWithPaths(t *testing.T, yamlBody string) *spec.APISpec {
	t.Helper()
	loaders := spec.DefaultLoaders()
	s, err := loaders[0].Load([]byte(yamlBody))
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
