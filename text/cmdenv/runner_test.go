// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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

package cmdenv

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunnerDirSeedsInterpCwd(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	r := Runner{
		Executor: &osExecutor{},
		Dir:      dir,
		Stdout:   &out,
	}
	_, err := r.Run(context.Background(), "pwd", nil)
	require.NoError(t, err)
	assert.Equal(t, dir+"\n", out.String())
}

func TestRunnerCdRelativeIsAnchoredAtDir(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	r := Runner{
		Executor: &osExecutor{},
		Dir:      dir,
		Stdout:   &out,
	}
	_, err := r.Run(context.Background(),
		`ROOT=$(cd . && pwd); echo $ROOT`, nil)
	require.NoError(t, err)
	assert.Equal(t, dir+"\n", out.String())
}

func TestRunnerCapturesParameterExpansionResult(t *testing.T) {
	r := Runner{Executor: &osExecutor{}}
	captured, err := r.Run(context.Background(),
		`ROOT=/Users/ernestrc/src/idelsp; ROOT_NAME=${ROOT##*/}`, nil)
	require.NoError(t, err)
	assert.Equal(t, "idelsp", captured["ROOT_NAME"])
}

func TestRunnerCapturesParameterExpansionFromEnvSource(t *testing.T) {
	src := Source(func(name string) (string, bool) {
		if name == "ROOT" {
			return "/Users/ernestrc/src/idelsp", true
		}
		return "", false
	})
	r := Runner{Executor: &osExecutor{}, EnvSource: src}
	captured, err := r.Run(context.Background(),
		`ROOT_NAME=${ROOT##*/}`, nil)
	require.NoError(t, err)
	assert.Equal(t, "idelsp", captured["ROOT_NAME"])
}

func TestRunnerCapturesParameterExpansionAfterExpandBody(t *testing.T) {
	src := Source(func(name string) (string, bool) {
		if name == "ROOT" {
			return "/Users/ernestrc/src/idelsp", true
		}
		return "", false
	})
	line := ExpandBody(context.Background(),
		`ROOT_NAME=${ROOT##*/}`, src)
	r := Runner{Executor: &osExecutor{}, EnvSource: src}
	captured, err := r.Run(context.Background(), line, nil)
	require.NoError(t, err)
	assert.Equal(t, "idelsp", captured["ROOT_NAME"])
}

func TestRunnerCapturesDoubleAssignmentWithShortCircuit(t *testing.T) {
	dir := t.TempDir()
	r := Runner{Executor: &osExecutor{}, Dir: dir}
	line := `ROOT=$(echo ".git") && ROOT=$(cd "$ROOT/.." && pwd) || exit 1`
	captured, err := r.Run(context.Background(), line, nil)
	require.NoError(t, err)
	assert.Equal(t, dir, captured["ROOT"],
		"chain capture must reflect the second assignment to ROOT")
}

func TestRunnerOnlyCapturesScriptAssignedVars(t *testing.T) {
	src := Source(func(name string) (string, bool) {
		if name == "ROOT" {
			return "/Users/ernestrc/src/idelsp", true
		}
		return "", false
	})
	r := Runner{Executor: &osExecutor{}, EnvSource: src}
	captured, err := r.Run(context.Background(),
		`ROOT_HASH=hashof_$ROOT`, nil)
	require.NoError(t, err)
	assert.Equal(t, "hashof_/Users/ernestrc/src/idelsp", captured["ROOT_HASH"])
	_, hasROOT := captured["ROOT"]
	assert.False(t, hasROOT,
		"parent-env ROOT must not leak into captured vars")
}
