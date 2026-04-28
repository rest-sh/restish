package cli_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rest-sh/restish/v2/internal/cli"
)

const benchmarkLargeOpenAPIOperations = 1000

func BenchmarkStartupCachedLargeOpenAPIHelp(b *testing.B) {
	env := setupLargeOpenAPIBenchmarkEnv(b, benchmarkLargeOpenAPIOperations)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		c := newLargeOpenAPIBenchmarkCLI(env, nil)
		if err := c.Run([]string{"restish", "tapi", "--help"}); err != nil {
			b.Fatalf("tapi --help: %v", err)
		}
	}
}

func BenchmarkStartupCachedLargeOpenAPICommand(b *testing.B) {
	env := setupLargeOpenAPIBenchmarkEnv(b, benchmarkLargeOpenAPIOperations)
	transport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
			Request:    req,
		}, nil
	})
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		c := newLargeOpenAPIBenchmarkCLI(env, transport)
		if err := c.Run([]string{"restish", "--rsh-silent", "tapi", "get-resource999"}); err != nil {
			b.Fatalf("generated command: %v", err)
		}
	}
}

func BenchmarkSyncLargeOpenAPISpec(b *testing.B) {
	env := setupLargeOpenAPIBenchmarkEnvWithoutSync(b, benchmarkLargeOpenAPIOperations)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cacheDir := filepath.Join(env.cacheDir, fmt.Sprintf("iter-%06d", i))
		c := newLargeOpenAPIBenchmarkCLI(largeOpenAPIBenchmarkEnv{
			cfgFile:  env.cfgFile,
			cacheDir: cacheDir,
		}, nil)
		if err := c.Run([]string{"restish", "api", "sync", "tapi"}); err != nil {
			b.Fatalf("api sync: %v", err)
		}
	}
}

type largeOpenAPIBenchmarkEnv struct {
	cfgFile  string
	cacheDir string
}

func setupLargeOpenAPIBenchmarkEnv(b *testing.B, operations int) largeOpenAPIBenchmarkEnv {
	b.Helper()
	env := setupLargeOpenAPIBenchmarkEnvWithoutSync(b, operations)
	c := newLargeOpenAPIBenchmarkCLI(env, nil)
	if err := c.Run([]string{"restish", "api", "sync", "tapi"}); err != nil {
		b.Fatalf("api sync: %v", err)
	}
	return env
}

func setupLargeOpenAPIBenchmarkEnvWithoutSync(b *testing.B, operations int) largeOpenAPIBenchmarkEnv {
	b.Helper()
	specBody := largeOpenAPIBenchmarkSpec("https://api.example.com", operations)
	specServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, specBody)
	}))
	b.Cleanup(specServer.Close)

	dir := b.TempDir()
	b.Setenv("RSH_CONFIG_DIR", filepath.Join(dir, "config-dir"))
	cfgFile := filepath.Join(dir, "restish.json")
	cfgData := fmt.Sprintf(`{
  "apis": {
    "tapi": {
      "base_url": "https://api.example.com",
      "spec_url": %q
    }
  }
}`, specServer.URL)
	if err := os.WriteFile(cfgFile, []byte(cfgData), 0o600); err != nil {
		b.Fatalf("write config: %v", err)
	}

	return largeOpenAPIBenchmarkEnv{
		cfgFile:  cfgFile,
		cacheDir: filepath.Join(dir, "spec-cache"),
	}
}

func newLargeOpenAPIBenchmarkCLI(env largeOpenAPIBenchmarkEnv, transport http.RoundTripper) *cli.CLI {
	c := cli.New()
	c.Stdin = strings.NewReader("")
	c.Stdout = io.Discard
	c.Stderr = io.Discard
	c.Hooks().ConfigPath = env.cfgFile
	c.Hooks().SpecCachePath = env.cacheDir
	c.Hooks().PluginManifestCachePath = filepath.Join(filepath.Dir(env.cfgFile), "plugin-manifest.cbor")
	c.Hooks().RetryBaseDelay = 0
	if transport != nil {
		c.Hooks().HTTPTransport = transport
	}
	return c
}

func largeOpenAPIBenchmarkSpec(baseURL string, operations int) string {
	var paths strings.Builder
	for i := 0; i < operations; i++ {
		if i > 0 {
			paths.WriteString(",")
		}
		fmt.Fprintf(&paths, `"/resources/%03d": {
      "get": {
        "operationId": "getResource%03d",
        "summary": "Get resource %03d",
        "tags": ["resources"],
        "parameters": [],
        "responses": {
          "200": {
            "description": "OK",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "id": {"type": "string"},
                    "name": {"type": "string"},
                    "created_at": {"type": "string", "format": "date-time"}
                  }
                }
              }
            }
          }
        }
      }
    }`, i, i, i)
	}
	return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {
    "title": "Large Benchmark API",
    "version": "1.0",
    "description": "Synthetic public-API shaped benchmark spec."
  },
  "servers": [{"url": %q}],
  "paths": {%s}
}`, baseURL, paths.String())
}
