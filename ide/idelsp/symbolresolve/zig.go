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
