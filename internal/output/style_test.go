package output

import "testing"

func TestParseThemeJSONDirectMap(t *testing.T) {
	entries, err := ParseThemeJSON([]byte(`{"key":"#ffffff","status_2xx":"bold #00ff00"}`))
	if err != nil {
		t.Fatalf("ParseThemeJSON: %v", err)
	}
	if entries["key"] != "#ffffff" {
		t.Fatalf("key entry = %q, want #ffffff", entries["key"])
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
