package cli

// Hooks provides test-only access to CLI internals. Call this in test code to
// configure overrides that are not part of the public API.
func (c *CLI) Hooks() *testHooks {
	return &c.hooks
}
