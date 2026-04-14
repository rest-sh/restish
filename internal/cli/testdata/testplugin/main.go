// testplugin is a minimal Restish plugin used in tests. It responds to
// --rsh-plugin-manifest by writing a CBOR-encoded manifest to stdout and
// exiting 0.  All other invocations exit 1.
package main

import (
	"os"

	"github.com/fxamacker/cbor/v2"
)

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "--rsh-plugin-manifest" {
			manifest := map[string]any{
				"name":                "testplugin",
				"version":             "1.0.0",
				"description":         "Test plugin for unit tests",
				"restish_api_version": 2,
				"hooks":               []string{"command"},
			}
			data, err := cbor.Marshal(manifest)
			if err != nil {
				os.Stderr.WriteString("marshal error: " + err.Error() + "\n")
				os.Exit(2)
			}
			os.Stdout.Write(data)
			os.Exit(0)
		}
	}
	os.Exit(1)
}
