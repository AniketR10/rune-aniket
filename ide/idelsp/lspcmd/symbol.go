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

package lspcmd

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/handler/locationpicker"
	"unstable.build/go-tui/ide/idelsp/symbolresolve"
)

type symbolMatch = symbolresolve.Match

type symbolProgress = symbolresolve.Progress

func resolveSymbol(
	ctx context.Context, parser syntaxapi.Parser, name string,
	progress symbolProgress,
) ([]symbolMatch, error) {
	matches, err := symbolresolve.Resolve(ctx, parser, name, progress)
	if err != nil {
		if errors.Is(err, symbolresolve.ErrNoDot) {
			return nil, fmt.Errorf("no symbols found for %q", name)
		}
		return nil, err
	}
	return matches, nil
}

func resolveCommandSymbol(
	_ context.Context, cmd *textapi.Command,
	wm browserapi.WindowManager,
	fs workspaceapi.FileSystem,
	notify browserapi.Notifications,
	scheduleNextTick func(func()) bool,
	parser syntaxapi.Parser,
	onResolve func(m symbolMatch, done func()),
) (proceed bool, err error) {
	if len(cmd.Args) == 0 {
		if cmd.Resource == nil {
			return false, fmt.Errorf("no file open; pass a symbol name or open a file first")
		}
		return true, nil
	}
	name := strings.Join(cmd.Args, " ")

	id, _ := notify.Notify(browserapi.LevelInfo, "Resolving %s…", name)
	_ = notify.UpdateNotificationProgress(id, "", 0, 4)
	done := func() {
		_ = notify.UpdateNotificationProgress(id, "", 4, 4)
	}

	go debug.CapturePanicReport(func() {
		progress := symbolresolve.ProgressFunc(func(msg string, found int, step, total int64) {
			if found > 0 {
				msg = fmt.Sprintf("%s (%d found)", msg, found)
			}
			_ = notify.UpdateNotificationProgress(id, msg, step, total)
		})
		matches, err := resolveSymbol(context.Background(), parser, name, progress)
		scheduleNextTick(func() {
			if err != nil {
				done()
				_, _ = notify.Notify(browserapi.LevelError, "%s", err)
				return
			}
			if len(matches) == 1 {
				onResolve(matches[0], done)
				return
			}
			if err := showSymbolPicker(matches, wm, fs, scheduleNextTick, parser, onResolve, done); err != nil {
				done()
				_, _ = notify.Notify(browserapi.LevelError, "%s", err)
			}
		})
	})

	return false, nil
}

func showSymbolPicker(
	matches []symbolMatch,
	wm browserapi.WindowManager,
	fs workspaceapi.FileSystem,
	scheduleNextTick func(func()) bool,
	parser syntaxapi.Parser,
	onPick func(m symbolMatch, done func()),
	done func(),
) error {
	entries := make([]locationEntry, len(matches))
	for i, m := range matches {
		entries[i] = locationEntry{
			uri:     m.URI,
			rng:     semanticapi.Range{Start: m.Pos, End: m.Pos},
			display: m.Display,
		}
	}
	pickerEntries := make([]locationpicker.Entry, len(entries))
	for i, e := range entries {
		uri, err := LspToURI(e.uri)
		if err != nil {
			return err
		}
		pickerEntries[i] = locationpicker.Entry{URI: uri, Range: e.rng, Display: e.display}
	}
	picker := locationpicker.New(
		pickerEntries, wm, fs, scheduleNextTick, parser,
		locationpicker.DefaultConfig(), nil,
	)
	picker.SetOnSelect(func(idx int) {
		onPick(matches[idx], done)
	})
	win, err := wm.Floating(picker, browserapi.FloatingConfig{
		Alignment: component.AlignmentCentered,
	})
	if err != nil {
		return err
	}
	picker.SetWindow(win)
	return nil
}

func completeReferencedSymbol(
	ctx context.Context, parser syntaxapi.Parser,
) (iterator.Iterator[string], error) {
	ctx, cancel := context.WithCancel(ctx)
	ch := make(chan string)
	errc := make(chan error, 1)

	go debug.CapturePanicReport(func() {

		defer close(ch)
		if err := produceReferencedSymbols(ctx, parser, ch); err != nil {
			errc <- err
		}

	})

	seen := make(map[string]bool)
	return iterator.FromFunc(func(ctx context.Context) (string, bool, error) {
		for {
			select {
			case s, ok := <-ch:
				if !ok {
					select {
					case err := <-errc:
						return "", false, err
					default:
						return "", false, nil
					}
				}
				if seen[s] {
					continue
				}
				seen[s] = true
				return s, true, nil
			case <-ctx.Done():
				return "", false, ctx.Err()
			}
		}
	}, func() error {
		cancel()
		for range ch {
		}
		select {
		case err := <-errc:
			return err
		default:
			return nil
		}
	}), nil
}

