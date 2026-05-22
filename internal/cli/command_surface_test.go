package cli_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestBuiltInCommandSurfaceMap(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "root",
			args: []string{"restish", "--help"},
			want: []string{
				"Generic HTTP Commands",
				"  get",
				"  post",
				"  put",
				"  patch",
				"  delete",
				"  head",
				"  options",
				"Configuration and Setup",
				"  api",
				"  cache",
				"  config",
				"  shell",
				"Plugin Commands",
				"  plugin",
				"Utilities",
				"  cert",
				"  doctor",
				"  edit",
				"  links",
				"  version",
			},
		},
		{
			name: "api",
			args: []string{"restish", "api", "--help"},
			want: []string{
				"auth",
				"connect",
				"inspect",
				"list",
				"remove",
				"set",
				"sync",
			},
		},
		{
			name: "api auth",
			args: []string{"restish", "api", "auth", "--help"},
			want: []string{
				"add",
				"inspect",
				"logout",
				"remove",
			},
		},
		{
			name: "cache",
			args: []string{"restish", "cache", "--help"},
			want: []string{
				"info",
				"clear",
			},
		},
		{
			name: "plugin",
			args: []string{"restish", "plugin", "--help"},
			want: []string{
				"debug",
				"install",
				"list",
				"remove",
			},
		},
		{
			name: "hidden completion alias",
			args: []string{"restish", "completion", "--help"},
			want: []string{
				"bash",
				"fish",
				"install",
				"powershell",
				"zsh",
			},
		},
		{
			name: "doctor",
			args: []string{"restish", "doctor", "--help"},
			want: []string{
				"api",
				"plugin",
			},
		},
		{
			name: "config",
			args: []string{"restish", "config", "--help"},
			want: []string{
				"edit",
				"path",
				"set",
				"show",
				"theme",
			},
		},
		{
			name: "config theme",
			args: []string{"restish", "config", "theme", "--help"},
			want: []string{
				"list",
				"reset",
				"set",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, out, _ := newTestCLI(t)
			if err := c.Run(tt.args); err != nil {
				t.Fatalf("%v: %v", tt.args, err)
			}
			got := out.String()
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Fatalf("%v help missing %q:\n%s", tt.args, want, got)
				}
			}
		})
	}
}

func TestHighTrafficCommandHelpIncludesExamples(t *testing.T) {
	commands := [][]string{
		{"restish", "--help"},
		{"restish", "get", "--help"},
		{"restish", "api", "connect", "--help"},
		{"restish", "api", "set", "--help"},
		{"restish", "api", "auth", "inspect", "--help"},
		{"restish", "plugin", "install", "--help"},
		{"restish", "config", "set", "--help"},
		{"restish", "doctor", "--help"},
		{"restish", "shell", "--help"},
		{"restish", "shell", "setup", "--help"},
		{"restish", "shell", "completion", "--help"},
		{"restish", "shell", "completion", "bash", "--help"},
		{"restish", "shell", "completion", "zsh", "--help"},
		{"restish", "shell", "completion", "fish", "--help"},
		{"restish", "shell", "completion", "powershell", "--help"},
		{"restish", "shell", "completion", "install", "--help"},
	}
	for _, args := range commands {
		c, out, _ := newTestCLI(t)
		if err := c.Run(args); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if !strings.Contains(out.String(), "Examples:") {
			t.Fatalf("%v: help missing examples:\n%s", args, out.String())
		}
	}
}

func TestRootHelpOmitsEmptyRegisteredAPIGroup(t *testing.T) {
	c, out, _ := newTestCLI(t)
	if err := c.Run([]string{"restish", "--help"}); err != nil {
		t.Fatalf("root help: %v", err)
	}
	if strings.Contains(out.String(), "Registered APIs") {
		t.Fatalf("root help should omit empty Registered APIs group:\n%s", out.String())
	}
}

func TestHelpAllDoesNotDuplicateGlobalFlagDefaults(t *testing.T) {
	c, out, _ := newTestCLI(t)
	if err := c.Run([]string{"restish", "get", "--help-all", "--help"}); err != nil {
		t.Fatalf("get --help-all: %v", err)
	}
	got := out.String()
	if strings.Contains(got, `(default: auto`) && strings.Contains(got, `(default "auto")`) {
		t.Fatalf("rsh-output-format help should not show duplicate defaults:\n%s", got)
	}
	if strings.Contains(got, "(default )") {
		t.Fatalf("help should not show empty defaults:\n%s", got)
	}
}

func TestRemovedPreReleaseCommandNames(t *testing.T) {
	for _, args := range [][]string{
		{"restish", "setup", "zsh"},
		{"restish", "theme", "set", "example/themes"},
		{"restish", "api", "show", "example"},
		{"restish", "api", "edit"},
		{"restish", "api", "clear-auth-cache", "example"},
		{"restish", "api", "auth", "clear-cache", "example"},
		{"restish", "api", "content-types"},
		{"restish", "content-types"},
	} {
		c, _, _ := newTestCLI(t)
		if err := c.Run(args); err == nil {
			t.Fatalf("%v should be removed before v2 release", args)
		}
	}
}

