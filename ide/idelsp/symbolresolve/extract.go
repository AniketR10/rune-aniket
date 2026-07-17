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
	"context"
	"io"
	"path"

	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// FileQueryer runs batched per-file queries for symbol extraction:
// every query in a batch runs against a single parse of the file.
// Consumers such as ExtractFile borrow the queryer and do not own its
// lifecycle; see QuerySession for the owned form.
type FileQueryer interface {
	// QueryMulti runs every query against a single parse of file,
	// returning each match's captures grouped per the query's contract.
	QueryMulti(ctx context.Context, file workspaceapi.URI, queries []MultiQuery) (
		[]MultiResult, error,
	)
}

// QuerySession is an owned FileQueryer: implementations cache loaded
// languages and compiled query batches across calls, so the creator
// must Close it to release them. Sessions need not be safe for
// concurrent use; create one per goroutine.
type QuerySession interface {
	FileQueryer
	io.Closer
}

// SymbolKind classifies one extracted symbol occurrence.
type SymbolKind int

const (
	// SymbolRef is a package-qualified reference site.
	SymbolRef SymbolKind = iota
	// SymbolDef is an exported function or type definition.
	SymbolDef
	// SymbolMethodDef is a method definition, named pkg.Type.Method.
	SymbolMethodDef
)

// FileSymbol is one symbol occurrence extracted from a file.
type FileSymbol struct {
	// Name is the qualified symbol name: pkg.Sym for references and
	// definitions, pkg.Type.Method for method definitions.
	Name string
	Pos  term.Coordinates
	Kind SymbolKind
}

// FileExtraction is one file's contribution to a symbol index.
type FileExtraction struct {
	Symbols []FileSymbol
	// Imports maps package aliases in scope to their import paths.
	Imports map[string]string
}

