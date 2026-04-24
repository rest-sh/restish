package output

import "testing"

func TestParseThemeJSONDirectMap(t *testing.T) {
	entries, err := ParseThemeJSON([]byte(`{"key":"#ffffff","status_2xx":"bold #00ff00","keyword":"#ff0000","function":"#00ffff","class":"#ff00ff","builtin":"#0000ff","operator":"#ffff00","markdown_heading":"#123456"}`))
	if err != nil {
		t.Fatalf("ParseThemeJSON: %v", err)
	}
	if entries["key"] != "#ffffff" {
		t.Fatalf("key entry = %q, want #ffffff", entries["key"])
	}
	for _, name := range []string{"keyword", "function", "class", "builtin", "operator"} {
		if entries[name] == "" {
			t.Fatalf("%s entry was not parsed", name)
		}
	}
	if entries["markdown_heading"] != "#123456" {
		t.Fatalf("markdown_heading entry = %q, want #123456", entries["markdown_heading"])
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
