package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/spf13/cobra"
)

func TestHandleCommandPluginMessageRejectsMalformedDone(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cli := &CLI{Stdout: &stdout, Stderr: &stderr}
	cmd := &cobra.Command{Use: "test"}
	cmd.SetErr(&stderr)

	raw, err := cbor.Marshal(map[string]any{
		"type":      "done",
		"exit_code": "boom",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	done, gotErr := cli.handleCommandPluginMessage(cmd, nil, "done", raw)
	if !done {
		t.Fatal("expected malformed done message to stop processing")
	}
	if gotErr == nil {
		t.Fatal("expected malformed done message to return an error")
	}
	if !strings.Contains(gotErr.Error(), "decode done") {
		t.Fatalf("expected decode error, got: %v", gotErr)
	}
}
