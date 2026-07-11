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

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunnableCommandCargo(t *testing.T) {
	r := runnable{
		Label: "run e2e",
		Kind:  "cargo",
		Args: runnableArgsJSON{
			WorkspaceRoot: "/ws",
			CargoArgs:     []string{"run", "--package", "e2e", "--bin", "e2e"},
		},
	}
	spec, err := runnableCommand(r, "/fallback")
	require.NoError(t, err)
	assert.Equal(t, "cargo", spec.path)
	assert.Equal(t, []string{"run", "--package", "e2e", "--bin", "e2e"}, spec.args)
	assert.Equal(t, "/ws", spec.dir)
}

func TestRunnableCommandCargoExecutableArgs(t *testing.T) {
	r := runnable{
		Kind: "cargo",
		Args: runnableArgsJSON{
			CargoArgs:      []string{"test", "--package", "e2e", "--", "some::test"},
			ExecutableArgs: []string{"--nocapture", "--exact"},
		},
	}
	spec, err := runnableCommand(r, "/fallback")
	require.NoError(t, err)
	// Executable args follow a `--` separator appended after the cargo args.
	assert.Equal(t, []string{
		"test", "--package", "e2e", "--", "some::test",
		"--", "--nocapture", "--exact",
	}, spec.args)
	// A missing workspaceRoot falls back to the provided directory.
	assert.Equal(t, "/fallback", spec.dir)
}

func TestRunnableCommandOverrideCargo(t *testing.T) {
	r := runnable{
		Kind: "cargo",
		Args: runnableArgsJSON{
			WorkspaceRoot: "/ws",
			CargoArgs:     []string{"run"},
			OverrideCargo: "cross build",
		},
	}
	spec, err := runnableCommand(r, "/fallback")
	require.NoError(t, err)
	// overrideCargo is space-split: first token is the program, the rest is
	// prepended to the cargo args.
	assert.Equal(t, "cross", spec.path)
	assert.Equal(t, []string{"build", "run"}, spec.args)
}

func TestRunnableCommandShell(t *testing.T) {
	r := runnable{
		Kind: "shell",
		Args: runnableArgsJSON{
			Program: "bash",
			Args:    []string{"-c", "echo hi"},
			Cwd:     "/tmp",
		},
	}
	spec, err := runnableCommand(r, "/fallback")
	require.NoError(t, err)
	assert.Equal(t, "bash", spec.path)
	assert.Equal(t, []string{"-c", "echo hi"}, spec.args)
	assert.Equal(t, "/tmp", spec.dir)
}

func TestRunnableCommandUnknownKind(t *testing.T) {
	_, err := runnableCommand(runnable{Kind: "wat"}, "/fallback")
	require.Error(t, err)
}

func TestRunnableEnvLayersRuntimeVars(t *testing.T) {
	env := runnableEnv(map[string]string{"FOO": "bar"})
	assert.Contains(t, env, "RUST_BACKTRACE=short")
	assert.Contains(t, env, "FOO=bar")
	// The inherited environment is materialized so RUST_BACKTRACE can be added.
	assert.Greater(t, len(env), 2)
}
