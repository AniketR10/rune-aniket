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


package ideshell

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/handlertest"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
)

const (
	testDocID  = "test-shell-history"
	testWidthH = 30
	testHeight = 12
)

func newTestHandler(t *testing.T, items []string) *Handler {
	return newTestHandlerFull(t, items, 100, nil)
}

func newTestHandlerFull(
	t *testing.T, items []string, maxHist int,
	register func(*CommandRegistry),
) *Handler {
	t.Helper()
	svc := storagestub.NewInMemoryService()
	if len(items) > 0 {
		require.NoError(t, svc.Create(
			context.Background(), testDocID,
			&historyDoc{Items: items, Version: 1},
		))
	}
	h, r := New(
		func(func()) bool { return false },
		term.NopInterrupter(),
		Config{
			Storage:           svc,
			HistoryDocumentID: testDocID,
			MaxHistory:        maxHist,
		},
	)
	if register != nil {
		register(r)
	}
	t.Cleanup(func() { _ = h.Close() })
	return h
}

// stubCmd is a no-op CommandHandler used for command-name completion
// tests. Its Complete method returns no candidates so the registry
// drives completion entirely by name matching.
type stubCmd struct{}

func (stubCmd) HandleCommand(
	context.Context, repl.Command, repl.ProgressWriter,
) (iterator.Iterator[component.Responsive], error) {
	return iterator.Empty[component.Responsive](), nil
}

func (stubCmd) Complete(
	context.Context, string, []string,
) (iterator.Iterator[string], error) {
	return iterator.Empty[string](), nil
}

func (stubCmd) Help(
	context.Context, []string,
) (iterator.Iterator[component.Responsive], error) {
	return iterator.Empty[component.Responsive](), nil
}

// argCmd returns a fixed list of candidates for any argument
// completion request. It is used to exercise tab completion of
// command arguments (as opposed to command names).
type argCmd struct{ candidates []string }

func (argCmd) HandleCommand(
	context.Context, repl.Command, repl.ProgressWriter,
) (iterator.Iterator[component.Responsive], error) {
	return iterator.Empty[component.Responsive](), nil
}

func (a argCmd) Complete(
	context.Context, string, []string,
) (iterator.Iterator[string], error) {
	return iterator.FromSlice(a.candidates), nil
}

func (argCmd) Help(
	context.Context, []string,
) (iterator.Iterator[component.Responsive], error) {
	return iterator.Empty[component.Responsive](), nil
}

// Common command registration fixtures used by the TestHandler cases
// below. Defined as variables so each table-driven case can refer to
// them without re-declaring three identical handlers.
var (
	// threeFoos registers three command names that all share a "fo"
	// prefix, used to drive multi-candidate command-name completion.
	threeFoos = func(r *CommandRegistry) {
		r.Register("foo", "do foo", stubCmd{})
		r.Register("foobar", "do foobar", stubCmd{})
		r.Register("foobaz", "do foobaz", stubCmd{})
	}
	// twoArgs registers a single command "g" whose argument completer
	// returns two distinct candidates with no shared prefix.
	twoArgs = func(r *CommandRegistry) {
		r.Register("g", "", argCmd{candidates: []string{"alpha", "beta"}})
	}
	// twoAls registers a command whose argument candidates share a
	// non-trivial prefix, so partial-prefix replacement can be tested.
	twoAls = func(r *CommandRegistry) {
		r.Register("g", "", argCmd{candidates: []string{"alpha", "alright"}})
	}
	// singleAlpha registers a single command name so that tab triggers
	// the inline single-candidate fast path rather than the overlay.
	singleAlpha = func(r *CommandRegistry) {
		r.Register("alpha", "", stubCmd{})
	}
)

// longX returns n 'x' characters, sized to drive multi-line wrap
// scenarios.
func longX(n int) string { return strings.Repeat("x", n) }

// manyEntries returns n synthetic history entries used to exercise
// the overlay's row cap.
func manyEntries(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("entry-%02d", i)
	}
	return out
}

