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

// Package symbolresolve resolves package-qualified symbol names
// (e.g. "iterator.Iterator") to their declaration locations using
// tree-sitter queries against the workspace. Language-specific
// queries and rules live behind a per-language Spec.
package symbolresolve

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

// Match is a resolved symbol candidate. URI and Pos point at the
// declaration; Display is a human-readable label; ImportPath, when
// set, is the Go import path the symbol resolves through.
type Match struct {
	URI        string
	Pos        semanticapi.Position
	Display    string
	ImportPath string
}

// ErrNoDot is returned by Resolve when name does not contain a "."
// separating the package alias from the symbol name.
var ErrNoDot = errors.New("name does not contain a package separator")

// Progress receives resolution progress updates. A nil Progress is
// accepted by Resolve and treated as a no-op.
type Progress interface {
	// Report is called once per phase. msg describes the phase,
	// found is the running count of candidate matches, and step/total
	// describe progress as a fraction of total work.
	Report(msg string, found int, step, total int64)
}

// ProgressFunc adapts a function to the Progress interface.
type ProgressFunc func(msg string, found int, step, total int64)

// Report implements Progress by calling f.
func (f ProgressFunc) Report(msg string, found int, step, total int64) {
	f(msg, found, step, total)
}

// Resolve resolves a package-qualified symbol name (e.g.
// "iterator.Iterator") to one or more declaration locations using the
// given language spec. Results are deduplicated by file URI and
// collapsed by import path; when multiple distinct packages remain,
// each Match Display name is prefixed to disambiguate. progress, if
// non-nil, receives per-phase updates. Returns ErrNoDot when name
// contains no ".".
func Resolve(
	ctx context.Context, parser syntaxapi.Parser, spec *Spec, name string,
	progress Progress,
) ([]Match, error) {
	pkg, sym, hasDot := strings.Cut(name, ".")
	if !hasDot {
		return nil, ErrNoDot
	}

	report := func(msg string, found int, step, total int64) {
		if progress != nil {
			progress.Report(msg, found, step, total)
		}
	}

	seen := make(map[string]bool)
	var matches []Match
	add := func(uri string, pos semanticapi.Position) {
		if seen[uri] {
			return
		}
		seen[uri] = true
		matches = append(matches, Match{URI: uri, Pos: pos, Display: name})
	}

	collect := func(query string, captures []string) error {
		iter, err := parser.Search(query, captures, spec.LangID)
		if err != nil {
			return err
		}
		pairs := pairedResults(iter)
		defer func() { _ = pairs.Close() }()
		for {
			p, ok := pairs.Next(ctx)
			if !ok {
				return pairs.Err()
			}
			if p[0].Text != pkg || p[1].Text != sym {
				continue
			}
			add(p[0].File.String(), semanticapi.Position{
				Line:      uint32(p[1].From.Y),
				Character: uint32(p[1].From.X),
			})
		}
	}

	report("Searching references…", 0, 0, 4)
	for i, rq := range spec.RefQueries {
		if err := collect(rq.Query, rq.Captures); err != nil {
			return nil, err
		}
		report("Searching references…", len(matches), int64(i+1), 4)
	}

	report("Searching definitions…", len(matches), 2, 4)
	packages, err := FilePackages(ctx, parser, spec)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		if err := collectDefinitions(
			ctx, parser, spec, packages, pkg, sym, add,
		); err != nil {
			return nil, err
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no symbols found for %q", name)
	}
	if len(matches) > 1 && spec.ImportPathQuery != "" {
		report("Resolving imports…", len(matches), 3, 4)
		matches = deduplicateByImport(ctx, parser, spec, matches, pkg)
	}
	if len(matches) > 1 {
		disambiguateDisplayNames(spec, matches, name)
	}
	return matches, nil
}

// collectDefinitions streams workspace definitions matching sym in the
// target package and feeds them to add. Files are qualified via their
// package clause when the spec has one, otherwise via spec.Qualifier.
func collectDefinitions(
	ctx context.Context, parser syntaxapi.Parser, spec *Spec,
	packages map[workspaceapi.URI]string, pkg, sym string,
	add func(uri string, pos semanticapi.Position),
) error {
	files, err := definitionFiles(ctx, parser, spec, packages, pkg)
	if err != nil {
		return err
	}
	for _, fileURI := range files {
		it, err := parser.QueryNode(fileURI,
			syntaxapi.NodeCaptureDefinitionFunc|syntaxapi.NodeCaptureDefinitionType,
		)
		if err != nil {
			return err
		}
		for {
			r, ok := it.Next(ctx)
			if !ok {
				break
			}
			if r.Text != sym || !spec.exported(r.Text) {
				continue
			}
			add(fileURI.String(), semanticapi.Position{
				Line:      uint32(r.From.Y),
				Character: uint32(r.From.X),
			})
		}
		if err := it.Err(); err != nil {
			_ = it.Close()
			return err
		}
		_ = it.Close()
	}
	return nil
}