func TestUserFacingArgErrorsIncludeHelpNextStep(t *testing.T) {
	tests := [][]string{
		{"restish", "get"},
		{"restish", "api", "remove"},
		{"restish", "api", "connect", "demo"},
		{"restish", "api", "auth", "add", "demo"},
		{"restish", "config", "set"},
		{"restish", "shell", "setup"},
		{"restish", "plugin", "install"},
		{"restish", "cert"},
		{"restish", "edit"},
		{"restish", "links"},
	}
	for _, args := range tests {
		t.Run(strings.Join(args[1:], " "), func(t *testing.T) {
			c, _, _ := newTestCLI(t)
			err := c.Run(args)
			if err == nil {
				t.Fatalf("%v: expected usage error", args)
			}
			got := err.Error()
			for _, raw := range []string{"accepts ", "requires at least", "requires a minimum", "arg(s)"} {
				if strings.Contains(got, raw) {
					t.Fatalf("%v: raw Cobra arity leaked in %q", args, got)
				}
			}
			if !strings.Contains(got, "--help") {
				t.Fatalf("%v: expected help next step, got %q", args, got)
			}
		})
	}
}

func TestUnknownAPIErrorsSuggestListAndConnect(t *testing.T) {
	for _, args := range [][]string{
		{"restish", "api", "inspect", "missing"},
		{"restish", "api", "set", "missing", "base_url: https://api.example.com"},
		{"restish", "api", "auth", "header", "missing", "Authorization"},
	} {
		t.Run(strings.Join(args[1:], " "), func(t *testing.T) {
			c, _, _ := newTestCLI(t)
			err := c.Run(args)
			if err == nil {
				t.Fatalf("%v: expected unknown API error", args)
			}
			requireContains(t, err.Error(), `unknown API "missing"`, "restish api list", "restish api connect")
		})
	}
}

func TestCacheClearTargetErrorsAreActionable(t *testing.T) {
	c, _, _ := newTestCLI(t)
	err := c.Run([]string{"restish", "cache", "clear", "all"})
	if err == nil {
		t.Fatal("expected cache clear target miss")
	}
	requireContains(t, err.Error(), `unknown API or cached namespace "all"`, "restish cache info", "omit the argument")

	c, _, _ = newTestCLI(t)
	err = c.Run([]string{"restish", "cache", "clear", "--direct", "demo"})
	if err == nil {
		t.Fatal("expected cache clear --direct target error")
	}
	requireContains(t, err.Error(), "--direct cannot be used", "restish cache clear --direct", "restish cache clear demo")
}

func TestEmptyStatesSuggestNextCommand(t *testing.T) {
	t.Run("api list", func(t *testing.T) {
		c, out, _ := newTestCLI(t)
		if err := c.Run([]string{"restish", "api", "list"}); err != nil {
			t.Fatalf("api list: %v", err)
		}
		requireContains(t, out.String(), "No APIs configured.", "restish api connect <name> <url>")
	})

	t.Run("plugin list", func(t *testing.T) {
		t.Setenv("RSH_CONFIG_DIR", t.TempDir())
		c, out, _ := newTestCLI(t)
		if err := c.Run([]string{"restish", "plugin", "list"}); err != nil {
			t.Fatalf("plugin list: %v", err)
		}
		requireContains(t, out.String(), "No plugins found.", "restish plugin install <source>")
	})
}

func TestInvalidThemeSourceErrorIsActionable(t *testing.T) {
	c, _, _ := newTestCLI(t)
	err := c.Run([]string{"restish", "config", "theme", "set", "not-a-theme"})
	if err == nil {
		t.Fatal("expected invalid theme source error")
	}
	requireContains(t, err.Error(), `unknown theme source "not-a-theme"`, "official theme name", "local JSON/JSONC path", "GitHub user/repo")
}

