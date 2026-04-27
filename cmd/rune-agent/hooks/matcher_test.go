// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.

package hooks

import "testing"

func TestMatchValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pattern string
		value   string
		want    bool
	}{
		{"empty matches all", "", "anything", true},
		{"star matches all", "*", "anything", true},

		{"exact single hit", "edit_file", "edit_file", true},
		{"exact single miss", "edit_file", "write_file", false},

		{"exact list hit first", "edit_file|write_file", "edit_file", true},
		{"exact list hit second", "edit_file|write_file", "write_file", true},
		{"exact list miss", "edit_file|write_file", "read_file", false},

		{"regex hit", "^Bash.*", "BashTool", true},
		{"regex miss", "^Bash.*", "Other", false},
		{"regex digits", `[0-9]+`, "abc123", true},

		{"invalid regex", "[abc", "value", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := matchValue(tc.pattern, tc.value); got != tc.want {
				t.Fatalf("matchValue(%q, %q) = %v, want %v", tc.pattern, tc.value, got, tc.want)
			}
		})
	}
}
