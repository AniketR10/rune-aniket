// Copyright (C) 2017-2026 The Rune Authors
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

package llamaserver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

// fakeLibDir is a pkgLibDirLister backed by a fixed slice of paths (or an
// error) so locator tests never touch a real package manager.
type fakeLibDir struct {
	paths []string
	err   error
}

func (f fakeLibDir) LibDir(
	context.Context, string,
) (iterator.Iterator[string], error) {
	if f.err != nil {
		return nil, f.err
	}
	return iterator.FromSlice(f.paths), nil
}

func TestNewPkgLocator_PanicsOnNil(t *testing.T) {
	assert.Panics(t, func() { NewPkgLocator(nil) })
}

func TestPkgLocator_ResolvesBinaryInLibDir(t *testing.T) {
	// LibDir yields the file paths the package shipped on the workspace host;
	// the locator matches by base name without touching the local filesystem.
	bin := "/remote/host/lib/llama-server/" + serverBinaryName
	loc := NewPkgLocator(fakeLibDir{paths: []string{
		"/remote/host/lib/llama-server/README.md",
		bin,
	}})
	got, err := loc.Locate(context.Background())
	require.NoError(t, err)
	assert.Equal(t, bin, got)
}

func TestPkgLocator_NotInstalledWhenAbsent(t *testing.T) {
	loc := NewPkgLocator(fakeLibDir{paths: nil})
	_, err := loc.Locate(context.Background())
	assert.ErrorIs(t, err, ErrServerNotInstalled)
}

func TestPkgLocator_NotInstalledWhenBinaryMissing(t *testing.T) {
	// LibDir returns files, but none is llama-server.
	loc := NewPkgLocator(fakeLibDir{paths: []string{"/lib/llama-server/notes.txt"}})
	_, err := loc.Locate(context.Background())
	assert.ErrorIs(t, err, ErrServerNotInstalled)
}

func TestPkgLocator_NotInstalledWhenLibDirErrors(t *testing.T) {
	loc := NewPkgLocator(fakeLibDir{err: assert.AnError})
	_, err := loc.Locate(context.Background())
	assert.ErrorIs(t, err, ErrServerNotInstalled)
}

func TestFixedLocator(t *testing.T) {
	loc := NewFixedLocator("/usr/bin/llama-server")
	got, err := loc.Locate(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "/usr/bin/llama-server", got)

	_, err = NewFixedLocator("").Locate(context.Background())
	assert.ErrorIs(t, err, ErrServerNotInstalled)
}
