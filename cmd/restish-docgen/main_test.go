package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratedReferenceDocsDoNotContainMissingCommandMarker(t *testing.T) {
	root, err := repoRoot("")
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range targets {
		for _, region := range target.regions {
			switch region {
			case "api-command", "cache-command", "config-command", "doctor-command", "edit-command", "http-commands", "plugin-command", "shell-command", "utility-commands":
			default:
				continue
			}
			path := filepath.Join(root, filepath.FromSlash(target.path))
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(data), "Command not found in the current binary.") {
				t.Fatalf("%s contains stale command reference output", target.path)
			}
		}
	}
}
