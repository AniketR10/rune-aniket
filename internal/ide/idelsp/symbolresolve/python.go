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

package symbolresolve

import (
	"path"
	"slices"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

// Python is the symbol-resolution spec for the Python language. Python
// has no package clause and no enforced visibility, so qualifiers are
// derived from the module file path (dotted through parent package
// directories) and every name is considered visible.
var Python = &Spec{
	LangID: "python",
	RefQueries: []RefQuery{
		{
			Query: `(attribute object: (identifier) @pkg ` +
				`attribute: (identifier) @attr)`,
			Captures: []string{"pkg", "attr"},
		},
	},
	IsExported:         nil,
	NestedModules:      true,
	Qualifier:          pythonModuleFromURI,
	DisplayPathFromURI: pythonDirFromURI,
	Extensions:         []string{".py", ".pyi"},
	MethodDefQuery: `(class_definition name: (identifier) @recv ` +
		`body: (block (function_definition name: (identifier) @method)))`,
	MethodDefCaptures: []string{"recv", "method"},
	// Names bound by "from X import Y" (or "... as Z") in a package
	// __init__ are re-exports: the package's public definition sites.
	ReexportQuery: `[(import_from_statement ` +
		`name: (dotted_name (identifier) @name .)) ` +
		`(import_from_statement ` +
		`(aliased_import alias: (identifier) @name))]`,
	ReexportCaptures: []string{"name"},
	ReexportFiles:    []string{"__init__.py", "__init__.pyi"},
}

// pythonModuleFromURI derives a Python module path from a file URI: the
// base file name without its extension (or the directory name for a
// package's __init__), prefixed with a dotted segment for every parent
// directory that is itself a package (contains __init__.py/.pyi).
// Files outside the context root keep the single-segment form.
func pythonModuleFromURI(qc QualifierContext, uri string) string {
	parsed, err := workspaceapi.ParseURI(uri)
	if err != nil {
		return uri
	}
	base := path.Base(parsed.Path())
	stem := strings.TrimSuffix(base, path.Ext(base))
	var dir string
	if rel := qc.relPath(parsed); rel != "" {
		dir = path.Dir(rel)
	}
	if stem == "__init__" {
		if dir == "" || dir == "." || dir == "/" {
			return path.Base(path.Dir(parsed.Path()))
		}
		stem = path.Base(dir)
		dir = path.Dir(dir)
	}
	segments := []string{stem}
	for dir != "" && dir != "." && dir != "/" && pythonPackageDir(qc, dir) {
		segments = append(segments, path.Base(dir))
		dir = path.Dir(dir)
	}
	slices.Reverse(segments)
	return strings.Join(segments, ".")
}

// pythonPackageDir reports whether the workspace-relative directory is
// a Python package.
func pythonPackageDir(qc QualifierContext, dir string) bool {
	return qc.Exists(path.Join(dir, "__init__.py")) ||
		qc.Exists(path.Join(dir, "__init__.pyi"))
}

// pythonDirFromURI returns the directory path of a file URI.
func pythonDirFromURI(uri string) string {
	parsed, err := workspaceapi.ParseURI(uri)
	if err != nil {
		return uri
	}
	return path.Dir(parsed.Path())
}
