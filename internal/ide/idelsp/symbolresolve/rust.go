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

package symbolresolve

import (
	"path"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

// Rust is the symbol-resolution spec for the Rust language. Rust has no
// per-file package clause, so qualifiers are derived from the module
// file name (the final path segment of a `mod::Symbol` reference). The
// definitions phase cannot observe `pub` visibility from tree-sitter
// captures, so every name is treated as visible.
var Rust = &Spec{
	LangID: "rust",
	RefQueries: []RefQuery{
		{
			Query: `(scoped_type_identifier ` +
				`path: (identifier) @pkg name: (type_identifier) @type)`,
			Captures: []string{"pkg", "type"},
		},
		{
			Query: `(scoped_identifier ` +
				`path: (identifier) @pkg name: (identifier) @symbol)`,
			Captures: []string{"pkg", "symbol"},
		},
	},
	IsExported:         nil,
	Qualifier:          rustModuleFromURI,
	DisplayPathFromURI: rustDirFromURI,
	Extensions:         []string{".rs"},
	MethodDefQuery: `(impl_item type: (type_identifier) @recv ` +
		`body: (declaration_list (function_item name: (identifier) @method)))`,
	MethodDefCaptures: []string{"recv", "method"},
}

// rustModuleFromURI derives a Rust module name from a file URI: the base
// file name without its extension, or the parent directory name for the
// module entry files mod.rs, lib.rs, and main.rs. The qualifier context
// is unused: Rust resolution keeps single-segment module names.
func rustModuleFromURI(_ QualifierContext, uri string) string {
	parsed, err := workspaceapi.ParseURI(uri)
	if err != nil {
		return uri
	}
	base := path.Base(parsed.Path())
	stem := strings.TrimSuffix(base, path.Ext(base))
	switch stem {
	case "mod", "lib", "main":
		return path.Base(path.Dir(parsed.Path()))
	}
	return stem
}

// rustDirFromURI returns the directory path of a file URI.
func rustDirFromURI(uri string) string {
	parsed, err := workspaceapi.ParseURI(uri)
	if err != nil {
		return uri
	}
	return path.Dir(parsed.Path())
}
