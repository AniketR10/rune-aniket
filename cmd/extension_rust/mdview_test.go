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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
)

func TestFencedCode(t *testing.T) {
	tests := []struct {
		name string
		text string
		lang string
		want string
	}{
		{
			name: "plain",
			text: "FN@0..12",
			want: "```\nFN@0..12\n```",
		},
		{
			name: "tagged",
			text: "fn main() {}",
			lang: "rust",
			want: "```rust\nfn main() {}\n```",
		},
		{
			name: "trailing newlines are dropped",
			text: "digraph {}\n\n",
			lang: "dot",
			want: "```dot\ndigraph {}\n```",
		},
		{
			name: "fence outgrows the longest backtick run",
			text: "before\n```\nnested\n```\nafter",
			want: "````\nbefore\n```\nnested\n```\nafter\n````",
		},
		{
			name: "short backtick runs still use three",
			text: "let x = `y`;",
			want: "```\nlet x = `y`;\n```",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, fencedCode(tt.text, tt.lang))
		})
	}
}

// The viewer keeps the rendered source and floats itself so the command
// layer never has to reach into the markdown stack.
func TestShowMarkdownFloatsSource(t *testing.T) {
	wm := &fakeWM{}
	require.NoError(t, showMarkdown(wm, nil, nil, "# Title\n\nbody"))
	wm.mu.Lock()
	defer wm.mu.Unlock()
	view, ok := wm.floating.(*markdownView)
	require.True(t, ok, "a markdown view is floated")
	assert.Equal(t, "# Title\n\nbody", view.source)
}

// A rust-fenced view must hand its code block to the parser for
// highlighting; the deferred highlight pass lands through the view's
// tick, so a recording parser observes the request.
func TestShowMarkdownHighlightsFencedCode(t *testing.T) {
	wm := &fakeWM{}
	parser := &recordingParser{}
	require.NoError(t, showMarkdown(wm, parser, nil, fencedCode("fn main() {}", "rust")))
	requireHighlightRequested(t, parser, ".rs")
}

// spanParser is a recordingParser whose highlight pass returns one
// span, so the markdown code block schedules its deferred apply (an
// empty result short-circuits before the tick).
type spanParser struct {
	recordingParser
}

func (p *spanParser) Highlight(
	uri workspaceapi.URI, code string,
) (iterator.Iterator[textapi.Location], error) {
	_, _ = p.recordingParser.Highlight(uri, code)
	return iterator.FromSlice([]textapi.Location{{
		From: term.Coordinates{X: 0, Y: 0},
		To:   term.Coordinates{X: 2, Y: 0},
	}}), nil
}

// The extension has no event loop: after the deferred highlight pass
// lands, the view must interrupt the IDE so the repaint is not deferred
// to the next key press or mouse move.
func TestShowMarkdownInterruptsAfterHighlight(t *testing.T) {
	wm := &fakeWM{}
	parser := &spanParser{}
	ir := &recordingInterrupter{}
	require.NoError(t, showMarkdown(wm, parser, ir, fencedCode("fn main() {}", "rust")))
	requireHighlightRequested(t, &parser.recordingParser, ".rs")
	requireInterrupted(t, ir)
}

// requireInterrupted waits for the view to request an IDE redraw.
func requireInterrupted(t *testing.T, ir *recordingInterrupter) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if ir.interrupts() > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("no redraw interrupt after the highlight pass")
}

// requireHighlightRequested waits for the asynchronous highlight pass
// to request highlighting for a URI containing substr.
func requireHighlightRequested(t *testing.T, parser *recordingParser, substr string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, uri := range parser.highlighted() {
			if strings.Contains(uri, substr) {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no highlight request for %q; got %v", substr, parser.highlighted())
}