func produceReferencedSymbols(
	ctx context.Context, parser syntaxapi.Parser, ch chan<- string,
) error {
	imports, err := reduceImports(ctx, parser)
	if err != nil {
		return err
	}

	if err := searchPairs(ctx, parser,
		`(qualified_type package: (package_identifier) @pkg name: (type_identifier) @type)`,
		[]string{"pkg", "type"}, "go",
		func(p [2]syntaxapi.Result) bool { return isExported(p[1].Text) },
		ch,
	); err != nil {
		return err
	}

	if err := searchPairs(ctx, parser,
		`(selector_expression operand: (identifier) @pkg field: (field_identifier) @symbol)`,
		[]string{"pkg", "symbol"}, "go",
		func(p [2]syntaxapi.Result) bool {
			if !isExported(p[1].Text) {
				return false
			}
			m := imports[p[0].File]
			return m != nil && m[p[0].Text]
		},
		ch,
	); err != nil {
		return err
	}

	packages, err := symbolresolve.FilePackages(ctx, parser)
	if err != nil {
		return err
	}
	return symbolresolve.SearchDefinitions(ctx, parser, packages, ch, isExported)
}

func searchPairs(
	ctx context.Context, parser syntaxapi.Parser,
	query string, captures []string, lang string,
	keep func([2]syntaxapi.Result) bool,
	ch chan<- string,
) error {
	iter, err := parser.Search(query, captures, lang)
	if err != nil {
		return err
	}
	results := iterator.Map(
		iterator.Filter(pairedResults(iter), keep),
		func(p [2]syntaxapi.Result) string {
			return p[0].Text + "." + p[1].Text
		},
	)
	defer func() { _ = results.Close() }()
	for {
		s, ok := results.Next(ctx)
		if !ok {
			return results.Err()
		}
		select {
		case ch <- s:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func reduceImports(
	ctx context.Context, parser syntaxapi.Parser,
) (map[workspaceapi.URI]map[string]bool, error) {
	type fileImports = map[workspaceapi.URI]map[string]bool

	pathIter, err := parser.Search(
		`(import_spec path: (interpreted_string_literal) @path)`,
		[]string{"path"}, "go",
	)
	if err != nil {
		return nil, err
	}
	imports, err := iterator.Reduce(ctx, pathIter,
		func(m fileImports, r syntaxapi.Result) (fileImports, error) {
			p := strings.Trim(r.Text, `"`)
			alias := path.Base(p)
			if alias == "." || alias == "_" {
				return m, nil
			}
			if m == nil {
				m = make(fileImports)
			}
			if m[r.File] == nil {
				m[r.File] = make(map[string]bool)
			}
			m[r.File][alias] = true
			return m, nil
		},
	)
	if err != nil {
		return nil, err
	}

	aliasIter, err := parser.Search(
		`(import_spec name: (package_identifier) @alias path: (interpreted_string_literal) @path)`,
		[]string{"alias", "path"}, "go",
	)
	if err != nil {
		return nil, err
	}
	_, err = iterator.Reduce(ctx, pairedResults(aliasIter),
		func(_ struct{}, p [2]syntaxapi.Result) (struct{}, error) {
			alias := p[0].Text
			if alias == "." || alias == "_" || imports == nil {
				return struct{}{}, nil
			}
			defaultAlias := path.Base(strings.Trim(p[1].Text, `"`))
			if fm := imports[p[0].File]; fm != nil {
				delete(fm, defaultAlias)
				fm[alias] = true
			}
			return struct{}{}, nil
		},
	)
	if err != nil {
		return nil, err
	}
	if imports == nil {
		imports = make(fileImports)
	}
	return imports, nil
}

func pairedResults(
	it iterator.Iterator[syntaxapi.Result],
) iterator.Iterator[[2]syntaxapi.Result] {
	// Buffer per-file because the gRPC stream may interleave results
	// from files processed concurrently.
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

func isExported(name string) bool {
	return len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z'
}

func normalizeMethodName(name string) string {
	if after, ok := strings.CutPrefix(name, "(*"); ok {
		if i := strings.Index(after, ")."); i != -1 {
			return after[:i] + "." + after[i+2:]
		}
	}
	if after, ok := strings.CutPrefix(name, "("); ok {
		if i := strings.Index(after, ")."); i != -1 {
			return after[:i] + "." + after[i+2:]
		}
	}
	return name
}