// definitionFiles returns the file URIs whose qualifier equals pkg. When
// the spec has a package clause it consults the packages map; otherwise
// it derives the qualifier from each file's URI via spec.Qualifier.
func definitionFiles(
	ctx context.Context, parser syntaxapi.Parser, spec *Spec,
	packages map[workspaceapi.URI]string, pkg string,
) ([]workspaceapi.URI, error) {
	if spec.hasPackages() {
		var files []workspaceapi.URI
		for fileURI, pkgName := range packages {
			if pkgName == pkg {
				files = append(files, fileURI)
			}
		}
		return files, nil
	}
	iter, err := parser.SearchNode(
		syntaxapi.NodeCaptureDefinitionFunc | syntaxapi.NodeCaptureDefinitionType,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = iter.Close() }()
	seen := make(map[workspaceapi.URI]bool)
	var files []workspaceapi.URI
	for {
		r, ok := iter.Next(ctx)
		if !ok {
			return files, iter.Err()
		}
		uri := r.File.String()
		if seen[r.File] || !spec.matchesFile(uri) || spec.qualifier(uri) != pkg {
			continue
		}
		seen[r.File] = true
		files = append(files, r.File)
	}
}

// FilePackages returns a map from file URI to the package name declared
// by its package clause, according to spec. Files without a parseable
// package clause are omitted. When the spec has no package clause it
// returns nil.
func FilePackages(
	ctx context.Context, parser syntaxapi.Parser, spec *Spec,
) (map[workspaceapi.URI]string, error) {
	if !spec.hasPackages() {
		return nil, nil
	}
	iter, err := parser.Search(
		spec.PackageClauseQuery, spec.PackageClauseCaptures, spec.LangID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = iter.Close() }()
	packages := make(map[workspaceapi.URI]string)
	for {
		r, ok := iter.Next(ctx)
		if !ok {
			if err := iter.Err(); err != nil {
				return nil, err
			}
			return packages, nil
		}
		if _, exists := packages[r.File]; !exists {
			packages[r.File] = r.Text
		}
	}
}

