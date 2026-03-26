// Copyright 2026 Unstable Build, LLC.
//
// This program is free software: you can redistribute it and/or modify it
// under the terms of the GNU General Public License as published by the
// Free Software Foundation, either version 3 of the License, or (at your
// option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// See <https://www.gnu.org/licenses/> for a copy of the license.

package idedebug

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

// PkgManager abstracts the ability to resolve package
// directories for a given package identifier.
type PkgManager interface {
	// LibDir returns an iterator of directory paths where
	// the package's binaries may be found.
	LibDir(ctx context.Context, pkgID string) (iterator.Iterator[string], error)
}

type debugConfig struct {
	id      string
	command string
	// args are passed to the debug adapter binary.
	// The placeholder {addr} is replaced at runtime
	// with the TCP address the adapter should listen
	// on (e.g. "127.0.0.1:56789").
	args []string
}

var _debugAdapters = map[string]debugConfig{
	"go": {
		id:      "go",
		command: "dlv",
		args:    []string{"dap", "--listen={addr}"},
	},
}

func debugAdapterForFile(filename workspaceapi.URI) (debugConfig, error) {
	return debugAdapterForFilename(filename.Path())
}

func doDebugAdapterForFile(filename string) (string, error) {
	ext := filepath.Ext(filename)
	switch ext {
	case ".go":
		return "go", nil
	default:
		return "", errors.New("unsupported language")
	}
}

func debugAdapterForFilename(filename string) (debugConfig, error) {
	id, err := doDebugAdapterForFile(filepath.Base(filename))
	if err != nil {
		return debugConfig{}, err
	}
	cfg, ok := _debugAdapters[id]
	if !ok {
		return debugConfig{}, fmt.Errorf("%s language debug adapter is not supported yet", id)
	}
	return cfg, nil
}