// ExtractFile computes one file's contribution to a symbol index by
// running every query the spec needs in one batch against a single
// parse of the file. It applies the same rules as the workspace-wide
// resolver: references are filtered by the spec's export predicate
// and, when a reference query requires it, by the file's resolved
// imports; definitions are qualified by the package clause (or the
// spec's path-derived qualifier) and filtered by the export predicate;
// method definitions are emitted as pkg.Type.Method. Definitions of
// nested-module specs are emitted once per dotted suffix of the
// file's module path, so a symbol is addressable by any trailing run
// of module segments. Re-export bindings in the spec's re-export
// files are emitted as definitions under the same qualifiers.
func ExtractFile(
	ctx context.Context, parser FileQueryer, spec *Spec, qc QualifierContext,
	file workspaceapi.URI,
) (FileExtraction, error) {
	refCount := len(spec.RefQueries)
	queries := make([]MultiQuery, 0, refCount+5)
	for i, rq := range spec.RefQueries {
		queries = append(queries, MultiQuery{
			ID: i, Query: rq.Query, Captures: rq.Captures,
		})
	}
	id := refCount
	nextID := func() int { n := id; id++; return n }
	pkgID, pathID, aliasID, methodID, reexportID := -1, -1, -1, -1, -1
	if spec.hasPackages() {
		pkgID = nextID()
		queries = append(queries, MultiQuery{
			ID: pkgID, Query: spec.PackageClauseQuery,
			Captures: spec.PackageClauseCaptures,
		})
	}
	if spec.ImportPathQuery != "" {
		pathID = nextID()
		queries = append(queries, MultiQuery{
			ID: pathID, Query: spec.ImportPathQuery,
			Captures: spec.ImportPathCaptures,
		})
		aliasID = nextID()
		queries = append(queries, MultiQuery{
			ID: aliasID, Query: spec.ImportAliasQuery,
			Captures: spec.ImportAliasCaptures,
		})
	}
	defsID := nextID()
	queries = append(queries, MultiQuery{
		ID:    defsID,
		Nodes: syntaxapi.NodeCaptureDefinitionFunc | syntaxapi.NodeCaptureDefinitionType,
	})
	if spec.hasMethods() {
		methodID = nextID()
		queries = append(queries, MultiQuery{
			ID: methodID, Query: spec.MethodDefQuery,
			Captures: spec.MethodDefCaptures,
		})
	}
	// The re-export query joins the batch for every file of the
	// language — not only re-export files — so sessions that cache the
	// compiled batch per language never recompile it; results from
	// other files are dropped below.
	if spec.hasReexports() {
		reexportID = nextID()
		queries = append(queries, MultiQuery{
			ID: reexportID, Query: spec.ReexportQuery,
			Captures: spec.ReexportCaptures,
		})
	}

	results, err := parser.QueryMulti(ctx, file, queries)
	if err != nil {
		return FileExtraction{}, err
	}

	// References need the resolved imports and definitions need the
	// package qualifier, so reduce those first.
	pkg := ""
	if !spec.hasPackages() {
		pkg = spec.qualifier(qc, file.String())
	}
	var paths map[workspaceapi.URI][]string
	var explicit map[workspaceapi.URI]map[string]string
	for _, r := range results {
		switch r.QueryID {
		case pkgID:
			pkg = r.Match[0].Text
		case pathID:
			paths, _ = reduceImportPaths(paths, r.Match[0])
		case aliasID:
			explicit, _ = reduceImportAliases(
				explicit, [2]syntaxapi.Result{r.Match[0], r.Match[1]})
		}
	}
	var ext FileExtraction
	// Single-file batch: take the sole entry rather than keying by the
	// caller's URI, which may not compare equal to parser-built ones.
	for _, aliases := range resolveAliases(paths, explicit) {
		ext.Imports = aliases
	}

	var pkgs []string
	if pkg != "" {
		pkgs = moduleSuffixes(pkg)
	}
	isReexportFile := reexportID >= 0 &&
		spec.isReexportFile(path.Base(file.Path()))
	for _, r := range results {
		switch {
		case r.QueryID < refCount:
			p0, p1 := r.Match[0], r.Match[1]
			if !spec.exported(p1.Text) {
				continue
			}
			if spec.RefQueries[r.QueryID].RequireImport &&
				ext.Imports[p0.Text] == "" {
				continue
			}
			ext.Symbols = append(ext.Symbols, FileSymbol{
				Name: p0.Text + "." + p1.Text, Pos: p1.From, Kind: SymbolRef,
			})
		case r.QueryID == defsID:
			if pkg == "" || !spec.exported(r.Match[0].Text) {
				continue
			}
			for _, q := range pkgs {
				ext.Symbols = append(ext.Symbols, FileSymbol{
					Name: q + "." + r.Match[0].Text,
					Pos:  r.Match[0].From,
					Kind: SymbolDef,
				})
			}
		case r.QueryID == methodID:
			if pkg == "" || !spec.exported(r.Match[1].Text) {
				continue
			}
			for _, q := range pkgs {
				ext.Symbols = append(ext.Symbols, FileSymbol{
					Name: q + "." + r.Match[0].Text + "." + r.Match[1].Text,
					Pos:  r.Match[1].From,
					Kind: SymbolMethodDef,
				})
			}
			// A bare Type.method (no module prefix) is the module-less
			// suffix nested-module specs also address.
			if spec.NestedModules {
				ext.Symbols = append(ext.Symbols, FileSymbol{
					Name: r.Match[0].Text + "." + r.Match[1].Text,
					Pos:  r.Match[1].From,
					Kind: SymbolMethodDef,
				})
			}
		case r.QueryID == reexportID:
			if pkg == "" || !isReexportFile || !spec.exported(r.Match[0].Text) {
				continue
			}
			for _, q := range pkgs {
				ext.Symbols = append(ext.Symbols, FileSymbol{
					Name: q + "." + r.Match[0].Text,
					Pos:  r.Match[0].From,
					Kind: SymbolDef,
				})
			}
		}
	}
	return ext, nil
}
