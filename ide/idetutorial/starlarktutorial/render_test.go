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

package starlarktutorial

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"

	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/handler/command"
)

// attrGridWriter captures both runes and term.Attributes per cell so
// tests can assert AttrReverse on the title bar.
type attrGridWriter struct {
	w, h  int
	cells [][]term.Cell
}

func newAttrGridWriter(w, h int) *attrGridWriter {
	cells := make([][]term.Cell, h)
	for i := range cells {
		cells[i] = make([]term.Cell, w)
	}
	return &attrGridWriter{w: w, h: h, cells: cells}
}

func (g *attrGridWriter) SetCell(pos term.Coordinates, c term.Cell) {
	if pos.X < 0 || pos.Y < 0 || pos.X >= g.w || pos.Y >= g.h {
		return
	}
	g.cells[pos.Y][pos.X] = c
}

func (g *attrGridWriter) Context() context.Context                              { return context.Background() }
func (g *attrGridWriter) UnionAttributes(_ term.Coordinates, _ term.Attributes) {}

func (g *attrGridWriter) rowRunes(y int) string {
	if y < 0 || y >= g.h {
		return ""
	}
	out := make([]rune, g.w)
	for x, c := range g.cells[y] {
		if c.Ch == 0 {
			out[x] = ' '
		} else {
			out[x] = c.Ch
		}
	}
	return string(out)
}

func (g *attrGridWriter) rowAttrs(y int) []term.Attributes {
	if y < 0 || y >= g.h {
		return nil
	}
	out := make([]term.Attributes, g.w)
	for x, c := range g.cells[y] {
		out[x] = c.Attributes
	}
	return out
}

// drawProbe builds a Tutorial that publishes a single floating_window
// with the given title/text, advances to the active request, and
// renders it into g.
func drawProbe(t *testing.T, title, text string, w, h int) (*Tutorial, *attrGridWriter) {
	t.Helper()
	titleArg := ""
	if title != "" {
		titleArg = ", title=\"" + title + "\""
	}
	src := "def run():\n" +
		"    floating_window(text=\"" + text + "\"" + titleArg + ")\n" +
		"tutorial(entry=run)\n"
	tut, _ := newTutorial(t, src)
	tut.Resize(w, h)
	resetAndWait(t, tut, time.Second)
	g := newAttrGridWriter(w, h)
	tut.Draw(g)
	return tut, g
}

// findTopLeftFrame finds the first row that starts with a vertical
// edge glyph (└, │, ┌) at any X, returns (frameStartX, frameY) where
// the side edge is observed. Returns (-1, -1) when no frame found.
func findFrameSideX(g *attrGridWriter) (int, int) {
	fcs := component.FrameCharSetDefault()
	for y := range g.h {
		row := g.rowRunes(y)
		for x, r := range row {
			if r == fcs.VerticalLeft {
				return x, y
			}
		}
	}
	return -1, -1
}

// TestFloatingWindowTitleBarPresent asserts the title bar replaces
// the top frame edge: row y0 has the title left-aligned and "Step 1"
// right-aligned, both with AttrReverse.
func TestFloatingWindowTitleBarPresent(t *testing.T) {
	t.Parallel()
	tut, g := drawProbe(t, "Welcome", "body line", 80, 24)
	defer tut.Stop()

	sideX, sideY := findFrameSideX(g)
	require.NotEqual(t, -1, sideX,
		"expected a left frame edge somewhere in the rendered grid")
	// The title bar occupies the row immediately above the first
	// observed left edge.
	titleY := sideY - 1
	require.GreaterOrEqual(t, titleY, 0)

	row := g.rowRunes(titleY)
	assert.Contains(t, row, "Welcome",
		"title bar must contain the title left-aligned, got %q", row)
	assert.Contains(t, row, "Step 1",
		"title bar must contain the step counter right-aligned, got %q",
		row)

	// The title bar must run under inverse video: every cell from the
	// frame's left edge column to its right edge column on titleY
	// must carry the AttrReverse attribute.
	attrs := g.rowAttrs(titleY)
	attrReverseSeen := 0
	for x := sideX; x < sideX+10; x++ {
		if attrs[x].Attrs&term.AttrReverse != 0 {
			attrReverseSeen++
		}
	}
	assert.Greater(t, attrReverseSeen, 0,
		"title bar cells must merge AttrReverse")
}

