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

// Go is the symbol-resolution spec for the Go language.
var Go = &Spec{
	LangID: "go",
	RefQueries: []RefQuery{
		{
			Query: `(qualified_type package: (package_identifier) @pkg ` +
				`name: (type_identifier) @type)`,
			Captures: []string{"pkg", "type"},
		},
		{
			Query: `(selector_expression operand: (identifier) @pkg ` +
				`field: (field_identifier) @symbol)`,
			Captures:      []string{"pkg", "symbol"},
			RequireImport: true,
		},
	},
	PackageClauseQuery:    `(package_clause (package_identifier) @pkg)`,
	PackageClauseCaptures: []string{"pkg"},
	ImportPathQuery: `(import_spec ` +
		`path: (interpreted_string_literal) @path)`,
	ImportPathCaptures: []string{"path"},
	ImportAliasQuery: `(import_spec name: (package_identifier) @alias ` +
		`path: (interpreted_string_literal) @path)`,
	ImportAliasCaptures: []string{"alias", "path"},
	IsExported:          isExported,
	DisplayPathFromURI:  goPackagePathFromURI,
	Extensions:          []string{".go"},
}

// goPackagePathFromURI derives a Go-style display prefix from a file
// URI. For Go module cache paths it recovers the import path by
// stripping "@version" segments; otherwise it returns the file's
// directory path. The URI scheme is ignored.
func goPackagePathFromURI(uri string) string {
	parsed, err := workspaceapi.ParseURI(uri)
	if err != nil {
		return uri
	}
	dir := path.Dir(parsed.Path())
	if idx := strings.Index(dir, "/pkg/mod/"); idx != -1 {
		modPath := dir[idx+len("/pkg/mod/"):]
		parts := strings.Split(modPath, "/")
		for i, part := range parts {
			if atIdx := strings.Index(part, "@"); atIdx != -1 {
				parts[i] = part[:atIdx]
			}
		}
		return strings.Join(parts, "/")
	}
	return dir
}

func isExported(name string) bool {
	return len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z'
}

// IsExported reports whether name starts with an uppercase ASCII
// letter, matching Go's export rules.
func IsExported(name string) bool { return isExported(name) }
