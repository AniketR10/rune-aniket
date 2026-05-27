// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.

package cmdenv

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpand(t *testing.T) {
	src := Source(func(name string) (string, bool) {
		switch name {
		case "WORKSPACE":
			return "blue", true
		case "WORKSPACE_URI":
			return "file:///tmp/blue", true
		case "FILE":
			return "/tmp/a b.go", true
		case "EMPTY":
			return "", true
		}
		return "", false
	})
	t.Setenv("RUNE_DATADIR", "/data")
	t.Setenv("HOME", "/home/u")

	cases := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"plain", "literal", "literal", false},
		{"simple var", "$WORKSPACE", "blue", false},
		{"braced var", "${WORKSPACE_URI}", "file:///tmp/blue", false},
		{"compose", "$WORKSPACE-$FILE",
			"blue-/tmp/a b.go", false},
		{"env fallback", "$RUNE_DATADIR/x", "/data/x", false},
		{"source wins over os", "$HOME-$WORKSPACE", "/home/u-blue",
			false},
		{"empty var present is empty", "x${EMPTY}y", "xy", false},
		{"missing var expands to empty",
			"x$UNDEFINED_VAR_NAMED_FOR_TEST_XYZ.y", "x.y", false},
		{"value with spaces is one field",
			"$FILE", "/tmp/a b.go", false},
		{"arithmetic", "$((1+2))", "3", false},
		{"escaped dollar", `\$WORKSPACE`, "$WORKSPACE", false},
		{"reject cmdsubst", "$(echo hi)", "", true},
		{"reject backtick", "`echo hi`", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Expand(context.Background(), tc.in, src)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestExpandNilSourceUsesOSEnv(t *testing.T) {
	t.Setenv("RUNE_TEST_NIL_ENV", "yes")
	got, err := Expand(context.Background(), "$RUNE_TEST_NIL_ENV", nil)
	require.NoError(t, err)
	assert.Equal(t, "yes", got)
}

func TestLookupNilReturnsOSGetenv(t *testing.T) {
	t.Setenv("RUNE_TEST_LOOKUP", "ok")
	fn := Lookup(nil)
	require.NotNil(t, fn)
	assert.Equal(t, "ok", fn("RUNE_TEST_LOOKUP"))
}
