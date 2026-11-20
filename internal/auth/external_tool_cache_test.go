package auth

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rest-sh/restish/v2/auth"
)

// countingTool returns a command line that counts its runs in a file and adds the header
// X-Token: t<run number>. Like many credential helpers, it also echoes the request headers
// it was given (Accept here), unchanged.
func countingTool(t *testing.T) (commandline string, runs func() int) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell command test not supported on Windows")
	}
	counter := filepath.Join(t.TempDir(), "runs")
	commandline = `n=$(cat '` + counter + `' 2>/dev/null || echo 0); n=$((n+1)); echo $n > '` + counter + `'; ` +
		`printf '{"headers":{"Accept":["application/json"],"X-Token":["t%s"]}}' "$n"`
	return commandline, func() int {
		b, err := os.ReadFile(counter)
		if err != nil {
			return 0
		}
		n := 0
		for _, c := range strings.TrimSpace(string(b)) {
			n = n*10 + int(c-'0')
		}
		return n
	}
}

func cacheContext(t *testing.T, store auth.TokenStore, params map[string]string) auth.AuthContext {
	return auth.AuthContext{APIName: "example", ProfileName: "default", CacheKey: "example:default", Params: params, TokenStore: store}
}

func authenticate(t *testing.T, ac auth.AuthContext, uri, accept string) *http.Request {
	t.Helper()
	req, _ := http.NewRequest("GET", uri, nil)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	if err := (&ExternalTool{}).Authenticate(context.Background(), req, ac); err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	return req
}

func TestExternalTool_CacheTTL_ReusesTheHeadersTheToolAdded(t *testing.T) {
	cmd, runs := countingTool(t)
	store := auth.NewTokenCache(filepath.Join(t.TempDir(), "tokens.cbor"))
	ac := cacheContext(t, store, map[string]string{"commandline": cmd, "cachettl": "10m"})

	first := authenticate(t, ac, "https://api.example.com/items", "application/json")
	second := authenticate(t, ac, "https://api.example.com/other?x=1", "text/csv")
	if runs() != 1 {
		t.Fatalf("the tool ran %d times for two requests within the TTL, want 1", runs())
	}
	if first.Header.Get("X-Token") != "t1" || second.Header.Get("X-Token") != "t1" {
		t.Errorf("X-Token: %q then %q, want t1 twice", first.Header.Get("X-Token"), second.Header.Get("X-Token"))
	}
	// a header the tool only echoed is the request's own: it is not cached
	if second.Header.Get("Accept") != "text/csv" {
		t.Errorf("Accept of the second request: %q, want its own text/csv", second.Header.Get("Accept"))
	}

	// another process (another restish run) reads the same cache file
	other := cacheContext(t, auth.NewTokenCache(store.Path()), ac.Params)
	if authenticate(t, other, "https://api.example.com/items", "").Header.Get("X-Token") != "t1" || runs() != 1 {
		t.Errorf("the cache is not shared through the token cache file (runs=%d)", runs())
	}
}

func TestExternalTool_CacheTTL_ForceRunsTheToolAgain(t *testing.T) {
	cmd, runs := countingTool(t)
	ac := cacheContext(t, auth.NewTokenCache(filepath.Join(t.TempDir(), "tokens.cbor")), map[string]string{"commandline": cmd, "cachettl": "10m"})
	authenticate(t, ac, "https://api.example.com/items", "")
	forced := ac
	forced.Force = true // restish retries with Force after a 401: the cached headers were rejected
	if got := authenticate(t, forced, "https://api.example.com/items", "").Header.Get("X-Token"); got != "t2" || runs() != 2 {
		t.Fatalf("after Force: X-Token %q, runs %d; want t2, 2", got, runs())
	}
	if got := authenticate(t, ac, "https://api.example.com/items", "").Header.Get("X-Token"); got != "t2" || runs() != 2 {
		t.Errorf("Force must refresh the cache: X-Token %q, runs %d; want t2, 2", got, runs())
	}
}

func TestExternalTool_CacheTTL_Expires(t *testing.T) {
	cmd, runs := countingTool(t)
	ac := cacheContext(t, auth.NewTokenCache(filepath.Join(t.TempDir(), "tokens.cbor")), map[string]string{"commandline": cmd, "cachettl": "50ms"})
	authenticate(t, ac, "https://api.example.com/items", "")
	time.Sleep(80 * time.Millisecond)
	if got := authenticate(t, ac, "https://api.example.com/items", "").Header.Get("X-Token"); got != "t2" || runs() != 2 {
		t.Errorf("after the TTL: X-Token %q, runs %d; want t2, 2", got, runs())
	}
}

func TestExternalTool_CacheTTL_IsPerHost(t *testing.T) {
	cmd, runs := countingTool(t)
	ac := cacheContext(t, auth.NewTokenCache(filepath.Join(t.TempDir(), "tokens.cbor")), map[string]string{"commandline": cmd, "cachettl": "10m"})
	authenticate(t, ac, "https://api.example.com/items", "")
	// a tool may give other credentials for another host
	if got := authenticate(t, ac, "https://api.example.us/items", "").Header.Get("X-Token"); got != "t2" || runs() != 2 {
		t.Errorf("another host: X-Token %q, runs %d; want t2, 2", got, runs())
	}
}

func TestExternalTool_CacheTTL_NotWhenTheToolRewritesTheURI(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell command test not supported on Windows")
	}
	counter := filepath.Join(t.TempDir(), "runs")
	cmd := `echo x >> '` + counter + `'; echo '{"uri":"https://proxy.example.com/items","headers":{"X-Token":["t"]}}'`
	ac := cacheContext(t, auth.NewTokenCache(filepath.Join(t.TempDir(), "tokens.cbor")), map[string]string{"commandline": cmd, "cachettl": "10m"})
	authenticate(t, ac, "https://api.example.com/items", "")
	req := authenticate(t, ac, "https://api.example.com/items", "")
	b, _ := os.ReadFile(counter)
	if strings.Count(string(b), "x") != 2 || req.URL.Host != "proxy.example.com" {
		t.Errorf("a URI rewrite depends on the request: the tool must run each time (runs %d, host %s)", strings.Count(string(b), "x"), req.URL.Host)
	}
}

func TestExternalTool_NoCacheByDefault(t *testing.T) {
	cmd, runs := countingTool(t)
	ac := cacheContext(t, auth.NewTokenCache(filepath.Join(t.TempDir(), "tokens.cbor")), map[string]string{"commandline": cmd})
	authenticate(t, ac, "https://api.example.com/items", "")
	authenticate(t, ac, "https://api.example.com/items", "")
	if runs() != 2 {
		t.Errorf("without cachettl the tool runs for every request: %d runs, want 2", runs())
	}
}

func TestExternalTool_CacheTTL_Invalid(t *testing.T) {
	for _, ttl := range []string{"soon", "-1m"} {
		req, _ := http.NewRequest("GET", "https://api.example.com", nil)
		err := (&ExternalTool{}).Authenticate(context.Background(), req, auth.AuthContext{
			Params: map[string]string{"commandline": "true", "cachettl": ttl},
		})
		if err == nil || !strings.Contains(err.Error(), "cachettl") {
			t.Errorf("cachettl %q: error %v, want one naming cachettl", ttl, err)
		}
	}
}
