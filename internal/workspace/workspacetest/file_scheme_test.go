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

package workspacetest

import (
	"context"

	os "os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/rune/internal/workspace"
)

func TestFileScheme(t *testing.T) {
	TestWorkspaceSchemeFiles(t, func(t *testing.T) schemeapi.Scheme {
		dir, err := os.MkdirTemp("/tmp", "file_scheme_suite")
		require.NoError(t, err)

		workspaceURI, err := workspaceapi.ParseURI("file://" + dir)
		require.NoError(t, err)

		fileScheme, err := workspace.NewFileScheme(
			context.Background(), config.NopConfig(), workspaceURI)
		require.NoError(t, err)

		t.Cleanup(func() {
			_ = os.RemoveAll(dir)
		})
		return fileScheme
	})

	TestWorkspaceSchemeExecutor(t, func(t *testing.T) schemeapi.Scheme {
		dir, err := os.MkdirTemp("", "file_scheme_suite")
		require.NoError(t, err)

		workspaceURI, err := workspaceapi.ParseURI("file://" + dir)
		require.NoError(t, err)

		fileScheme, err := workspace.NewFileScheme(
			context.Background(), config.NopConfig(), workspaceURI)
		require.NoError(t, err)

		t.Cleanup(func() {
			_ = os.RemoveAll(dir)
		})
		return fileScheme
	})
}
