package cli

import (
	"strings"
	"testing"
)

func TestRedactDiagnosticAssignmentMultipleSensitiveValues(t *testing.T) {
	input := "access_token=one refresh_token:two client_secret=three password:four"
	got := redactDiagnosticSecretText(input)
	for _, secret := range []string{"one", "two", "three", "four"} {
		if strings.Contains(got, secret) {
			t.Fatalf("redacted text leaked %q: %q", secret, got)
		}
	}
	for _, marker := range []string{"access_token=***", "refresh_token:***", "client_secret=***", "password:***"} {
		if !strings.Contains(got, marker) {
			t.Fatalf("redacted text missing %q: %q", marker, got)
		}
	}
}
