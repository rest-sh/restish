package cli

import (
	"net/http"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExternalToolAuthStderrOnError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses /bin/sh")
	}

	auth := &ExternalToolAuth{}
	req, _ := http.NewRequest("GET", "https://example.com", nil)

	err := auth.OnRequest(req, "test", map[string]string{
		"commandline": "echo 'token expired, please re-authenticate' >&2; exit 1",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "external auth tool failed")
	assert.Contains(t, err.Error(), "token expired, please re-authenticate")
}

func TestExternalToolAuthStderrEmptyOnError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses /bin/sh")
	}

	auth := &ExternalToolAuth{}
	req, _ := http.NewRequest("GET", "https://example.com", nil)

	err := auth.OnRequest(req, "test", map[string]string{
		"commandline": "exit 1",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "external auth tool failed")
}
