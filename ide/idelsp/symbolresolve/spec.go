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

import "strings"

// RefQuery is a tree-sitter query that captures a package-qualified
// reference. Captures[0] must capture the package alias and Captures[1]
// the referenced symbol name.
type RefQuery struct {
	Query    string
	Captures []string

	// RequireImport restricts matches to files that import the captured
	// package alias. Used by the completer to drop dot-imported or
	// same-package references that would otherwise look package-qualified.
	RequireImport bool
}

// Spec describes how to resolve and complete package-qualified symbol
// names for one language. The tree-sitter queries it carries are scoped
// to LangID by the parser, so applying a Spec to a workspace that
// contains no files of that language is a cheap no-op.
type Spec struct {
	// LangID is the tree-sitter language identifier (e.g. "go",
	// "python") used to scope parser.Search.
	LangID string

	// RefQueries are run in order to find package-qualified references
	// (qualified types, selector/attribute access, …).
	RefQueries []RefQuery

	// PackageClauseQuery, when non-empty, derives the package name a
	// file declares. Captures[0] of the query must capture the package
	// identifier. When empty, Qualifier is used to derive the
	// bare-definition prefix instead.
	PackageClauseQuery    string
	PackageClauseCaptures []string

	// ImportPathQuery and ImportAliasQuery support import-aware
	// deduplication. When ImportPathQuery is empty, dedup-by-import is
	// skipped.
	ImportPathQuery     string
	ImportPathCaptures  []string
	ImportAliasQuery    string
	ImportAliasCaptures []string

	// IsExported reports whether a bare definition name is visible
	// outside its package. A nil predicate means every name is visible.
	IsExported func(string) bool

	// MethodDefQuery captures method definitions as (receiverType,
	// methodName) pairs for resolving pkg.Type.Method names. Captures[0]
	// is the receiver type, Captures[1] the method name. Empty disables
	// method resolution for this language.
	MethodDefQuery    string
	MethodDefCaptures []string

	// Qualifier returns the prefix applied to bare definition names
	// during the definitions phase when the language has no package
	// clause (e.g. a Python module name derived from the file path).
	Qualifier func(uri string) string

	// DisplayPathFromURI derives a human-readable display prefix from a
	// file URI, used when disambiguating matches.
	DisplayPathFromURI func(uri string) string

	// Extensions lists the file-name suffixes (including the dot) that
	// belong to this language. The definitions phase uses them to scope
	// the cross-language SearchNode/QueryNode results to this spec's
	// files. Empty means no extension filtering.
	Extensions []string
}

// exported reports whether name passes the spec's export predicate. A
// nil predicate accepts every name.
func (s *Spec) exported(name string) bool {
	if s.IsExported == nil {
		return true
	}
	return s.IsExported(name)
}

// hasPackages reports whether the spec derives qualifiers from a package
// clause rather than from the file path.
func (s *Spec) hasPackages() bool {
	return s.PackageClauseQuery != ""
}

// hasMethods reports whether the spec can resolve pkg.Type.Method names.
func (s *Spec) hasMethods() bool {
	return s.MethodDefQuery != ""
}

// qualifier returns the bare-definition prefix for a file URI when the
// spec has no package clause; it returns "" when Qualifier is nil.
func (s *Spec) qualifier(uri string) string {
	if s.Qualifier == nil {
		return ""
	}
	return s.Qualifier(uri)
}

// displayPath derives the display prefix for a file URI, falling back to
// the raw URI when DisplayPathFromURI is nil.
func (s *Spec) displayPath(uri string) string {
	if s.DisplayPathFromURI == nil {
		return uri
	}
	return s.DisplayPathFromURI(uri)
}

// matchesFile reports whether uri belongs to this language according to
// its extensions. An empty Extensions list matches every file.
func (s *Spec) matchesFile(uri string) bool {
	if len(s.Extensions) == 0 {
		return true
	}
	for _, ext := range s.Extensions {
		if strings.HasSuffix(uri, ext) {
			return true
		}
	}
	return false
}