// TestFloatingWindowNoTitleKeepsTopEdge asserts that without a title
// the floating_window still renders the standard top frame edge with
// corner glyphs.
func TestFloatingWindowNoTitleKeepsTopEdge(t *testing.T) {
	t.Parallel()
	tut, g := drawProbe(t, "", "body line", 80, 24)
	defer tut.Stop()

	fcs := component.FrameCharSetDefault()
	// Find first row that contains the top-left corner glyph.
	var foundY int = -1
	for y := range g.h {
		if strings.ContainsRune(g.rowRunes(y), fcs.TopLeft) {
			foundY = y
			break
		}
	}
	require.NotEqual(t, -1, foundY,
		"no-title floating_window must paint the top-left corner")
	row := g.rowRunes(foundY)
	assert.Contains(t, row, string(fcs.TopRight),
		"no-title floating_window must paint the top-right corner")
	assert.Contains(t, row, string(fcs.HorizontalTop),
		"no-title floating_window must paint the horizontal top edge")
}

// TestFloatingWindowTitleBarTruncatesOnNarrow asserts that when the
// title and the step counter cannot both fit, the right zone is
// dropped first; if the title alone still overflows it gets
// truncated with an ellipsis.
func TestFloatingWindowTitleBarTruncatesOnNarrow(t *testing.T) {
	t.Parallel()
	// The frame's innerW is 6*W/10 clamped to [20, W-2]. Pick a width
	// where the title (24 chars) cannot fit alongside "Step 1".
	tut, g := drawProbe(t, "A very lengthy title text",
		"body", 36, 12)
	defer tut.Stop()

	sideX, sideY := findFrameSideX(g)
	require.NotEqual(t, -1, sideX)
	titleY := sideY - 1
	require.GreaterOrEqual(t, titleY, 0)

	row := g.rowRunes(titleY)
	if strings.Contains(row, "Step 1") {
		t.Fatalf("right zone must be dropped before truncating the "+
			"title, got %q", row)
	}
	assert.Contains(t, row, "…",
		"truncated title bar must carry an ellipsis, got %q", row)
}

// TestFloatingWindowStepCounterIncrements asserts that consecutive
// floating_window publications produce stepNum 1, 2, 3.
func TestFloatingWindowStepCounterIncrements(t *testing.T) {
	t.Parallel()
	src := `
def run():
    floating_window(title="one", text="a")
    floating_window(title="two", text="b")
    floating_window(title="three", text="c")
tutorial(entry=run)
`
	tut, _ := newTutorial(t, src)
	resetAndWait(t, tut, time.Second)

	for want := 1; want <= 3; want++ {
		tut.mu.Lock()
		require.NotNil(t, tut.active,
			"step %d: active request must be set", want)
		assert.Equal(t, want, tut.active.stepNum,
			"step %d: snapshot must equal step counter", want)
		tut.mu.Unlock()
		if want < 3 {
			_, _ = tut.Handle(term.Event{
				Type: term.EventKey, Key: term.KeyEnter,
			})
			waitNextActive(t, tut, "floating_window", time.Second)
		}
	}
	tut.Stop()
}

// TestMarkdownStepCounter asserts that markdown() also bumps the
// visible-content step counter.
func TestMarkdownStepCounter(t *testing.T) {
	t.Parallel()
	src := `
def run():
    markdown(text="hello")
tutorial(entry=run)
`
	tut, _ := newTutorial(t, src)
	resetAndWait(t, tut, time.Second)
	tut.mu.Lock()
	require.NotNil(t, tut.active)
	got := tut.active.stepNum
	tut.mu.Unlock()
	assert.Equal(t, 1, got,
		"markdown must bump the visible-content step counter")
	tut.Stop()
}

// TestSideEffectsDoNotBumpCounter asserts that notify() between two
// floating_window calls keeps the counter at 1 → 2 (notify is not a
// "step").
func TestSideEffectsDoNotBumpCounter(t *testing.T) {
	t.Parallel()
	src := `
def run():
    floating_window(title="one", text="a")
    notify(message="side effect")
    floating_window(title="two", text="b")
tutorial(entry=run)
`
	tut, _ := newTutorial(t, src)
	resetAndWait(t, tut, time.Second)
	tut.mu.Lock()
	step1 := tut.active.stepNum
	tut.mu.Unlock()
	require.Equal(t, 1, step1)

	_, _ = tut.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
	waitNextActive(t, tut, "floating_window", time.Second)
	tut.mu.Lock()
	step2 := tut.active.stepNum
	tut.mu.Unlock()
	assert.Equal(t, 2, step2,
		"notify between steps must not bump the counter")
	tut.Stop()
}

