package cli

import (
	"io"
	"os"

	"github.com/danielgtaylor/restish/v2/internal/config"
	"github.com/danielgtaylor/restish/v2/internal/content"
)

// Version is the current build version, set at build time via -ldflags.
var Version = "2.0.0-dev"

// CLI holds all state for a Restish instance. Using a struct instead of
// package-level globals makes it safe to instantiate multiple independent
// instances and trivially testable with in-memory I/O.
type CLI struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	// ConfigPath overrides the default config file location. Used in tests
	// to point at a temp file; leave empty to use the platform default.
	ConfigPath string

	// PassReader, if non-nil, is used as the source for secret prompts (e.g.
	// password input). Falls back to Stdin when nil. Set in tests to provide
	// a password without consuming the body stdin.
	PassReader io.Reader

	cfg     *config.Config
	content *content.Registry
}

// New returns a CLI wired to the real OS stdin/stdout/stderr.
func New() *CLI {
	return &CLI{
		Stdin:   os.Stdin,
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
		content: content.Default(),
	}
}

// AddContentType registers an additional content type with the CLI's registry.
func (c *CLI) AddContentType(ct *content.ContentType) {
	c.content.AddContentType(ct)
}

// AddEncoding registers an additional compression encoding with the CLI's registry.
func (c *CLI) AddEncoding(e *content.Encoding) {
	c.content.AddEncoding(e)
}

// Run executes the CLI with the provided arguments (pass os.Args from main).
func (c *CLI) Run(args []string) error {
	path := c.ConfigPath
	if path == "" {
		path = config.DefaultPath()
	}
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	c.cfg = cfg

	root := c.newRootCmd()
	root.SetArgs(args[1:])
	root.SetOut(c.Stdout)
	root.SetErr(c.Stderr)
	return root.Execute()
}
