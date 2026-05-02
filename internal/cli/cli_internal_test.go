package cli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/rest-sh/restish/v2/internal/config"
)

func TestExplicitConfigSidecarAndCachePaths(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "restish.json")
	c := &CLI{
		Paths:              config.NewPathsWithConfigFile(cfgPath),
		explicitConfigFile: true,
	}
	if got, want := c.tokenCachePath(), filepath.Join(filepath.Dir(cfgPath), "tokens.cbor"); got != want {
		t.Fatalf("tokenCachePath() = %q, want %q", got, want)
	}
	if got, want := c.externalToolApprovalsPath(), filepath.Join(filepath.Dir(cfgPath), "external-tool-approvals.json"); got != want {
		t.Fatalf("externalToolApprovalsPath() = %q, want %q", got, want)
	}
	if got := c.specCacheDir(); !strings.Contains(got, filepath.Join("specs", "configs")) {
		t.Fatalf("specCacheDir() = %q, want config-scoped specs dir", got)
	}
	if got := c.cacheDir(); !strings.Contains(got, "configs") {
		t.Fatalf("cacheDir() = %q, want config-scoped cache dir", got)
	}
}

func TestSaveExternalToolApprovalsConcurrentLeavesValidFile(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "restish.json")
	c := &CLI{hooks: testHooks{ConfigPath: cfgPath}}

	var wg sync.WaitGroup
	for _, approvals := range []map[string]bool{
		{"sha256:a": true},
		{"sha256:b": true},
		{"sha256:c": true},
	} {
		wg.Add(1)
		go func(approvals map[string]bool) {
			defer wg.Done()
			if err := c.saveExternalToolApprovals(approvals); err != nil {
				t.Errorf("saveExternalToolApprovals: %v", err)
			}
		}(approvals)
	}
	wg.Wait()

	data, err := os.ReadFile(c.externalToolApprovalsPath())
	if err != nil {
		t.Fatalf("read approvals: %v", err)
	}
	var stored externalToolApprovals
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatalf("approvals file is not valid JSON: %v\n%s", err, data)
	}
	if len(stored.Approved) != 1 {
		t.Fatalf("expected one complete approval set from final writer, got %#v", stored.Approved)
	}
	matches, err := filepath.Glob(c.externalToolApprovalsPath() + ".*.tmp")
	if err != nil {
		t.Fatalf("glob temp files: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected temp files to be cleaned up, got %v", matches)
	}
}

func TestIsSignalCancellationRequiresSignalCause(t *testing.T) {
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(errors.New("ordinary cancellation"))
	if isSignalCancellation(context.Canceled, ctx) {
		t.Fatal("ordinary context cancellation should not map to signal exit")
	}

	sigCtx, signalCancel := context.WithCancelCause(context.Background())
	signalCancel(signalCancelError{signal: os.Interrupt})
	if !isSignalCancellation(context.Canceled, sigCtx) {
		t.Fatal("signal cancellation should map to signal exit")
	}
}

func TestRootContextUsesSignalHandlingByDefault(t *testing.T) {
	c := New()
	called := false
	c.hooks.SignalAwareContext = func() (context.Context, context.CancelFunc) {
		called = true
		return context.WithCancel(context.Background())
	}

	_, cancel := c.rootContext()
	cancel()

	if !called {
		t.Fatal("default CLI should install signal-aware context")
	}
}

func TestSetSignalHandlingFalseUsesPlainContext(t *testing.T) {
	c := New()
	c.SetSignalHandling(false)
	c.hooks.SignalAwareContext = func() (context.Context, context.CancelFunc) {
		t.Fatal("signal-aware context should not be installed when signal handling is disabled")
		return context.WithCancel(context.Background())
	}

	ctx, cancel := c.rootContext()
	cancel()

	if isSignalCancellation(context.Canceled, ctx) {
		t.Fatal("plain cancellation should not map to signal exit")
	}
}

func TestGeneratedAPINames_FastPath(t *testing.T) {
	cfg := &config.Config{
		APIs: map[string]*config.APIConfig{
			"myapi":    {BaseURL: "https://api.example.com"},
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

	got = c.generatedAPINames([]string{"restish", "--rsh-insecure", "myapi", "op"}, cfg)
	if !reflect.DeepEqual(got, []string{"myapi"}) {
		t.Errorf("--bool flag: got %v, want [myapi]", got)
	}

	got = c.generatedAPINames([]string{"restish", "-vv", "myapi", "op"}, cfg)
	if !reflect.DeepEqual(got, []string{"myapi"}) {
		t.Errorf("count flag cluster: got %v, want [myapi]", got)
	}
}

func TestGeneratedAPINames_BuiltinVerb(t *testing.T) {
	cfg := &config.Config{
		APIs: map[string]*config.APIConfig{
			"myapi":    {BaseURL: "https://api.example.com"},
			"otherapi": {BaseURL: "https://other.example.com"},
		},
	}
	c := &CLI{}

	// Built-in verb first → no generated APIs are needed.
	got := c.generatedAPINames([]string{"restish", "get", "myapi/items"}, cfg)
	if len(got) != 0 {
		t.Errorf("builtin verb: got %v, want none", got)
	}
}

func TestGeneratedAPINames_UnknownArg(t *testing.T) {
	cfg := &config.Config{
		APIs: map[string]*config.APIConfig{
			"myapi": {BaseURL: "https://api.example.com"},
		},
	}
	c := &CLI{}

	// First positional is not a configured API → no generated API is needed.
	got := c.generatedAPINames([]string{"restish", "unknown"}, cfg)
	if len(got) != 0 {
		t.Errorf("unknown arg: got %v, want none", got)
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

func TestGeneratedAPINames_HelpAndCompletionLoadAll(t *testing.T) {
	cfg := &config.Config{
		APIs: map[string]*config.APIConfig{
			"aaa": {BaseURL: "https://a.example.com"},
			"bbb": {BaseURL: "https://b.example.com"},
		},
	}
	c := &CLI{}
	want := []string{"aaa", "bbb"}

	for _, args := range [][]string{
		{"restish", "--help"},
		{"restish", "help", "aaa"},
		{"restish", "__complete", ""},
	} {
		got := c.generatedAPINames(args, cfg)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%v: got %v, want %v", args, got, want)
		}
	}
}
