//go:build !windows

package plugin

import (
	"errors"
	"time"

	"golang.org/x/sys/unix"
)

func terminalInputReady(fd uintptr, timeout time.Duration) (bool, error) {
	fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
	for {
		n, err := unix.Poll(fds, int(timeout.Milliseconds()))
		if errors.Is(err, unix.EINTR) {
			continue
		}
		return n > 0, err
	}
}
