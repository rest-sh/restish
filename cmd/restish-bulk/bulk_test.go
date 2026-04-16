package main

import "testing"

func TestBulkRelativePath(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		resolved string
		want     string
		wantErr  bool
	}{
		{
			name:     "valid child path",
			base:     "https://api.example.com/users/",
			resolved: "https://api.example.com/users/a/items/a1",
			want:     "a/items/a1.json",
		},
		{
			name:     "reject different host",
			base:     "https://api.example.com/users/",
			resolved: "https://attacker.example.com/users/a/items/a1",
			wantErr:  true,
		},
		{
			name:     "reject parent escape",
			base:     "https://api.example.com/users/a/",
			resolved: "https://api.example.com/users/secrets",
			wantErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := bulkRelativePath(tc.base, tc.resolved)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("bulkRelativePath: %v", err)
			}
			if got != tc.want {
				t.Fatalf("bulkRelativePath = %q, want %q", got, tc.want)
			}
		})
	}
}
