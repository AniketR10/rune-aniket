// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package idecmd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"unstable.build/go-tui/text/cmdenv"
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
