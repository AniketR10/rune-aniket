// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.
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

package extension

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"unicode/utf8"

	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/handlertest"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/cmd/rune-agent/dialogue/dialoguetui"
)

var _ dialoguetui.CommandHandler = (*commandAdapter)(nil)

// countingInterrupter records Interrupt calls so tests can
// verify that schedule triggers redraws.
type countingInterrupter struct {
	count atomic.Int64
}

func (c *countingInterrupter) Interrupt(context.Context) error {
	c.count.Add(1)
	return nil
}

// echoHandler is a minimal repl.CommandHandler that returns
// the command name as output. This exercises the full
// schedule→drain→draw path without needing the real agentshell.
type echoHandler struct{}

func (echoHandler) HandleCommand(_ context.Context, cmd repl.Command, _ repl.ProgressWriter) (
	iterator.Iterator[component.Responsive], error,
) {
	r := component.NewResponsiveString(cmd.Name, component.StringResponsiveConfig{})
	return iterator.FromSlice([]component.Responsive{r}), nil
}

func (echoHandler) Complete(context.Context, string, []string) (
	iterator.Iterator[string], error,
) {
	return iterator.Empty[string](), nil
}

// schedulingFlusher wraps schedulingHandler for use with
// handlertest.RunHandlerSequence. It calls inner.Wait()
// after Handle so that async command goroutines have
// finished scheduling their callbacks before Draw drains them.
type schedulingFlusher struct {
	sh *schedulingHandler
}

func (f *schedulingFlusher) Handle(ev term.Event) (exit, handled bool) {
	exit, handled = f.sh.Handle(ev)
	f.sh.inner.Wait()
	return
}

func (f *schedulingFlusher) Resize(w, h int)    { f.sh.Resize(w, h) }
func (f *schedulingFlusher) Draw(w term.Writer) { f.sh.Draw(w) }
func (f *schedulingFlusher) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return f.sh.Cursor()
}
func (f *schedulingFlusher) Selection() (string, bool) { return f.sh.Selection() }

func newSchedulingFlusher(interrupter term.Interrupter) *schedulingFlusher {
	sh := &schedulingHandler{interrupter: interrupter, ctx: context.Background()}
	replHandler := repl.New(echoHandler{}, sh.schedule, interrupter,
		repl.WithPrompt("> "),
	)
	sh.inner = replHandler
	return &schedulingFlusher{sh: sh}
}

const (
	shTestWidth  = 40
	shTestHeight = 10
)

func shPad(s string) string {
	n := utf8.RuneCountInString(s)
	if n >= shTestWidth {
		return s
	}
	return s + strings.Repeat(" ", shTestWidth-n)
}

func shExpected(numBlank int, lines ...string) string {
	all := make([]string, 0, numBlank+len(lines))
	bl := strings.Repeat(" ", shTestWidth)
	for range numBlank {
		all = append(all, bl)
	}
	for _, l := range lines {
		all = append(all, shPad(l))
	}
	return strings.Join(all, "\n")
}

func TestSchedulingHandler_initial_prompt(t *testing.T) {
	ci := &countingInterrupter{}
	f := newSchedulingFlusher(ci)
	handlertest.RunHandlerSequence(t, f, shTestWidth, shTestHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "",
			Expected:      shExpected(9, "> \u2590"),
		},
	})
}

func TestSchedulingHandler_command_output(t *testing.T) {
	ci := &countingInterrupter{}
	f := newSchedulingFlusher(ci)
	handlertest.RunHandlerSequence(t, f, shTestWidth, shTestHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "hello<enter>",
			Expected: shExpected(7,
				"> hello",
				"hello",
				"> \u2590",
			),
		},
	})
	if ci.count.Load() == 0 {
		t.Fatal("expected at least one interrupt from schedule")
	}
}

func TestSchedulingHandler_multiple_commands(t *testing.T) {
	ci := &countingInterrupter{}
	f := newSchedulingFlusher(ci)
	handlertest.RunHandlerSequence(t, f, shTestWidth, shTestHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "first<enter>",
			Expected: shExpected(7,
				"> first",
				"first",
				"> \u2590",
			),
		},
		{
			InputSequence: "second<enter>",
			Expected: shExpected(5,
				"> first",
				"first",
				"> second",
				"second",
				"> \u2590",
			),
		},
	})
	if ci.count.Load() < 2 {
		t.Fatalf("expected at least 2 interrupts, got %d", ci.count.Load())
	}
}

func TestSchedulingHandler_concurrent_schedule(t *testing.T) {
	ci := &countingInterrupter{}
	sh := &schedulingHandler{
		interrupter: ci,
		ctx:         context.Background(),
	}

	const n = 100
	var counter atomic.Int64
	var wg sync.WaitGroup
	wg.Add(n)

	for range n {
		go func() {
			defer wg.Done()
			sh.schedule(func() { counter.Add(1) })
		}()
	}
	wg.Wait()

	sh.drainPending()
	if counter.Load() != n {
		t.Fatalf("expected %d callbacks executed, got %d", n, counter.Load())
	}
	if ci.count.Load() != n {
		t.Fatalf("expected %d interrupts, got %d", n, ci.count.Load())
	}
}
