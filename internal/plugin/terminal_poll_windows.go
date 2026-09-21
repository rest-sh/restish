//go:build windows

package plugin

import (
	"time"

	"golang.org/x/sys/windows"
)

func terminalInputReady(fd uintptr, timeout time.Duration) (bool, error) {
	result, err := windows.WaitForSingleObject(windows.Handle(fd), uint32(timeout.Milliseconds()))
	return result == windows.WAIT_OBJECT_0, err
}
