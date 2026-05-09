package output

import (
	"testing"

	"github.com/alecthomas/chroma/v2"
)

func TestParseThemeJSONDirectMap(t *testing.T) {
	entries, err := ParseThemeJSON([]byte(`{"key":"#ffffff","header_key":"#abcdef","status_2xx":"bold #00ff00","keyword":"#ff0000","function":"#00ffff","class":"#ff00ff","builtin":"#0000ff","operator":"#ffff00","markdown_heading":"#123456","diagnostic_warn":"bold #ffaa00"}`))
	if err != nil {
		t.Fatalf("ParseThemeJSON: %v", err)
	}
	if entries["key"] != "#ffffff" {
		t.Fatalf("key entry = %q, want #ffffff", entries["key"])
	}
	if entries["header_key"] != "#abcdef" {
		t.Fatalf("header_key entry = %q, want #abcdef", entries["header_key"])
	}
	for _, name := range []string{"keyword", "function", "class", "builtin", "operator"} {
		if entries[name] == "" {
			t.Fatalf("%s entry was not parsed", name)
		}
	}
	if entries["markdown_heading"] != "#123456" {
		t.Fatalf("markdown_heading entry = %q, want #123456", entries["markdown_heading"])
	}
	if entries["diagnostic_warn"] != "bold #ffaa00" {
		t.Fatalf("diagnostic_warn entry = %q, want bold #ffaa00", entries["diagnostic_warn"])
	}
}

func TestBuildThemeDefaultHeaderKeyCanDifferFromKey(t *testing.T) {
	style, err := BuildTheme(nil)
	if err != nil {
		t.Fatalf("BuildTheme: %v", err)
	}
	if got, want := style.Get(httpHeaderKey).Colour.String(), "#6fbfbf"; got != want {
		t.Fatalf("default header key color = %q, want %q", got, want)
	}
	if got, want := style.Get(chroma.NameTag).Colour.String(), "#5fafd7"; got != want {
		t.Fatalf("default key color = %q, want %q", got, want)
	}
}

func TestBuildThemeHeaderKeyFollowsUserKeyByDefault(t *testing.T) {
	style, err := BuildTheme(ThemeEntries{"key": "#ffffff"})
	if err != nil {
		t.Fatalf("BuildTheme: %v", err)
	}
	if got, want := style.Get(httpHeaderKey), style.Get(chroma.NameTag); got != want {
		t.Fatalf("header key style = %#v, want key style %#v", got, want)
	}
}

func TestBuildThemeHeaderKeyCanDifferFromKey(t *testing.T) {
	style, err := BuildTheme(ThemeEntries{"key": "#ffffff", "header_key": "#abcdef"})
	if err != nil {
		t.Fatalf("BuildTheme: %v", err)
	}
	if got, want := style.Get(chroma.NameTag).Colour.String(), "#ffffff"; got != want {
		t.Fatalf("key color = %q, want %q", got, want)
	}
	if got, want := style.Get(httpHeaderKey).Colour.String(), "#abcdef"; got != want {
		t.Fatalf("header key color = %q, want %q", got, want)
	}
}

func TestParseThemeJSONRejectsUnknownToken(t *testing.T) {
	if _, err := ParseThemeJSON([]byte(`{"not_a_token":"#ffffff"}`)); err == nil {
		t.Fatal("expected unknown token error")
	}
}

func TestParseThemeJSONRejectsWrappedTheme(t *testing.T) {
	if _, err := ParseThemeJSON([]byte(`{"theme":{"NameTag":"#ffffff"}}`)); err == nil {
		t.Fatal("expected wrapped theme object to be rejected")
	}
}
