package procutil

import (
	"reflect"
	"testing"
)

func TestShellCommandArgs(t *testing.T) {
	cases := []struct {
		name     string
		goos     string
		wantName string
		wantArgs []string
	}{
		{
			name:     "windows",
			goos:     "windows",
			wantName: "cmd",
			wantArgs: []string{"/c", "echo hi"},
		},
		{
			name:     "posix",
			goos:     "linux",
			wantName: "/bin/sh",
			wantArgs: []string{"-c", "echo hi"},
		},
		{
			name:     "darwin",
			goos:     "darwin",
			wantName: "/bin/sh",
			wantArgs: []string{"-c", "echo hi"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotName, gotArgs := ShellCommandArgs(tc.goos, "echo hi")
			if gotName != tc.wantName {
				t.Fatalf("name = %q, want %q", gotName, tc.wantName)
			}
			if !reflect.DeepEqual(gotArgs, tc.wantArgs) {
				t.Fatalf("args = %#v, want %#v", gotArgs, tc.wantArgs)
			}
		})
	}
}
