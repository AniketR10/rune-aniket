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

package idelsp

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/rune/internal/ide/idelsp/languages"
)

// ErrLanguageNotSupported is returned when a file's language has no
// LSP server configuration. The message must keep the literal
// "language LSP is not supported yet" suffix: agent-side code matches
// it across the gRPC boundary, where sentinels do not survive.
var ErrLanguageNotSupported = errors.New("language LSP is not supported yet")

// PkgManager abstracts the ability to resolve package
// directories for a given package identifier.
type PkgManager interface {
	// LibDir returns an iterator of directory paths where
	// the package's binaries may be found.
	LibDir(ctx context.Context, pkgID string) (
		iterator.Iterator[string], error,
	)
}

type langConfig struct {
	id      string
	command string
	args    []string
	env     []string
}

var langConfigs = map[string]langConfig{
	"go": {
		id:      "go",
		command: "gopls",
		//args: []string{"-mode", "stdio", "-logfile",
		//"/Users/ernestrc/.rune/gopls.log", "-debug", "127.0.0.1:3636", "-rpc.trace", "-v"},
		args: []string{"serve"},
	},
	"python": {
		id:      "python",
		command: "pyright-langserver",
		args:    []string{"--stdio"},
	},
	"typescript": {
		id:      "typescript",
		command: "typescript-language-server",
		args:    []string{"--stdio"},
	},
	"rust": {
		id:      "rust",
		command: "rust-analyzer",
		args:    nil,
	},
	"c": {
		id:      "c",
		command: "clangd",
		args:    nil,
	},
	"zig": {
		id:      "zig",
		command: "zls",
		args:    nil,
	},
}

func languageForFile(filename workspaceapi.URI) (langConfig, error) {
	return languageForFilename(filename.Path())
}

func languageForFilename(filename string) (langConfig, error) {
	id, err := languages.LanguageForFile(filepath.Base(filename))
	if err != nil {
		return langConfig{}, err
	}
	lang, ok := langConfigs[id]
	if !ok {
		return langConfig{}, fmt.Errorf("%s %w", id, ErrLanguageNotSupported)
	}
	return lang, nil
}
