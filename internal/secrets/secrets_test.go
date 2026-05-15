package secrets

import "testing"

func TestHeaderNames(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"Authorization", true},
		{"authorization", true},
		{"Api-Key", true},
		{"Ocp-Apim-Subscription-Key", true},
		{"X-API-Key", true},
		{"X-Auth-Token", true},
		{"X-Secret", true},
		{"X-User-Token-Refresh-Hint", false},
		{"X-Trace-ID", false},
	}
	for _, tt := range tests {
		if got := IsHeaderName(tt.name); got != tt.want {
			t.Errorf("IsHeaderName(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestQueryParamNames(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"token", true},
		{"access_token", true},
		{"id_token", true},
		{"api_key", true},
		{"client_secret", true},
		{"subscription-key", true},
		{"token_type", false},
		{"key", false},
		{"user_token_refresh_hint", false},
	}
	for _, tt := range tests {
		if got := IsQueryParamName(tt.name); got != tt.want {
			t.Errorf("IsQueryParamName(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestSensitiveValueHeuristic(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{"key", "testing", false},
		{"key", "kh24g2j", true},
		{"key", "codex-fake-key", true},
		{"auth", "demo", false},
		{"auth", "Bearer abc123def", true},
		{"page", "kh24g2j", false},
	}
	for _, tt := range tests {
		if got := IsQueryParamValue(tt.name, tt.value); got != tt.want {
			t.Errorf("IsQueryParamValue(%q, %q) = %v, want %v", tt.name, tt.value, got, tt.want)
		}
	}
}

func TestJSONBodyKeys(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"Authorization", true},
		{"Cookie", true},
		{"client_secret", true},
		{"token_type", false},
		{"X-User-Token-Refresh-Hint", false},
	}
	for _, tt := range tests {
		if got := IsJSONBodyKey(tt.name); got != tt.want {
			t.Errorf("IsJSONBodyKey(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestOAuthErrorBodyKeys(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"access_token", true},
		{"refresh_token", true},
		{"client_secret", true},
		{"token_type", false},
		{"error_description", false},
	}
	for _, tt := range tests {
		if got := IsOAuthErrorBodyKey(tt.name); got != tt.want {
			t.Errorf("IsOAuthErrorBodyKey(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}