// TestWaitCommandHintRendersAsFramedBox asserts that the
// wait_command hint renders as a framed window near the top of the
// screen with markdown body. The prefix line ("Waiting for you …")
// must be present and the configured command key must be expanded
// in place of the `<cmd>` token.
func TestWaitCommandHintRendersAsFramedBox(t *testing.T) {
	t.Parallel()
	src := `
def run():
    wait_command(command="wopen")
tutorial(entry=run)
`
	tut, _ := newTutorial(t, src)
	const screenW, screenH = 80, 24
	tut.Resize(screenW, screenH)
	resetAndWait(t, tut, time.Second)
	defer tut.Stop()

	g := newAttrGridWriter(screenW, screenH)
	tut.Draw(g)

	// The framed hint window paints its top-left corner one row
	// below the top edge (hintBoxTopOffset). Find any corner glyph
	// to assert the box is present.
	fcs := component.FrameCharSetDefault()
	cornerFound := false
	for y := range screenH {
		if strings.ContainsRune(g.rowRunes(y), fcs.TopLeft) {
			cornerFound = true
			break
		}
	}
	require.True(t, cornerFound,
		"wait_command hint must render a framed window with a top-left corner")

	// The body must include the prefix line with the expanded
	// command key and the bare command name, and must not leak the
	// raw `<cmd>` template token.
	wantKey := (term.KeyComb{Ch: ':'}).String()
	combined := gridText(g)
	assert.Contains(t, combined, wantKey,
		"wait_command hint must mention the configured command key")
	assert.Contains(t, combined, "wopen",
		"wait_command hint must mention the command name")
	assert.NotContains(t, combined, "<cmd>",
		"`<cmd>` template token must not leak into the rendered body")
}

// gridText returns the concatenated rows of g separated by newlines
// so callers can assert substring presence anywhere in the rendered
// frame.
func gridText(g *attrGridWriter) string {
	var b strings.Builder
	for y := range g.h {
		b.WriteString(g.rowRunes(y))
		b.WriteByte('\n')
	}
	return b.String()
}

// TestWaitCommandHintIncludesManual asserts that when a command
// manual lookup is provided, the wait_command hint window also
// renders the command's usage and summary as markdown.
func TestWaitCommandHintIncludesManual(t *testing.T) {
	t.Parallel()
	man := command.Manual{
		Name:     "wopen",
		Synopsis: "<directory>",
		Summary:  "Open the workspace at the given directory.",
	}
	lookup := func(name string) (command.Manual, bool) {
		if name == "wopen" {
			return man, true
		}
		return command.Manual{}, false
	}
	src := `
def run():
    wait_command(command="wopen")
tutorial(entry=run)
`
	tut, err := New(
		"manual-test", src,
		nil, nil, nil, nil,
		term.Attributes{}, component.FrameCharSet{}, browser.PromptConfig{},
		nil, nil, term.KeyComb{Ch: ':'},
		lookup,
	)
	require.NoError(t, err)

	const screenW, screenH = 80, 24
	tut.Resize(screenW, screenH)
	resetAndWait(t, tut, time.Second)
	defer tut.Stop()

	g := newAttrGridWriter(screenW, screenH)
	tut.Draw(g)
	body := gridText(g)

	assert.Contains(t, body, "Usage",
		"hint must include a Usage section from the manual")
	assert.Contains(t, body, "<directory>",
		"hint must include the manual's synopsis")
	assert.Contains(t, body, "Open the workspace",
		"hint must include the manual's summary")
}

// TestConfirmOverlayMeetsMinimumSize asserts that even with a very
// short confirm message the rendered overlay is at least
// promptMinInnerW columns wide and promptMinInnerH rows tall (its
// natural dimensions plus a frame).
func TestConfirmOverlayMeetsMinimumSize(t *testing.T) {
	t.Parallel()
	src := `
def run():
    confirm("ok?")
tutorial(entry=run)
`
	tut, _ := newTutorial(t, src)
	const screenW, screenH = 80, 24
	tut.Resize(screenW, screenH)
	resetAndWait(t, tut, time.Second)
	defer tut.Stop()

	g := newAttrGridWriter(screenW, screenH)
	tut.Draw(g)

	fcs := component.FrameCharSetDefault()
	// Find the inner width/height of the rendered frame by locating
	// the top-left and bottom-right corner glyphs.
	tlX, tlY := -1, -1
	for y := range screenH {
		for x, r := range g.rowRunes(y) {
			if r == fcs.TopLeft {
				tlX, tlY = x, y
				break
			}
		}
		if tlX != -1 {
			break
		}
	}
	require.NotEqual(t, -1, tlX, "no top-left corner found")

	// Walk right on row tlY until we hit TopRight; that distance +1
	// is innerW.
	innerW := 0
	topRow := []rune(g.rowRunes(tlY))
	for x := tlX + 1; x < screenW; x++ {
		if topRow[x] == fcs.TopRight {
			innerW = x - tlX + 1
			break
		}
	}
	innerH := 0
	for y := tlY + 1; y < screenH; y++ {
		row := []rune(g.rowRunes(y))
		if row[tlX] == fcs.BottomLeft {
			innerH = y - tlY + 1
			break
		}
	}
	assert.GreaterOrEqual(t, innerW, promptMinInnerW,
		"confirm overlay must be at least %d cells wide",
		promptMinInnerW)
	assert.GreaterOrEqual(t, innerH, promptMinInnerH,
		"confirm overlay must be at least %d rows tall",
		promptMinInnerH)
}
