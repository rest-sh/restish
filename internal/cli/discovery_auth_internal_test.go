package cli

import (
	"testing"

	"github.com/rest-sh/restish/v2/internal/config"
)

func TestDiscoveryAuthAllowedNormalizesRequestURL(t *testing.T) {
	origins := discoveryAuthOrigins(&config.APIConfig{BaseURL: "api.example.com"}, "default")
	if !discoveryAuthAllowed(origins, "api.example.com/openapi.json") {
		t.Fatal("expected shorthand request URL to match normalized base URL origin")
	}
}

func TestDiscoveryAuthOriginsIgnoreLocalSpecFiles(t *testing.T) {
	origins := discoveryAuthOrigins(&config.APIConfig{
		BaseURL: "https://base.example.com",
		SpecFiles: []string{
			"api.example.com/openapi.yaml",
			"file:///tmp/openapi.yaml",
			"https://spec.example.com/openapi.yaml",
		},
	}, "default")

	if discoveryAuthAllowed(origins, "https://api.example.com/schema.json") {
		t.Fatal("local-looking spec file path should not authorize matching HTTP origin")
	}
	if !discoveryAuthAllowed(origins, "https://spec.example.com/schema.json") {
		t.Fatal("explicit HTTP spec file URL should authorize matching origin")
	}
}