// TestHandler exercises the IDE shell handler's reverse-history search
// and tab-completion overlays end-to-end via
// handlertest.RunHandlerSequence so each case asserts against the
// literal terminal output. The fixtures share a common Handler shape:
// optional persisted history items, an optional MaxHistory cap, and an
// optional command-registry registration callback for completion
// scenarios.
type handlerCase struct {
	name     string
	items    []string
	maxHist  int
	register func(*CommandRegistry)
	sequence string
	expected string
}

func handlerTestCases() []handlerCase {
	return []handlerCase{{
		name:     "initial draw shows shell prompt",
		items:    []string{"alpha", "beta", "charlie"},
		sequence: "",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
> ▐                           `,
	}, {
		name:     "<c-r> opens overlay listing newest history first",
		items:    []string{"alpha", "beta", "charlie"},
		sequence: "<c-r>",
		expected: `                              
                              
                              
                              
                              
                              
                              
  charlie                     
  beta                        
  alpha                       
                              
> ▐                        3/3`,
	}, {
		name:     "typed query narrows matches",
		items:    []string{"git status", "ls -la", "make test"},
		sequence: "<c-r>make",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
                              
  make test                   
                              
> make▐                    1/3`,
	}, {
		name:     "<c-r> seeds query from inputbox text",
		items:    []string{"alpha", "beta", "charlie"},
		sequence: "bet<c-r>",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
                              
  beta                        
                              
> bet▐                     1/3`,
	}, {
		name:     "<backspace> shrinks query",
		items:    []string{"abc", "abd"},
		sequence: "<c-r>abc<backspace>",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
  abd                         
  abc                         
                              
> ab▐                      2/2`,
	}, {
		name:     "<backspace> on empty query is a noop",
		items:    []string{"alpha"},
		sequence: "<c-r><backspace>",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
                              
  alpha                       
                              
> ▐                        1/1`,
	}, {
		name:     "<space> is included in the query",
		items:    []string{"git log", "git status"},
		sequence: "<c-r>git<space>l",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
                              
  git log                     
                              
> git l▐                   1/2`,
	}, {
		name:     "uppercase query characters narrow matches",
		items:    []string{"Alpha", "alpha"},
		sequence: "<c-r>A",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
  alpha                       
  Alpha                       
                              
> A▐                       2/2`,
	}, {
		name:     "query that matches nothing shows 0 matches",
		items:    []string{"alpha", "beta"},
		sequence: "<c-r>zz",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
> zz▐                      0/2`,
	}, {
		name:     "<enter> accepts focused entry into prompt",
		items:    []string{"alpha", "beta", "charlie"},
		sequence: "<c-r><enter>",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
> charlie▐                    `,
	}, {
		name:     "<tab> also accepts focused entry into prompt",
		items:    []string{"echo hi", "ls -la"},
		sequence: "<c-r><tab>",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
> ls -la▐                     `,
	}, {
		name:     "<c-j> moves focus down before <enter> accepts",
		items:    []string{"one", "two", "three"},
		sequence: "<c-r><c-j><enter>",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
> two▐                        `,
	}, {
		name:     "<c-k> moves focus back up before <enter> accepts",
		items:    []string{"one", "two", "three"},
		sequence: "<c-r><c-j><c-j><c-k><enter>",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
> two▐                        `,
	}, {
		name:     "<down> and <up> also navigate the overlay",
		items:    []string{"one", "two", "three"},
		sequence: "<c-r><down><down><up><enter>",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
> two▐                        `,
	}, {
		name:     "repeated <c-r> moves focus down within overlay",
		items:    []string{"one", "two", "three"},
		sequence: "<c-r><c-r><enter>",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
> two▐                        `,
	}, {
		name: "accepting clears any pre-existing prompt text " +
			"before inserting entry",
		items:    []string{"hello-entry"},
		sequence: "hello<c-r><enter>",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
> hello-entry▐                `,
	}, {
		name:     "accepting with no matches leaves prompt untouched",
		items:    []string{"alpha"},
		sequence: "pq<c-r>z<enter>",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
> pq▐                         `,
	}, {
		name:     "<esc> cancels overlay and restores prompt",
		items:    []string{"echo hi"},
		sequence: "<c-r><esc>",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
> ▐                           `,
	}, {
		name:     "<c-g> cancels overlay and restores prompt",
		items:    []string{"echo hi"},
		sequence: "<c-r><c-g>",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
> ▐                           `,
	}, {
		name:     "<c-c> cancels overlay and restores prompt",
		items:    []string{"echo"},
		sequence: "<c-r><c-c>",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
> ▐                           `,
	}, {
		name:     "cancelling preserves any pre-existing prompt text",
		items:    []string{"alpha"},
		sequence: "hi<c-r><esc>",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
> hi▐                         `,
	}, {
		name: "characters typed after cancel reach the prompt, " +
			"not the closed overlay",
		items:    []string{"ls -la"},
		sequence: "<c-r><esc>ab",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
> ab▐                         `,
	}, {
		name: "characters typed inside the overlay do not reach the " +
			"underlying prompt",
		items:    []string{"alpha"},
		sequence: "<c-r>q<esc>",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
> ▐                           `,
	}, {
		name: "re-opening overlay after an accept seeds the " +
			"query with the accepted entry",
		items:    []string{"alpha", "beta", "charlie"},
		sequence: "<c-r><enter><c-r>",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
                              
  charlie                     
                              
> charlie▐                 1/3`,
	}, {
		name:     "without storage <c-r> opens an empty overlay",
		items:    nil,
		sequence: "<c-r>",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
> ▐                        0/0`,
	}, {
		name: "typing a query with empty history shows " +
			"0 of 0 matches and no entries",
		items:    nil,
		sequence: "<c-r>x",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
> x▐                       0/0`,
	}, {
		name: "MaxHistory truncates the oldest entries when " +
			"loading the overlay",
		items:    []string{"old", "mid", "new"},
		maxHist:  2,
		sequence: "<c-r>",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
  new                         
  mid                         
                              
> ▐                        2/2`,
	}, {
		name: "<enter> on a focused completion candidate replaces " +
			"only the completed word",
		register: threeFoos,
		sequence: "fo<tab>b<enter>",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
> foobar▐                     `,
	}, {
		name: "<tab> in the completion overlay also accepts the " +
			"focused candidate",
		register: threeFoos,
		sequence: "fo<tab><tab>",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
> foo▐                        `,
	}, {
		name: "<esc> cancels the completion overlay and leaves the " +
			"prompt text untouched",
		register: threeFoos,
		sequence: "fo<tab><esc>",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
> fo▐                         `,
	}, {
		name:     "<c-g> also cancels the completion overlay",
		register: threeFoos,
		sequence: "fo<tab><c-g>",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
> fo▐                         `,
	}, {
		name: "single completion candidate is applied inline " +
			"without opening the overlay",
		register: singleAlpha,
		sequence: "al<tab>",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
> alpha▐                      `,
	}, {
		name:     "<tab> with no completion candidates is a no-op",
		register: threeFoos,
		sequence: "zz<tab>",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
> zz▐                         `,
	}, {
		name: "<tab> after a space opens an argument completion " +
			"overlay",
		register: twoArgs,
		sequence: "g<space><tab>",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
  alpha                       
  beta                        
                              
> g ▐                         `,
	}, {
		name: "accepting an argument completion candidate appends " +
			"it after the existing line",
		register: twoArgs,
		sequence: "g<space><tab><enter>",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
> g alpha▐                    `,
	}, {
		name: "<tab> on a partially-typed argument shows matching " +
			"candidates",
		register: twoAls,
		sequence: "g<space>al<tab>",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
  alpha                       
  alright                     
                              
> g al▐                       `,
	}, {
		name: "accepting a partial argument completion replaces " +
			"only the partial word",
		register: twoAls,
		sequence: "g<space>al<tab><enter>",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
> g alpha▐                    `,
	}, {
		name: "<c-j> moves focus down before <enter> accepts in " +
			"the completion overlay",
		register: threeFoos,
		sequence: "fo<tab><c-j><enter>",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
> foobar▐                     `,
	}, {
		name: "<c-r> still opens history search even when the " +
			"prompt already has typed text",
		register: threeFoos,
		sequence: "fo<c-r>",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
> fo▐                      0/0`,
	}, {
		// Width is 30 and the prompt is "> ", so each input
		// line holds 28 runes. 90 'x' wraps to 4 lines.
		name: "input that wraps over multiple lines without overlay",
		sequence: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx" +
			"xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
> xxxxxxxxxxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
xx▐                           `,
	}, {
		name: "opening overlay with empty history pushes a " +
			"multi-line input upward to make room",
		sequence: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx" +
			"xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx<c-r>",
		expected: `                              
                              
                              
                              
                              
> xxxxxxxxxxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                              
                              
                              
>                             `,
	}, {
		name: "opening overlay with history above a multi-line " +
			"input keeps both the input and the candidates visible",
		items: []string{"alpha", "beta", "charlie"},
		sequence: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx" +
			"xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx<c-r>",
		expected: `                              
                              
                              
                              
                              
> xxxxxxxxxxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                              
                              
                              
>                             `,
	}, {
		name: "cancelling the overlay restores the original " +
			"multi-line input layout",
		sequence: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx" +
			"xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx<c-r><esc>",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
> xxxxxxxxxxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
xx▐                           `,
	}, {
		// 30 entries — the overlay caps the visible list at
		// searchOverlayMaxRows (10), reserving the top of the
		// screen for the prompt.
		name:     "history overlay caps visible list at 10 rows",
		items:    manyEntries(30),
		sequence: "<c-r>",
		expected: `                              
  entry-29                    
  entry-28                    
  entry-27                    
  entry-26                    
  entry-25                    
  entry-24                    
  entry-23                    
  entry-22                    
  entry-21                    
  entry-20                    
> ▐                      30/30`,
	}, {
		// Input that is longer than the entire viewport. The
		// inner repl scrolls/truncates so only the bottom of the
		// input is visible — the overlay still appears at the
		// bottom and pushes the visible portion up.
		name: "input larger than the screen is truncated to keep " +
			"the prompt edge visible",
		sequence: longX(300),
		expected: `                              
> xxxxxxxxxxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
xx▐                           `,
	}, {
		name: "opening the overlay over an oversized input " +
			"truncates the input from the top to fit the candidates",
		items:    []string{"alpha"},
		sequence: longX(300) + "<c-r>",
		expected: `xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                              
                              
                              
>                             `,
	}, {
		// Argument completion with a wrapped first line — the
		// overlay still appears at the bottom and the partial
		// input remains visible.
		name: "argument completion overlay appears below a wrapped " +
			"first line of input",
		register: twoArgs,
		sequence: "g<space>" +
			"xxxxxxxxxxxxxxxxxxxxxxxxxx" +
			"<tab>",
		expected: `                              
                              
                              
                              
                              
                              
                              
  alpha                       
  beta                        
                              
> g xxxxxxxxxxxxxxxxxxxxxxxxxx
▐                             `,
	}, {
		// Bug 1: a partial word typed before <tab> stayed in the
		// inputbox; opening completion must keep it visible.
		name: "completion overlay keeps the partial word in the " +
			"inputbox",
		register: threeFoos,
		sequence: "fo<tab>",
		// the inputbox should still show "> fo" with the cursor
		// after it; the candidate rows render above.
		expected: `                              
                              
                              
                              
                              
                              
                              
  foo                         
  foobar                      
  foobaz                      
                              
> fo▐                         `,
	}, {
		// Bug 1 (continued): typing more chars after <tab>
		// extends the inputbox AND narrows the list.
		name: "typing in the completion overlay extends the " +
			"inputbox word and narrows the list",
		register: threeFoos,
		sequence: "fo<tab>b",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
  foobar                      
  foobaz                      
                              
> fob▐                        `,
	}, {
		// Bug 2: completing the first arg, then space, then tab
		// must not erase the first arg.
		name: "completing arg then typing space then tab keeps the " +
			"prior args visible",
		register: func(r *CommandRegistry) {
			r.Register("g", "", argCmd{
				candidates: []string{"alpha", "beta"},
			})
		},
		sequence: "g<space><tab><tab><space><tab>",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
  alpha                       
  beta                        
                              
> g alpha ▐                   `,
	}, {
		// Bug 2 (continued): the list filter for the new tab is
		// driven only by the partial word at the cursor, not by
		// the previous arg.
		name: "argument completion filter ignores prior accepted " +
			"arguments",
		register: func(r *CommandRegistry) {
			r.Register("g", "", argCmd{
				candidates: []string{"alpha", "beta"},
			})
		},
		sequence: "g<space><tab><tab><space><tab>b",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
                              
  beta                        
                              
> g alpha b▐                  `,
	}, {
		// Backspace to delete a typed-after-tab character keeps
		// the inputbox in sync.
		name: "backspace in completion overlay shrinks the " +
			"inputbox word",
		register: threeFoos,
		sequence: "fo<tab>b<backspace>",
		expected: `                              
                              
                              
                              
                              
                              
                              
  foo                         
  foobar                      
  foobaz                      
                              
> fo▐                         `,
	}, {
		// Pressing space inside the completion overlay closes
		// the overlay and forwards the space to the inputbox
		// (since the user is starting a new argument).
		name: "space inside the completion overlay closes it and " +
			"reaches the inputbox",
		register: threeFoos,
		sequence: "fo<tab><space>",
		expected: `                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
> fo ▐                        `,
	}, {
		// Regression: <c-r> over a wrapped multi-line input
		// should keep the wrapped input visible above the
		// candidate band, the same way <tab> does. The search
		// bar with the user's query lives on the bottom row.
		name: "history overlay does not overdraw a tall " +
			"wrapped input",
		items: []string{"alpha", "beta", "charlie"},
		sequence: "g<space>" +
			longX(28*3) +
			"<c-r>",
		expected: `                              
                              
                              
                              
                              
                              
> g xxxxxxxxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                              
                              
                              
>                             `,
	}, {
		// Regression: opening completion when the prompt line
		// has wrapped over several rows must not overdraw the
		// existing input. The candidate band lands below the
		// (now-shrunk) inputbox and the wrapped tail of the
		// input is still visible.
		name: "completion overlay does not overdraw a tall " +
			"wrapped input",
		register: twoArgs,
		sequence: "g<space>" +
			longX(28*3) +
			"<tab>",
		expected: `                              
                              
                              
                              
                              
                              
  alpha                       
  beta                        
                              
> g xxxxxxxxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxxxxxxxxxx▐ `,
	}}
}

func TestHandler(t *testing.T) {
	for _, tc := range handlerTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			maxHist := tc.maxHist
			if maxHist == 0 {
				maxHist = 100
			}
			h := newTestHandlerFull(t, tc.items, maxHist, tc.register)
			handlertest.RunHandlerSequence(t, h, testWidthH, testHeight,
				[]handlertest.SequenceTestCase{{
					InputSequence: tc.sequence,
					Expected:      tc.expected,
				}})
		})
	}
}

// TestHandlerLoadHistoryReturnsNewestFirst covers the non-UI helper
// that seeds the overlay from persisted storage.
func TestHandlerLoadHistoryReturnsNewestFirst(t *testing.T) {
	h := newTestHandler(t, []string{"oldest", "middle", "newest"})
	assert.Equal(t,
		[]string{"newest", "middle", "oldest"}, h.loadHistory(),
	)
}
