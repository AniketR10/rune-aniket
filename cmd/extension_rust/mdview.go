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

package main

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler"
	"github.com/unstablebuild/rune-go-sdk/term"
	mdcomp "unstable.build/go-tui/component/markdown"
	mdhandler "unstable.build/go-tui/handler/markdown"
)

// markdownView is the floating viewer used for rust-analyzer's textual
// output. The markdown stack gives it less-like scrolling, mouse wheel
// support, `/` search, and selection; source is retained so tests can
// assert on what is displayed. Its mutex stands in for the event loop
// the extension does not have: handler callbacks arrive on the window
// stream goroutine while asynchronous code-block highlighting lands
// via tick, so both run under the same lock.
type markdownView struct {
	mu        sync.Mutex
	inner     browserapi.Floating
	source    string
	interrupt term.Interrupter
}

var _ browserapi.Floating = (*markdownView)(nil)

// tick runs deferred highlight work under the view lock, then wakes the
// IDE event loop: the extension has no loop of its own, so without the
// interrupt the new highlights would render only on the next input
// event.
func (v *markdownView) tick(fn func()) bool {
	v.mu.Lock()
	fn()
	v.mu.Unlock()
	_ = v.interrupt.Interrupt(context.Background())
	return true
}

func (v *markdownView) Handle(ev term.Event) (exit, handled bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.inner.Handle(ev)
}

func (v *markdownView) Draw(w term.Writer) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.inner.Draw(w)
}

func (v *markdownView) Resize(w, h int) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.inner.Resize(w, h)
}

func (v *markdownView) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.inner.Cursor()
}

func (v *markdownView) Selection() (string, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.inner.Selection()
}

func (v *markdownView) Dimensions() (int, int) {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.inner.Dimensions()
}

func (v *markdownView) Close() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.inner.Close()
}

// showMarkdown floats a scrollable, dismiss-on-key markdown view of
// source, with fenced code blocks highlighted through parser. interrupt
// wakes the IDE event loop when the deferred highlight pass lands.
func showMarkdown(
	wm browserapi.WindowManager, parser syntaxapi.Parser,
	interrupt term.Interrupter, source string,
) error {
	if interrupt == nil {
		interrupt = term.NopInterrupter()
	}
	view := &markdownView{source: source, interrupt: interrupt}
	cfg := mdcomp.DefaultConfig()
	cfg.Parser = parser
	cfg.ScheduleNextTick = view.tick
	comp, err := mdcomp.NewWithConfig(source, cfg)
	if err != nil {
		return fmt.Errorf("render markdown: %w", err)
	}
	mdh := mdhandler.New(comp)
	span := handler.NewSpan(mdh, component.SpanConfig{
		PadHorizontal:    2,
		ContentAlignment: component.AlignmentCentered,
	})
	var win browserapi.Window
	view.inner = browserapi.FuncFloatingHandler(span, func() error {
		defer wm.CloseWindow(win) //nolint:errcheck
		return mdh.Close()
	})
	w, err := wm.Floating(view, browserapi.FloatingConfig{
		Alignment: component.AlignmentCentered,
	})
	if err != nil {
		return fmt.Errorf("show viewer: %w", err)
	}
	// Close (on the stream goroutine) reads win under the view lock.
	view.mu.Lock()
	win = w
	view.mu.Unlock()
	return nil
}

// fencedCode wraps plain server output in a fenced code block tagged
// with lang, so the markdown viewer renders it verbatim. The fence is
// longer than any backtick run in text, which is what keeps content like
// a macro expansion containing ``` from closing the block early.
func fencedCode(text, lang string) string {
	longest, run := 0, 0
	for _, r := range text {
		if r != '`' {
			run = 0
			continue
		}
		run++
		longest = max(longest, run)
	}
	fence := strings.Repeat("`", max(longest+1, 3))
	return fence + lang + "\n" + strings.TrimRight(text, "\n") + "\n" + fence
}
