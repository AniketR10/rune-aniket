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
)

// matchPosition converts a resolved symbol's term coordinates into an LSP
// position (0-based line and character).
func matchPosition(m syntaxapi.Match) semanticapi.Position {
	return semanticapi.Position{
		Line:      uint32(m.Pos.Y),
		Character: uint32(m.Pos.X),
	}
}

func resolveSymbol(
	ctx context.Context, parser syntaxapi.Parser, name string,
	progress syntaxapi.Progress,
) ([]syntaxapi.Match, error) {
	it, err := parser.ResolveSymbol(ctx, name, progress)
	if err != nil {
		if errors.Is(err, syntaxapi.ErrNoDot) {
			return nil, errUnqualifiedSymbol(name)
		}
		return nil, err
	}
	matches, err := iterator.ToSlice(ctx, it)
	if err != nil {
		if errors.Is(err, syntaxapi.ErrNoDot) {
			return nil, errUnqualifiedSymbol(name)
		}
		return nil, err
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no symbols found for %q", name)
	}
	return matches, nil
}

// errUnqualifiedSymbol explains that a symbol name must be qualified with
// its package or module, since resolution keys off the qualifier separator.
func errUnqualifiedSymbol(name string) error {
	return fmt.Errorf(
		"%q is not qualified; prefix it with its package or module", name)
}

func resolveCommandSymbol(
	_ context.Context, cmd *textapi.Command,
	wm browserapi.WindowManager,
	fs workspaceapi.FileSystem,
	notify browserapi.Notifications,
	scheduleNextTick func(func()) bool,
	parser syntaxapi.Parser,
	onResolve func(m syntaxapi.Match, done func()),
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
		progress := syntaxapi.ProgressFunc(func(msg string, found int, step, total int64) {
			if found > 0 {
				msg = fmt.Sprintf("%s (%d found)", msg, found)
			}
			scheduleNextTick(func() {
				_ = notify.UpdateNotificationProgress(id, msg, step, total)
			})
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
	matches []syntaxapi.Match,
	wm browserapi.WindowManager,
	fs workspaceapi.FileSystem,
	scheduleNextTick func(func()) bool,
	parser syntaxapi.Parser,
	onPick func(m syntaxapi.Match, done func()),
	done func(),
) error {
	entries := make([]locationEntry, len(matches))
	for i, m := range matches {
		entries[i] = locationEntry{
			uri:     m.URI,
			rng:     semanticapi.Range{Start: matchPosition(m), End: matchPosition(m)},
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
	it, err := parser.ListReferencedSymbols(ctx)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	return iterator.FromFunc(func(ctx context.Context) (string, bool, error) {
		for {
			s, ok := it.Next(ctx)
			if !ok {
				return "", false, it.Err()
			}
			if seen[s] {
				continue
			}
			seen[s] = true
			return s, true, nil
		}
	}, it.Close), nil
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
