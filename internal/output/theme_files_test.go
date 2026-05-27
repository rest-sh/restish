package output

import (
	"os"
	"path/filepath"
	"testing"
)

func TestThemeFilesParse(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "themes", "*.json"))
	if err != nil {
		t.Fatalf("glob theme files: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("expected at least one theme file")
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("read theme file: %v", err)
			}
			if _, err := ParseThemeJSON(data); err != nil {
				t.Fatalf("parse theme file: %v", err)
			}
		})
	}
}
