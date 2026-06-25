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
// module entry files mod.rs, lib.rs, and main.rs.
func rustModuleFromURI(uri string) string {
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
