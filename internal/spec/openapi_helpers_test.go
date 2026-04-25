package spec

import "testing"

func TestPathItemMethodsNilSafe(t *testing.T) {
	if got := PathItemMethods(nil); got != nil {
		t.Fatalf("PathItemMethods(nil) = %#v, want nil", got)
	}
}
