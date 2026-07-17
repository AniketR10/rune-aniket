// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

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
