package cli_test

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/rest-sh/restish/v2/config"
)

// TestRootCompletionDescriptionsWithoutLoadingSpecs checks that `restish <TAB>`
// still describes an API with a cached spec as generated, and one without as
// generic, now that root completion no longer decodes the cached specs.
func TestRootCompletionDescriptionsWithoutLoadingSpecs(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	env := setupEnvWithSpec(t, mux, enumSpec)

	// Add an API with no cached spec next to the synced "tapi".
	var cfg config.Config
	data, err := os.ReadFile(env.cfgFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	cfg.APIs["uncached"] = &config.APIConfig{BaseURL: "https://uncached.example.com"}
	cfg.APIs["missingfile"] = &config.APIConfig{
		BaseURL:   "https://missing.example.com",
		SpecFiles: []string{t.TempDir() + "/does-not-exist.json"},
	}
	data, _ = json.Marshal(&cfg)
	if err := os.WriteFile(env.cfgFile, data, 0o600); err != nil {
		t.Fatal(err)
	}

	for _, word := range []string{"", "t", "u", "m"} {
		c, out := env.newCaptureCLI()
		if err := c.Run([]string{"restish", "__complete", word}); err != nil {
			t.Fatalf("__complete %q: %v", word, err)
		}
		got := out.String()
		if strings.HasPrefix("tapi", word) && !strings.Contains(got, "tapi\tCommands generated from the tapi API spec") {
			t.Errorf("__complete %q: want tapi as a generated API, got:\n%s", word, got)
		}
		if strings.HasPrefix("uncached", word) && !strings.Contains(got, "uncached\tGeneric requests using https://uncached.example.com") {
			t.Errorf("__complete %q: want uncached as a generic API, got:\n%s", word, got)
		}
		if strings.HasPrefix("missingfile", word) && !strings.Contains(got, "missingfile\tGeneric requests using https://missing.example.com") {
			t.Errorf("__complete %q: want missingfile (local spec file missing) as a generic API, got:\n%s", word, got)
		}
	}
}
