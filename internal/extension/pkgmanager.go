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

package extension

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/rune/internal/workspace/walkdir"
)

// ErrNotInstalled is returned when a package is not installed.
var ErrNotInstalled = errors.New("package is not installed")

// PkgManager provides a view of the packages installed.
type PkgManager struct {
	fs      workspaceapi.FileSystem
	cwd     workspaceapi.URI
	dataDir string
}

// NewPkgManager allocates storage for a new PkgManager and initializes it.
func NewPkgManager(
	dataDir string, cwd workspaceapi.URI, fs workspaceapi.FileSystem,
) *PkgManager {
	return &PkgManager{
		fs:      fs,
		cwd:     cwd,
		dataDir: dataDir,
	}
}

// LibDir returns an iterator to the lib directory of the given package.
// The paths returned by the iterator are always absolute.
func (m *PkgManager) LibDir(ctx context.Context, pkgID string) (
	iterator.Iterator[string], error,
) {
	libDir := makePackageLibDirname(m.dataDir, pkgID)

	_, err := os.Stat(libDir)
	if err != nil {
		return nil, ErrNotInstalled
	}

	return getFiles(ctx, m.fs, m.cwd, libDir), nil
}

func getFiles(
	ctx context.Context, cwd workspaceapi.FileSystem,
	schemeURI workspaceapi.URI, libDir string,
) iterator.Iterator[string] {
	it, err := walkdir.ListFiles(ctx, cwd, libDir)
	if err != nil {
		return iterator.Error[string](fmt.Errorf("list files: %v", err))
	}
	// make paths absolute
	return iterator.Map(it, func(filename string) string {
		path, _ := workspaceapi.ExpandPathWithURI(filename, schemeURI)
		return path
	})
}

func makePackageLibDirname(
	dataDir, pkgID string,
) string {
	return filepath.Join(dataDir, "lib", pkgID)
}
