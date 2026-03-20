package cli

import (
	"testing"
)

func TestSplitTrimmed(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"empty string", "", nil},
		{"whitespace only", "   ", nil},
		{"single token", "create", []string{"create"}},
		{"two tokens", "create,write", []string{"create", "write"}},
		{"tokens with spaces", "create, write, remove", []string{"create", "write", "remove"}},
		{"trailing comma", "create,", []string{"create"}},
		{"comma only", ",", nil},
		{"spaces between commas", " , ", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := splitTrimmed(tc.input, ",")
			if len(got) != len(tc.want) {
				t.Fatalf("splitTrimmed(%q) = %v (len %d), want %v (len %d)",
					tc.input, got, len(got), tc.want, len(tc.want))
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("splitTrimmed(%q)[%d] = %q, want %q", tc.input, i, got[i], tc.want[i])
				}
			}
		})
	}
}