func TestUnknownSubcommandsRejectConsistently(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "api",
			args: []string{"restish", "api", "wat"},
			want: `unknown api command "wat"`,
		},
		{
			name: "cache",
			args: []string{"restish", "cache", "wat"},
			want: `unknown cache command "wat"`,
		},
		{
			name: "config",
			args: []string{"restish", "config", "wat"},
			want: `unknown config command "wat"`,
		},
		{
			name: "plugin",
			args: []string{"restish", "plugin", "wat"},
			want: `unknown plugin command "wat"`,
		},
		{
			name: "api help",
			args: []string{"restish", "api", "does-not-exist", "--help"},
			want: `unknown api command "does-not-exist"`,
		},
		{
			name: "cache help",
			args: []string{"restish", "cache", "does-not-exist", "--help"},
			want: `unknown cache command "does-not-exist"`,
		},
		{
			name: "plugin help",
			args: []string{"restish", "plugin", "does-not-exist", "--help"},
			want: `unknown plugin command "does-not-exist"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _, _ := newTestCLI(t)
			err := c.Run(tt.args)
			if err == nil {
				t.Fatalf("%v: expected unknown subcommand error", tt.args)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("%v: expected error containing %q, got %v", tt.args, tt.want, err)
			}
		})
	}
}

func TestPlainUtilityCommandsRejectResponseTransformFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "version output format",
			args: []string{"restish", "version", "-o", "json"},
			want: "does not support -o/--rsh-output-format",
		},
		{
			name: "version filter",
			args: []string{"restish", "version", "-f", "body"},
			want: "does not support -f/--rsh-filter",
		},
		{
			name: "cert output format",
			args: []string{"restish", "cert", "https://example.com", "-o", "json"},
			want: "does not support -o/--rsh-output-format",
		},
		{
			name: "cert paging",
			args: []string{"restish", "cert", "https://example.com", "--rsh-max-pages", "1"},
			want: "does not support --rsh-max-pages",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _, _ := newTestCLI(t)
			err := c.Run(tt.args)
			if err == nil {
				t.Fatalf("%v: expected unsupported transform flag error", tt.args)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("%v: expected error containing %q, got %v", tt.args, tt.want, err)
			}
		})
	}
}

func TestWorkflowCommandHelpSurface(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "shell setup",
			args: []string{"restish", "shell", "setup", "--help"},
			want: []string{"<shell>", "--no-completion", "--dry-run", "--yes"},
		},
		{
			name: "edit",
			args: []string{"restish", "edit", "--help"},
			want: []string{"<uri> [patch ...]", "--edit-format", "--no-editor", "--dry-run", "--yes"},
		},
		{
			name: "api auth inspect",
			args: []string{"restish", "api", "auth", "inspect", "--help"},
			want: []string{"<api>", "--credential", "--operation", "--redact"},
		},
		{
			name: "links",
			args: []string{"restish", "links", "--help"},
			want: []string{"<uri> [rel...]", "hypermedia links"},
		},
		{
			name: "cert",
			args: []string{"restish", "cert", "--help"},
			want: []string{"<uri>", "--warn-days"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, out, _ := newTestCLI(t)
			if err := c.Run(tt.args); err != nil {
				t.Fatalf("%v: %v", tt.args, err)
			}
			got := out.String()
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Fatalf("%v help missing %q:\n%s", tt.args, want, got)
				}
			}
		})
	}
}

func TestWorkflowCommandHelpOmitsOldPrefixedLocalFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
		deny []string
	}{
		{
			name: "edit",
			args: []string{"restish", "edit", "--help"},
			deny: []string{"--rsh-yes"},
		},
		{
			name: "api auth inspect",
			args: []string{"restish", "api", "auth", "inspect", "--help"},
			deny: []string{"--rsh-credential", "--rsh-operation"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, out, _ := newTestCLI(t)
			if err := c.Run(tt.args); err != nil {
				t.Fatalf("%v: %v", tt.args, err)
			}
			got := out.String()
			for _, deny := range tt.deny {
				if strings.Contains(got, deny) {
					t.Fatalf("%v help should not include %q:\n%s", tt.args, deny, got)
				}
			}
		})
	}
}

func TestGeneratedAPICommandSurfaceMap(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/items", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	})
	mux.HandleFunc("/items/42", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"42"}`))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	env := setupGeneratedEnv(t, mux)

	c, out := env.newCaptureCLI()
	if err := c.Run([]string{"restish", "tapi", "--help"}); err != nil {
		t.Fatalf("tapi --help: %v", err)
	}
	apiHelp := out.String()
	for _, want := range []string{"list-items", "get-item", "create-item", "get-public"} {
		if !strings.Contains(apiHelp, want) {
			t.Fatalf("generated API help missing %q:\n%s", want, apiHelp)
		}
	}
	if strings.Contains(apiHelp, "get-secret") {
		t.Fatalf("generated API help should hide x-cli-hidden operations:\n%s", apiHelp)
	}

	c, out = env.newCaptureCLI()
	if err := c.Run([]string{"restish", "tapi", "get-item", "--help"}); err != nil {
		t.Fatalf("get-item --help: %v", err)
	}
	opHelp := out.String()
	for _, want := range []string{"get-item <id>", "--format", "--help-all", "The item ID", "Examples:", "restish tapi get-item <id>"} {
		if !strings.Contains(opHelp, want) {
			t.Fatalf("generated operation help missing %q:\n%s", want, opHelp)
		}
	}
	if strings.Contains(opHelp, "--rsh-header") {
		t.Fatalf("focused generated operation help should hide inherited request flags:\n%s", opHelp)
	}

	c, _ = env.newCaptureCLI()
	if err := c.Run([]string{"restish", "tapi", "get-itm", "--help"}); err == nil {
		t.Fatal("expected generated API unknown command help to fail")
	} else if !strings.Contains(err.Error(), `unknown command "get-itm" for "tapi"`) ||
		!strings.Contains(err.Error(), `did you mean "get-item"?`) {
		t.Fatalf("unexpected generated unknown command help error: %v", err)
	}
}
