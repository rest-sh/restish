package cli

import (
	"reflect"
	"testing"

	"github.com/danielgtaylor/restish/v2/internal/config"
)

func TestGeneratedAPINames_FastPath(t *testing.T) {
	cfg := &config.Config{
		APIs: map[string]*config.APIConfig{
			"myapi":   {BaseURL: "https://api.example.com"},
			"otherapi": {BaseURL: "https://other.example.com"},
		},
	}
	c := &CLI{}

	// Plain API name as first arg → only that API is loaded.
	got := c.generatedAPINames([]string{"restish", "myapi", "list-items"}, cfg)
	if !reflect.DeepEqual(got, []string{"myapi"}) {
		t.Errorf("got %v, want [myapi]", got)
	}
}

func TestGeneratedAPINames_SkipsLeadingFlags(t *testing.T) {
	cfg := &config.Config{
		APIs: map[string]*config.APIConfig{
			"myapi": {BaseURL: "https://api.example.com"},
		},
	}
	c := &CLI{}

	// Boolean short flag before API name: -v myapi op
	got := c.generatedAPINames([]string{"restish", "-v", "myapi", "op"}, cfg)
	if !reflect.DeepEqual(got, []string{"myapi"}) {
		t.Errorf("-v flag: got %v, want [myapi]", got)
	}

	// Value flag with = form: --rsh-profile=default myapi op
	got = c.generatedAPINames([]string{"restish", "--rsh-profile=default", "myapi", "op"}, cfg)
	if !reflect.DeepEqual(got, []string{"myapi"}) {
		t.Errorf("--flag=value: got %v, want [myapi]", got)
	}

	// Value flag with separate value: --rsh-profile default myapi op
	got = c.generatedAPINames([]string{"restish", "--rsh-profile", "default", "myapi", "op"}, cfg)
	if !reflect.DeepEqual(got, []string{"myapi"}) {
		t.Errorf("--flag value: got %v, want [myapi]", got)
	}
}

func TestGeneratedAPINames_BuiltinVerb(t *testing.T) {
	cfg := &config.Config{
		APIs: map[string]*config.APIConfig{
			"myapi":   {BaseURL: "https://api.example.com"},
			"otherapi": {BaseURL: "https://other.example.com"},
		},
	}
	c := &CLI{}

	// Built-in verb first → load all APIs.
	got := c.generatedAPINames([]string{"restish", "get", "myapi/items"}, cfg)
	want := []string{"myapi", "otherapi"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("builtin verb: got %v, want %v", got, want)
	}
}

func TestGeneratedAPINames_UnknownArg(t *testing.T) {
	cfg := &config.Config{
		APIs: map[string]*config.APIConfig{
			"myapi": {BaseURL: "https://api.example.com"},
		},
	}
	c := &CLI{}

	// First positional is not a configured API → load all.
	got := c.generatedAPINames([]string{"restish", "unknown"}, cfg)
	if !reflect.DeepEqual(got, []string{"myapi"}) {
		t.Errorf("unknown arg: got %v, want [myapi]", got)
	}
}

func TestGeneratedAPINames_NoArgs(t *testing.T) {
	cfg := &config.Config{
		APIs: map[string]*config.APIConfig{
			"aaa": {BaseURL: "https://a.example.com"},
			"bbb": {BaseURL: "https://b.example.com"},
		},
	}
	c := &CLI{}

	// No args → load all, sorted.
	got := c.generatedAPINames([]string{"restish"}, cfg)
	want := []string{"aaa", "bbb"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("no args: got %v, want %v", got, want)
	}
}
