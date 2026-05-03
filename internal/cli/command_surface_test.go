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
				"  content-types",
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
				"list",
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
			name: "completion",
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
				"migrate-v1",
				"plugin",
			},
		},
		{
			name: "flags",
			args: []string{"restish", "flags", "--help"},
			want: []string{
				"request",
				"output",
				"auth",
				"tls",
				"pagination",
				"cache",
				"general",
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

func TestFlagsCommandSurface(t *testing.T) {
	c, out, _ := newTestCLI(t)
	if err := c.Run([]string{"restish", "flags", "output"}); err != nil {
		t.Fatalf("flags output: %v", err)
	}
	got := out.String()
	for _, want := range []string{"--rsh-output-format", "--rsh-filter"} {
		if !strings.Contains(got, want) {
			t.Fatalf("flags output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "--rsh-query") {
		t.Fatalf("flags output should not include request flags:\n%s", got)
	}
}

func TestRemovedPreReleaseCommandNames(t *testing.T) {
	for _, args := range [][]string{
		{"restish", "setup", "zsh"},
		{"restish", "theme", "set", "example/themes"},
		{"restish", "api", "show", "example"},
		{"restish", "api", "edit"},
		{"restish", "api", "clear-auth-cache", "example"},
		{"restish", "api", "content-types"},
	} {
		c, _, _ := newTestCLI(t)
		if err := c.Run(args); err == nil {
			t.Fatalf("%v should be removed before v2 release", args)
		}
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
			want: []string{"<uri> [patch ...]", "--edit-format", "--rsh-interactive", "--no-editor", "--dry-run", "--rsh-yes"},
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
	for _, want := range []string{"get-item <id>", "--format", "--help-all", "The item ID"} {
		if !strings.Contains(opHelp, want) {
			t.Fatalf("generated operation help missing %q:\n%s", want, opHelp)
		}
	}
	if strings.Contains(opHelp, "--rsh-header") {
		t.Fatalf("focused generated operation help should hide inherited request flags:\n%s", opHelp)
	}
}
