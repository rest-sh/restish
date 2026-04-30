//go:build windows

package procutil

import (
	"context"
	"os/exec"
)

// ConfigureCommandTreeKill is a placeholder for Windows process-tree cleanup.
// exec.CommandContext still terminates the immediate process on cancellation.
func ConfigureCommandTreeKill(context.Context, *exec.Cmd) {}
