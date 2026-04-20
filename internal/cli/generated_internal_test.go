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

	"github.com/danielgtaylor/restish/v2/internal/config"
	"github.com/danielgtaylor/restish/v2/internal/spec"
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
