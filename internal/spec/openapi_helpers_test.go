package spec

import (
	"testing"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

func TestPathItemMethodsNilSafe(t *testing.T) {
	if got := PathItemMethods(nil); got != nil {
		t.Fatalf("PathItemMethods(nil) = %#v, want nil", got)
	}
}

func TestPathItemMethodsIncludesTrace(t *testing.T) {
	trace := &v3.Operation{OperationId: "traceDiagnostics"}
	got := PathItemMethods(&v3.PathItem{Trace: trace})
	for _, method := range got {
		if method.Method == "TRACE" && method.Op == trace {
			return
		}
	}
	t.Fatalf("PathItemMethods did not include TRACE operation: %#v", got)
}
