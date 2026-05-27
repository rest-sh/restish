//go:build !windows

package procutil

import (
	"context"
	"os/exec"
	"syscall"
)

// ConfigureCommandTreeKill arranges for cmd and children in its process group
// to be killed when ctx is canceled.
func ConfigureCommandTreeKill(ctx context.Context, cmd *exec.Cmd) {
	_ = ctx
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
