package procutil

import (
	"context"
	"os/exec"
	"runtime"
)

// ShellCommand returns a command that runs commandLine through the platform's
// standard command shell.
func ShellCommand(ctx context.Context, commandLine string) *exec.Cmd {
	name, args := ShellCommandArgs(runtime.GOOS, commandLine)
	return exec.CommandContext(ctx, name, args...)
}

// ShellCommandArgs returns the executable and arguments used by ShellCommand.
// goos is injectable so callers can unit-test platform construction without
// requiring that platform's shell to exist on the test host.
func ShellCommandArgs(goos, commandLine string) (string, []string) {
	if goos == "windows" {
		return "cmd", []string{"/c", commandLine}
	}
	return "/bin/sh", []string{"-c", commandLine}
}
