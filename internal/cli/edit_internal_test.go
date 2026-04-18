package cli

import (
	"reflect"
	"testing"
)

func TestSplitCommandLinePreservesEmptyQuotedArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want []string
	}{
		{
			name: "double quotes",
			in:   `emacsclient -a "" -c`,
			want: []string{"emacsclient", "-a", "", "-c"},
		},
		{
			name: "single quotes",
			in:   `cmd '' tail`,
			want: []string{"cmd", "", "tail"},
		},
		{
			name: "mixed quoted and unquoted",
			in:   `cmd prefix""suffix ""`,
			want: []string{"cmd", "prefixsuffix", ""},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := splitCommandLine(tt.in)
			if err != nil {
				t.Fatalf("splitCommandLine(%q): %v", tt.in, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("splitCommandLine(%q) = %#v, want %#v", tt.in, got, tt.want)
			}
		})
	}
}
