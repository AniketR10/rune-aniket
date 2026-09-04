// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package idecmd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"unstable.build/rune/text/cmdenv"
)

func TestIsPluginTarget(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		target string
		want   bool
	}{
		{"! echo hi", true},
		{"!! VAR=$(echo hi)", true},
		{"!foo", false},
		{"edit a.go", false},
		{"  !\techo hi", true},
	} {
		t.Run(tc.target, func(t *testing.T) {
			assert.Equal(t, tc.want, isPluginTarget(tc.target))
		})
	}
}

func TestExpandTargetPositional(t *testing.T) {
	t.Parallel()
	argv, ref, err := expandTarget(
		context.Background(),
		"edit $1 hellagood",
		[]string{"arg1"},
		nil,
		nil,
	)
	require.NoError(t, err)
	assert.Equal(t, []string{"edit", "arg1", "hellagood"}, argv)
	_, used := ref[0]
	assert.True(t, used)
}

func TestExpandTargetMissingPositional(t *testing.T) {
	t.Parallel()
	_, _, err := expandTarget(
		context.Background(),
		"edit $2",
		[]string{"arg1"},
		nil,
		nil,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "$2")
}

func TestExpandTargetPluginBodyPreservesCommandSubst(t *testing.T) {
	t.Parallel()
	argv, _, err := expandTarget(
		context.Background(),
		`!! echo "$(echo hello)"`,
		nil,
		nil,
		nil,
	)
	require.NoError(t, err)
	require.Len(t, argv, 2)
	assert.Equal(t, "!!", argv[0])
	assert.Contains(t, argv[1], "$(echo hello)")
}

func TestExpandTargetChainCapture(t *testing.T) {
	t.Parallel()
	chain := NewChain()
	chain.Set("VAR", "captured")
	argv, _, err := expandTarget(
		context.Background(),
		"edit $VAR",
		nil,
		nil,
		chain,
	)
	require.NoError(t, err)
	assert.Equal(t, []string{"edit", "captured"}, argv)
}

func TestExpandTargetDollarDollarEscape(t *testing.T) {
	t.Parallel()
	argv, _, err := expandTarget(
		cmdenv.WithCommandSubstitution(context.Background()),
		`echo $$1`,
		[]string{"arg1"},
		nil,
		nil,
	)
	require.NoError(t, err)
	assert.Equal(t, []string{"echo", "$1"}, argv)
}
