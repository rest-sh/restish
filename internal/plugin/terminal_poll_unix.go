//go:build !windows

package plugin

import (
	"time"

	"golang.org/x/sys/unix"
)

func terminalInputReady(fd uintptr, timeout time.Duration) (bool, error) {
	fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
	n, err := unix.Poll(fds, int(timeout.Milliseconds()))
	return n > 0, err
}
