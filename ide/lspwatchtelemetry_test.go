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

package ide

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
)

// stubWatchLSP embeds the interface so only the methods the test exercises
// need an implementation; any other call is a test bug and nil-panics.
type stubWatchLSP struct {
	semanticapi.LSP
	watched     []semanticapi.DidChangeWatchedFilesParams
	definitions int
}

func (s *stubWatchLSP) DidChangeWatchedFiles(
	_ context.Context, params semanticapi.DidChangeWatchedFilesParams,
) error {
	s.watched = append(s.watched, params)
	return nil
}

func (s *stubWatchLSP) Definition(
	_ context.Context, _ semanticapi.DefinitionParams,
) (semanticapi.LocationResult, error) {
	s.definitions++
	return semanticapi.LocationResult{}, nil
}

func TestWatchedFilesTelemetryLSP(t *testing.T) {
	stub := new(stubWatchLSP)
	var counts []int
	lsp := semanticapi.LSP(watchedFilesTelemetryLSP{
		LSP:      stub,
		onChange: func(n int) { counts = append(counts, n) },
	})

	params := semanticapi.DidChangeWatchedFilesParams{
		Changes: []semanticapi.FileEvent{
			{URI: "file:///a.go", Type: semanticapi.FileChangeTypeChanged},
			{URI: "file:///b.go", Type: semanticapi.FileChangeTypeCreated},
		},
	}
	require.NoError(t, lsp.DidChangeWatchedFiles(context.Background(), params))
	assert.Equal(t, []int{2}, counts)
	assert.Equal(t, []semanticapi.DidChangeWatchedFilesParams{params}, stub.watched)

	_, err := lsp.Definition(context.Background(), semanticapi.DefinitionParams{})
	require.NoError(t, err)
	assert.Equal(t, 1, stub.definitions)
	assert.Equal(t, []int{2}, counts)
}
