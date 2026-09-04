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
