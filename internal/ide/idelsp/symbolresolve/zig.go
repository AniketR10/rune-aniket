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

// Zig is the symbol-resolution spec for the Zig language. Zig has no
// per-file package clause: files are imported as values (`const geo =
// @import("geometry.zig")`) and members are reached with field access,
// so qualifiers are derived from the module file name like Rust and
// qualified references are field expressions like Python attributes.
// Visibility (`pub`) is not observable from tree-sitter captures, so
// every name is treated as visible.
var Zig = &Spec{
	LangID: "zig",
	RefQueries: []RefQuery{
		{
			Query: `(field_expression object: (identifier) @pkg ` +
				`member: (identifier) @member)`,
			Captures:      []string{"pkg", "member"},
			RequireImport: true,
		},
	},
	// `const alias = @import("path.zig")` binds the module to alias.
	// The structural query cannot distinguish @import from other
	// builtin calls that take a string (e.g. @embedFile); those rare
	// extra bindings are harmless for import-aware deduplication.
	ImportPathQuery: `(variable_declaration (identifier) ` +
		`(builtin_function (builtin_identifier) ` +
		`(arguments (string) @path)))`,
	ImportPathCaptures: []string{"path"},
	ImportAliasQuery: `(variable_declaration (identifier) @alias ` +
		`(builtin_function (builtin_identifier) ` +
		`(arguments (string) @path)))`,
	ImportAliasCaptures: []string{"alias", "path"},
	IsExported:          nil,
	Qualifier:           zigModuleFromURI,
	DisplayPathFromURI:  zigDirFromURI,
	Extensions:          []string{".zig"},
	MethodDefQuery: `(variable_declaration (identifier) @recv ` +
		`[(struct_declaration (function_declaration name: (identifier) @method)) ` +
		`(enum_declaration (function_declaration name: (identifier) @method)) ` +
		`(union_declaration (function_declaration name: (identifier) @method))])`,
	MethodDefCaptures: []string{"recv", "method"},
}

// zigModuleFromURI derives a Zig module name from a file URI: the base
// file name without its extension, or the parent directory name for
// the entry files main.zig and root.zig. The qualifier context is
// unused: Zig resolution keeps single-segment module names.
func zigModuleFromURI(_ QualifierContext, uri string) string {
	parsed, err := workspaceapi.ParseURI(uri)
	if err != nil {
		return uri
	}
	base := path.Base(parsed.Path())
	stem := strings.TrimSuffix(base, path.Ext(base))
	switch stem {
	case "main", "root":
		return path.Base(path.Dir(parsed.Path()))
	}
	return stem
}

// zigDirFromURI returns the directory path of a file URI.
func zigDirFromURI(uri string) string {
	parsed, err := workspaceapi.ParseURI(uri)
	if err != nil {
		return uri
	}
	return path.Dir(parsed.Path())
}
