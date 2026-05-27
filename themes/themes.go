// Package themes exposes the official Restish theme files bundled with each
// release.
package themes

import (
	"embed"
	"fmt"
	"io/fs"
	"strings"
)

//go:embed *.json
var files embed.FS

// Names returns official theme names bundled with this Restish build.
func Names() []string {
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		names = append(names, strings.TrimSuffix(entry.Name(), ".json"))
	}
	return names
}

// Read returns the bundled JSON for an official theme name.
func Read(name string) ([]byte, error) {
	if name == "" || strings.ContainsAny(name, `/\`) {
		return nil, fmt.Errorf("theme %q is not bundled", name)
	}
	data, err := files.ReadFile(name + ".json")
	if err != nil {
		return nil, fmt.Errorf("theme %q is not bundled", name)
	}
	return data, nil
}