// SearchDefinitions streams package-qualified definition names
// ("pkg.Name") for every function and type defined in the workspace,
// according to spec. When the spec has a package clause, packages
// provides the file→package mapping and files absent from it are
// skipped; otherwise the qualifier is derived from each file's URI.
// Names rejected by the spec's export predicate or by keep are skipped.
// keep may be nil.
func SearchDefinitions(
	ctx context.Context, parser syntaxapi.Parser, spec *Spec,
	packages map[workspaceapi.URI]string,
	ch chan<- string, keep func(string) bool,
) error {
	iter, err := parser.SearchNode(
		syntaxapi.NodeCaptureDefinitionFunc | syntaxapi.NodeCaptureDefinitionType,
	)
	if err != nil {
		return err
	}
	defer func() { _ = iter.Close() }()
	for {
		r, ok := iter.Next(ctx)
		if !ok {
			return iter.Err()
		}
		if !spec.matchesFile(r.File.String()) {
			continue
		}
		var pkgName string
		if spec.hasPackages() {
			pkgName = packages[r.File]
		} else {
			pkgName = spec.qualifier(r.File.String())
		}
		if pkgName == "" {
			continue
		}
		if !spec.exported(r.Text) {
			continue
		}
		if keep != nil && !keep(r.Text) {
			continue
		}
		select {
		case ch <- pkgName + "." + r.Text:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// ImportedAliases returns, per file, the set of import aliases in scope
// according to spec. The completer uses it to keep only references whose
// package alias is actually imported. Returns an empty map when the spec
// declares no import queries.
func ImportedAliases(
	ctx context.Context, parser syntaxapi.Parser, spec *Spec,
) (map[workspaceapi.URI]map[string]bool, error) {
	result := make(map[workspaceapi.URI]map[string]bool)
	if spec.ImportPathQuery == "" {
		return result, nil
	}
	importPaths, err := resolveImportPaths(ctx, parser, spec)
	if err != nil {
		return nil, err
	}
	for file, aliases := range importPaths {
		set := make(map[string]bool, len(aliases))
		for alias := range aliases {
			set[alias] = true
		}
		result[file] = set
	}
	return result, nil
}

func disambiguateDisplayNames(spec *Spec, matches []Match, name string) {
	for i, m := range matches {
		prefix := m.ImportPath
		if prefix == "" {
			prefix = spec.displayPath(m.URI)
		}
		matches[i].Display = prefix + ": " + name
	}
}

func resolveImportPaths(
	ctx context.Context, parser syntaxapi.Parser, spec *Spec,
) (map[workspaceapi.URI]map[string]string, error) {
	type fileImports = map[workspaceapi.URI][]string
	type fileAliases = map[workspaceapi.URI]map[string]string

	pathIter, err := parser.Search(
		spec.ImportPathQuery, spec.ImportPathCaptures, spec.LangID,
	)
	if err != nil {
		return nil, err
	}
	imports, err := iterator.Reduce(ctx, pathIter,
		func(m fileImports, r syntaxapi.Result) (fileImports, error) {
			p := strings.Trim(r.Text, `"`)
			if m == nil {
				m = make(fileImports)
			}
			m[r.File] = append(m[r.File], p)
			return m, nil
		},
	)
	if err != nil {
		return nil, err
	}

	aliasIter, err := parser.Search(
		spec.ImportAliasQuery, spec.ImportAliasCaptures, spec.LangID,
	)
	if err != nil {
		return nil, err
	}
	explicitAliases, err := iterator.Reduce(ctx, pairedResults(aliasIter),
		func(m fileAliases, p [2]syntaxapi.Result) (fileAliases, error) {
			alias := p[0].Text
			if alias == "." || alias == "_" {
				return m, nil
			}
			importPath := strings.Trim(p[1].Text, `"`)
			if m == nil {
				m = make(fileAliases)
			}
			if m[p[0].File] == nil {
				m[p[0].File] = make(map[string]string)
			}
			m[p[0].File][importPath] = alias
			return m, nil
		},
	)
	if err != nil {
		return nil, err
	}
	resolved := make(map[workspaceapi.URI]map[string]string, len(imports))
	for file, paths := range imports {
		aliases := make(map[string]string)
		for _, importPath := range paths {
			alias := path.Base(importPath)
			if fileAliases := explicitAliases[file]; fileAliases != nil {
				if explicitAlias, ok := fileAliases[importPath]; ok {
					alias = explicitAlias
				}
			}
			if alias == "." || alias == "_" {
				continue
			}
			aliases[alias] = importPath
		}
		if len(aliases) > 0 {
			resolved[file] = aliases
		}
	}
	return resolved, nil
}

func deduplicateByImport(
	ctx context.Context, parser syntaxapi.Parser, spec *Spec,
	matches []Match, alias string,
) []Match {
	importPaths, err := resolveImportPaths(ctx, parser, spec)
	if err != nil {
		return matches
	}
	lookup := make(map[string]map[string]string, len(importPaths))
	for uri, aliases := range importPaths {
		lookup[uri.String()] = aliases
	}
	seen := make(map[string]bool)
	result := matches[:0]
	for _, m := range matches {
		key := m.URI
		if fileImports := lookup[m.URI]; fileImports != nil {
			if importPath, ok := fileImports[alias]; ok {
				key = importPath
				m.ImportPath = importPath
			}
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, m)
	}
	return result
}

func pairedResults(
	it iterator.Iterator[syntaxapi.Result],
) iterator.Iterator[[2]syntaxapi.Result] {
	// Buffer per-file because the gRPC stream may interleave results
	// from files processed concurrently, so consecutive results are
	// not guaranteed to belong to the same match.
	pending := make(map[workspaceapi.URI]syntaxapi.Result)
	return iterator.FromFunc(
		func(ctx context.Context) ([2]syntaxapi.Result, bool, error) {
			for {
				r, ok := it.Next(ctx)
				if !ok {
					return [2]syntaxapi.Result{}, false, it.Err()
				}
				if first, exists := pending[r.File]; exists {
					delete(pending, r.File)
					return [2]syntaxapi.Result{first, r}, true, nil
				}
				pending[r.File] = r
			}
		}, it.Close,
	)
}
