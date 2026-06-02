// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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

package component

import (
	"fmt"
	"slices"

	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/term/graphemecluster"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"unstable.build/go-tui/cell"
)

var (
	defaultFocusAttr    = term.Attributes{Fg: term.ColorRed}
	defaultNonFocusAttr = term.Attributes{Fg: term.ColorDefault}
	defaultScrollAttr   = term.Attributes{Fg: term.ColorWhite}
	defaultFrameAttr    = term.Attributes{Fg: term.ColorRed}
	defaultSeparator    = "  "
)

// defaultFocusFrameChar is the highlight rune drawn on the top row over
// the focused tab's cell columns. U+2501 BOX DRAWINGS HEAVY HORIZONTAL.
const defaultFocusFrameChar = '━'

// minTabCellWidth is the smallest cell width a non-focused tab can shrink
// to. The focused tab is never shrunk below its full label width unless
// the inner width is smaller than that label.
const minTabCellWidth = 1

// tabBarRightPad is the minimum number of trailing blank cells reserved
// on the right of the last visible tab. The reservation is unconditional
// so the rightmost tab never sits flush against the viewport edge across
// the fast-path and resize-path width distributions.
const tabBarRightPad = 2

// tabCellLayout describes how a single tab is rendered: which tab (idx)
// and how many cells of the bar it occupies.
type tabCellLayout struct {
	idx   int
	width int
}

// tabLayout is the rendered geometry of all visible tabs.
type tabLayout struct {
	cells    []tabCellLayout
	padAfter int
}

type tab struct {
	name     string
	defName  string
	icon     rune
	defIcon  rune
	iconAttr *term.Attributes
	focus    bool
	defAttr  term.Attributes
	attr     term.Attributes
}

// Tabs is a simple component that draws a list of component
// names which can in in Focus (highlighted) or not.
type Tabs struct {
	fileListBuf   *cell.Buffer
	fileListFrame tui.Component
	tabs          []*tab
	width, height int
	focusIdx      int
	highlightIdx  int
	layout        tabLayout

	border           bool
	focusAttr        term.Attributes
	nonFocusAttr     term.Attributes
	focusIconAttr    term.Attributes
	nonFocusIconAttr term.Attributes
	backgroundAttr   term.Attributes
	frameAttr        term.Attributes
	focusFrameAttr   term.Attributes
	focusFrameChar   rune
	bottomHighlight  bool
	frameBorders     component.FrameCharSet
	separator        string
	dirty            bool
}

func newListFrame(
	scrollAttr, frameAttr term.Attributes, buf *cell.Buffer, border bool,
	frameBorders component.FrameCharSet,
) (content tui.Component) {
	scroll := new(Scroll)
	scroll.InitPerformance(buf)
	scroll.Attributes = scrollAttr
	background := term.Cell{Attributes: term.Attributes{Bg: scroll.Attributes.Bg}}
	spanCfg := component.SpanConfig{
		ContentAlignment: component.AlignmentCentered,
		PadVertical:      -1,
	}
	span := component.WithBackground(component.NewSpan(scroll, spanCfg), background)
	if !border {
		return span
	}

	f := component.NewFrame(span)
	f.FrameCharSet = frameBorders
	f.Attributes = frameAttr
	return f
}

// NewTabs allocates storage for a new instance of Tabs and initializes it.
func NewTabs() *Tabs {
	t := new(Tabs)
	t.Init()
	return t
}

// Init initializes this Tab and effectively resets all content.
func (t *Tabs) Init() {
	t.border = true
	t.focusAttr = defaultFocusAttr
	t.nonFocusAttr = defaultNonFocusAttr
	t.focusFrameChar = defaultFocusFrameChar
	t.highlightIdx = -1
	t.frameBorders = component.FrameCharSetDefault()
	t.fileListBuf = new(cell.Buffer)
	t.fileListBuf.InitPerformance(1, 10, ' ')
	t.fileListFrame = newListFrame(
		defaultScrollAttr, defaultFrameAttr, t.fileListBuf,
		t.border, t.frameBorders)
	t.separator = defaultSeparator
	t.dirty = true
}

// SetAttr sets the attributes of the text in focus, text not in focus, the icon
// of the tab in focus, the icon of tabs not in focus, the focus-frame highlight
// drawn on top of the focused tab, the tabs frame and the tabs background.
func (t *Tabs) SetAttr(
	focusTab, tab, focusIcon, icon, focusFrame, frame, background term.Attributes,
) {
	t.focusAttr = focusTab
	t.nonFocusAttr = tab
	t.focusIconAttr = focusIcon
	t.nonFocusIconAttr = icon
	t.backgroundAttr = background
	t.frameAttr = frame
	t.focusFrameAttr = focusFrame
	t.fileListFrame = newListFrame(t.backgroundAttr,
		t.frameAttr, t.fileListBuf, t.border, t.frameBorders)
	t.fileListFrame.Resize(t.width, t.height)
	t.dirty = true
}

// SetBorder defines whether this Tabs draws a border around or not.
// The default is true.
func (t *Tabs) SetBorder(border bool) {
	t.border = border
	t.fileListFrame = newListFrame(
		t.backgroundAttr, t.frameAttr, t.fileListBuf, t.border, t.frameBorders)
	t.fileListFrame.Resize(t.width, t.height)
	t.dirty = true
}

// SetNameSeparator defines the separator used to separate the different tab names.
// By default two spaces are used.
func (t *Tabs) SetNameSeparator(separator string) {
	t.separator = separator
	t.fileListFrame = newListFrame(
		t.backgroundAttr, t.frameAttr, t.fileListBuf, t.border, t.frameBorders)
	t.fileListFrame.Resize(t.width, t.height)
	t.dirty = true
}

// SetFrameCharSet defines the characters used to draw a frame border.
// Note that this has no effect if border is set to false on this Tabs.
func (t *Tabs) SetFrameCharSet(fb component.FrameCharSet) {
	t.frameBorders = fb
	t.fileListFrame = newListFrame(
		t.backgroundAttr, t.frameAttr, t.fileListBuf, t.border, t.frameBorders)
	t.fileListFrame.Resize(t.width, t.height)
	t.dirty = true
}

// SetFocusFrameChar defines the rune drawn on the top row of the tab bar
// over the focused tab's cell columns. The default is `━` (U+2501).
func (t *Tabs) SetFocusFrameChar(r rune) {
	t.focusFrameChar = r
	t.dirty = true
}

// SetBottomHighlight controls where the focus-frame highlight is drawn.
// When true the highlight is rendered on the bottom row of the tab bar
// (above the bottom frame in bordered mode), and labels stay anchored
// to y=0 in borderless mode. When false (the default) the highlight is
// drawn on the top row and labels are pushed down by one row in
// borderless mode.
func (t *Tabs) SetBottomHighlight(on bool) {
	t.bottomHighlight = on
	t.dirty = true
}

// Resize : tui.Component
func (t *Tabs) Resize(width, height int) {
	t.width, t.height = width, height
	t.dirty = true
	t.fileListFrame.Resize(width, height)
}

// Draw : tui.Component
func (t *Tabs) Draw(w term.Writer) {
	if t.dirty {
		t.prepareFileList()
		t.dirty = false
	}

	// In borderless mode with at least 2 rows, push the labels down by
	// one row so y=0 is reserved for the focus highlight. Bordered mode
	// already places labels on y>=1 because the frame occupies y=0.
	// When the highlight is rendered at the bottom, labels stay
	// anchored to y=0 in both modes — the highlight overlays the
	// bottom frame row in bordered mode and the trailing blank row in
	// borderless mode.
	if !t.border && t.height >= 2 && !t.bottomHighlight {
		vw := &component.VirtualWriter{
			Writer: w,
			Offset: term.Coordinates{Y: 1},
			Width:  t.width,
			Height: t.height - 1,
		}
		t.fileListFrame.Draw(vw)
	} else {
		t.fileListFrame.Draw(w)
	}
	t.drawFocusHighlight(w)
}

// drawFocusHighlight overlays the focus-frame rune on y=0 across the
// columns occupied by the highlighted tab's cell. No-op when the
// highlight is disabled (highlightIdx < 0), when there are no tabs,
// when the bar has no second row to host labels (h<2) or when nothing
// is drawable.
func (t *Tabs) drawFocusHighlight(w term.Writer) {
	if t.highlightIdx < 0 || t.height < 2 ||
		t.width <= 0 || len(t.layout.cells) == 0 ||
		t.focusFrameChar == 0 {
		return
	}
	xLeft := 0
	xRight := t.width
	if t.border {
		xLeft = 1
		xRight = t.width - 1
	}
	y := 0
	if t.bottomHighlight {
		y = t.height - 1
	}
	innerX := 0
	sepLen := len(t.separator)
	for i, cell := range t.layout.cells {
		if cell.idx == t.highlightIdx {
			for dx := 0; dx < cell.width; dx++ {
				x := xLeft + innerX + dx
				if x < xLeft || x >= xRight {
					continue
				}
				w.SetCell(term.Coordinates{X: x, Y: y}, term.Cell{
					Width:      1,
					Ch:         t.focusFrameChar,
					Attributes: t.focusFrameAttr,
				})
			}
		}
		innerX += cell.width
		if i < len(t.layout.cells)-1 {
			innerX += sepLen
		}
	}
}

// ResetFocus resets the focus of all the tabs to false.
func (t *Tabs) ResetFocus() {
	for _, tab := range t.tabs {
		tab.focus = false
	}
	t.focusIdx = 0
	t.highlightIdx = -1
	t.dirty = true
}

// SetFocus sets the focus to the tab at idx. If the tab at idx does not exist,
// this method will panic.
func (t *Tabs) SetFocus(idx int) {
	t.tabs[idx].focus = true
	t.focusIdx = idx
	t.highlightIdx = idx
	t.dirty = true
}

// ResetHighlight disables the focus-frame highlight overlay. After this
// call no tab is highlighted until SetHighlight is invoked.
func (t *Tabs) ResetHighlight() {
	t.highlightIdx = -1
	t.dirty = true
}

// SetHighlight enables the focus-frame highlight overlay on the tab at
// idx. If the tab at idx does not exist, this method will panic.
func (t *Tabs) SetHighlight(idx int) {
	if idx < 0 || idx >= len(t.tabs) {
		panic(fmt.Sprintf("Tabs.SetHighlight: idx %d out of range", idx))
	}
	t.highlightIdx = idx
	t.dirty = true
}

// SetTabAttr sets the attributes of the tab at idx. If the tab at idx does not exist,
// this method will panic.
func (t *Tabs) SetTabAttr(idx int, attr term.Attributes) {
	t.tabs[idx].attr = attr
	t.dirty = true
}

// SetIconAttr overrides the attributes of the icon at idx. If the tab at idx does
// not exist, this method will panic.
func (t *Tabs) SetIconAttr(idx int, attr term.Attributes) {
	t.tabs[idx].iconAttr = &attr
	t.dirty = true
}

// TabAttr returns the attributes of the tab at idx. If the tab at idx does not exist,
// this method will panic.
func (t *Tabs) TabAttr(idx int) term.Attributes {
	return t.tabs[idx].attr
}

// TabDefaultAttr returns the default attributes of the tab at idx.
// If the tab at idx does not exist, this method will panic.
func (t *Tabs) TabDefaultAttr(idx int) term.Attributes {
	return t.tabs[idx].defAttr
}

// TabIcon returns the icon of the tab at idx.
// If the tab at idx does not exist, this method will panic.
func (t *Tabs) TabIcon(idx int) rune {
	return t.tabs[idx].icon
}

// SetTabDefaultAttr sets the default attributes of the tab at idx. Calls to ResetTabAttr
// will reset the tab attributes to the given attributes.
// If the tab at idx does not exist, this method will panic.
func (t *Tabs) SetTabDefaultAttr(idx int, attr term.Attributes) {
	t.tabs[idx].defAttr = attr
	t.dirty = true
}

// ResetTabAttr resets the attributes of the tab at idx. If the tab at idx does not exist,
// this method will panic.
func (t *Tabs) ResetTabAttr(idx int) {
	t.tabs[idx].attr = t.tabs[idx].defAttr
	t.dirty = true
}

// SetTabName sets the name of the tab at idx. If the tab at idx does not exist,
// this method will panic.
func (t *Tabs) SetTabName(idx int, name string) {
	t.tabs[idx].name = name
	t.dirty = true
}

// SetTabIcon sets the icon of the tab at idx. If the tab at idx does not exist,
// this method will panic.
func (t *Tabs) SetTabIcon(idx int, icon rune) {
	t.tabs[idx].icon = icon
	t.dirty = true
}

// SetTabDefaultIcon sets the default icon of the tab at idx. Calls to
// ResetTabIcon will reset the tab icon to the given icon. If the tab
// at idx does not exist, this method will panic.
func (t *Tabs) SetTabDefaultIcon(idx int, icon rune) {
	t.tabs[idx].defIcon = icon
	t.dirty = true
}

// ResetTabIcon resets the icon of the tab at idx to either the initial
// icon given to this tab or the last icon set via SetTabDefaultIcon.
// If the tab at idx does not exist, this method will panic.
func (t *Tabs) ResetTabIcon(idx int) {
	t.tabs[idx].icon = t.tabs[idx].defIcon
	t.dirty = true
}

// SetTabDefaultName sets the default name of the tab at idx. Calls to ResetTabName
// will reset the tab name to the given name. If the tab at idx does not exist,
// this method will panic.
func (t *Tabs) SetTabDefaultName(idx int, name string) {
	t.tabs[idx].defName = name
	t.dirty = true
}

// ResetTabName resets the name of the tab at idx to either the initial name
// given to this tab or the last name set via SetDefaultTabName.
// If the tab at idx does not exist, this method will panic.
func (t *Tabs) ResetTabName(idx int) {
	t.tabs[idx].name = t.tabs[idx].defName
	t.dirty = true
}

// TabName returns the name of the tab at idx.
// If the tab at idx does not exist, this method will panic.
func (t *Tabs) TabName(idx int) string {
	return t.tabs[idx].name
}

// DefaultTabName returns the default name of the tab at idx.
// If the tab at idx does not exist, this method will panic.
func (t *Tabs) DefaultTabName(idx int) string {
	return t.tabs[idx].defName
}

// Add adds a tab with the given name and icon.
func (t *Tabs) Add(icon rune, name string) int {
	t.dirty = true
	tt := &tab{
		defAttr: term.Attributes{},
		attr:    term.Attributes{},
		defName: name,
		icon:    icon,
		defIcon: icon,
		name:    name,
	}
	if t.tabs == nil {
		t.tabs = make([]*tab, 1)
		tt.focus = true
		t.focusIdx = 0
		t.highlightIdx = 0
		t.tabs[0] = tt
		return 0
	}
	idx := len(t.tabs)
	t.tabs = append(t.tabs, tt)
	return idx
}

// Remove removes the tab at idx.
func (t *Tabs) Remove(idx int) bool {
	focused := t.currentFocusTab()
	highlighted := t.currentHighlightTab()
	t.doRemoveTab(idx)
	t.restoreFocusIdx(focused)
	t.restoreHighlightIdx(highlighted)
	t.dirty = true
	return true
}

// MoveRight moves the tab at idx to the right.
func (t *Tabs) MoveRight(idx int) bool {
	if idx >= len(t.tabs)-1 {
		return false
	}
	focused := t.currentFocusTab()
	highlighted := t.currentHighlightTab()
	tt := t.doRemoveTab(idx)
	idx++
	t.doInsertTab(idx, tt)
	t.restoreFocusIdx(focused)
	t.restoreHighlightIdx(highlighted)
	t.dirty = true
	return true
}

// MoveLeft moves the tab at idx to the left.
func (t *Tabs) MoveLeft(idx int) bool {
	if idx <= 0 {
		return false
	}
	focused := t.currentFocusTab()
	highlighted := t.currentHighlightTab()
	tt := t.doRemoveTab(idx)
	idx--
	t.doInsertTab(idx, tt)
	t.restoreFocusIdx(focused)
	t.restoreHighlightIdx(highlighted)
	t.dirty = true
	return true
}

// MoveTo moves the tab at curridx to the given idx.
func (t *Tabs) MoveTo(curridx, idx int) bool {
	if curridx < 0 || idx < 0 || curridx >= len(t.tabs) || idx >= len(t.tabs) {
		return false
	}
	focused := t.currentFocusTab()
	highlighted := t.currentHighlightTab()
	tt := t.doRemoveTab(curridx)
	t.doInsertTab(idx, tt)
	t.restoreFocusIdx(focused)
	t.restoreHighlightIdx(highlighted)
	t.dirty = true
	return true
}

// RemoveAll removes all tabs.
func (t *Tabs) RemoveAll() bool {
	ret := t.Size() != 0
	// Clear pointers before truncating so removed *tab values can be GC'd
	// without waiting for subsequent appends.
	clear(t.tabs)
	t.tabs = t.tabs[:0]
	t.focusIdx = 0
	t.dirty = true
	return ret
}

// TabAt returns the idx of the tab at pos, or panics if pos is
// out of bounds.
func (t *Tabs) TabAt(pos term.Coordinates) (int, bool) {
	if len(t.tabs) == 0 {
		return -1, false
	}

	// If we have a current layout (the component has been drawn at
	// least once), use it: it is the authoritative geometry.
	if len(t.layout.cells) > 0 {
		posX := pos.X
		if t.border {
			if posX <= 0 {
				return t.layout.cells[0].idx, true
			}
			posX--
		}
		sepLen := len(t.separator)
		x := 0
		for i, cell := range t.layout.cells {
			end := x + cell.width
			if posX >= x && posX < end {
				return cell.idx, true
			}
			x = end
			if i < len(t.layout.cells)-1 {
				if posX >= x && posX < x+sepLen {
					return cell.idx, true
				}
				x += sepLen
			}
		}
		return -1, false
	}

	// Fallback: no layout computed yet (component never drawn / never
	// resized). Hit-test against a natural-width layout so callers can
	// resolve clicks before a draw cycle has happened.
	sepLen := len(t.separator)
	x := 0
	for i, tab := range t.tabs {
		w := tabFullWidth(tab)
		end := x + w
		if pos.X >= x && pos.X < end {
			return i, true
		}
		x = end
		if i < len(t.tabs)-1 {
			if pos.X >= x && pos.X < x+sepLen {
				return i, true
			}
			x += sepLen
		}
	}
	return 0, true
}

// Tab returns the name of the tab at idx.
func (t *Tabs) Tab(idx int) (string, bool) {
	if idx >= len(t.tabs) {
		return "", false
	}
	return t.tabs[idx].name, true
}

// Size returns the number of tabs.
func (t *Tabs) Size() int {
	return len(t.tabs)
}

func (t *Tabs) prepareFileList() {
	t.fileListBuf.Reset()
	t.layout = tabLayout{}

	if t.width == 0 || t.height == 0 {
		return
	}

	t.layout = t.calculateLayout()
	if len(t.layout.cells) == 0 {
		return
	}

	next := term.Coordinates{}
	for i, cell := range t.layout.cells {
		next = t.insertTabCell(next, cell)
		if i < len(t.layout.cells)-1 {
			_, next = t.fileListBuf.InsertString(next, t.separator)
		}
	}
	for i := 0; i < t.layout.padAfter; i++ {
		next = t.fileListBuf.Insert(next, ' ')
	}
}

// insertTabCell writes a single tab into the buffer using exactly
// cell.width cells. When cell.width >= the tab's full width the label is
// rendered in full; otherwise the icon (if any) is written first and the
// rest of the cells are filled with as many name graphemes as fit.
func (t *Tabs) insertTabCell(pos term.Coordinates, cell tabCellLayout) term.Coordinates {
	tab := t.tabs[cell.idx]
	attr := tab.attr
	iconAttr := tab.iconAttr
	if tab.focus {
		attr = term.AttributesUnion(t.focusAttr, attr)
		if iconAttr == nil {
			iconAttr = &t.focusIconAttr
		}
	} else {
		attr = term.AttributesUnion(t.nonFocusAttr, attr)
		if iconAttr == nil {
			iconAttr = &t.nonFocusIconAttr
		}
	}

	remaining := cell.width
	if tab.icon != 0 {
		iconWidth := runeCellWidth(tab.icon)
		if remaining >= iconWidth {
			pos = t.fileListBuf.InsertWithAttr(pos, tab.icon, *iconAttr)
			remaining -= iconWidth
			if remaining > 0 {
				pos = t.fileListBuf.Insert(pos, ' ')
				remaining--
			}
		} else {
			// No room for the icon: fill the cell with blanks.
			for remaining > 0 {
				pos = t.fileListBuf.Insert(pos, ' ')
				remaining--
			}
			return pos
		}
	}

	name := []rune(tab.name)
	nameLen, usedNameWidth := runesThatFit(name, remaining)
	for _, r := range name[:nameLen] {
		pos = t.fileListBuf.InsertWithAttr(pos, r, attr)
	}
	remaining -= usedNameWidth
	for remaining > 0 {
		pos = t.fileListBuf.Insert(pos, ' ')
		remaining--
	}
	return pos
}

// calculateLayout decides the visible window of tabs and the width
// allocated to each. See the algorithm description in tabs_test.go.
func (t *Tabs) calculateLayout() tabLayout {
	innerW := t.innerWidth()
	if len(t.tabs) == 0 || innerW <= 0 {
		return tabLayout{}
	}
	// Reserve tabBarRightPad trailing cells so the rightmost tab is
	// never flush against the viewport edge. All width-budget decisions
	// run against budgetW; padAfter naturally fills the reservation
	// plus any extra slack when the row is drawn.
	budgetW := innerW - tabBarRightPad
	if budgetW < 0 {
		budgetW = 0
	}

	sepLen := len(t.separator)
	fullWidths := make([]int, len(t.tabs))
	totalFull := 0
	for i, tab := range t.tabs {
		fullWidths[i] = tabFullWidth(tab)
		totalFull += fullWidths[i]
	}

	// Fast path: everything fits at full width.
	if totalFull+sepLen*(len(t.tabs)-1) <= budgetW {
		layout := tabLayout{cells: make([]tabCellLayout, len(t.tabs))}
		for i := range t.tabs {
			layout.cells[i] = tabCellLayout{idx: i, width: fullWidths[i]}
		}
		layout.padAfter = innerW - totalFull - sepLen*(len(t.tabs)-1)
		return layout
	}

	focusIdx := t.focusedIndex()
	start, end := t.visibleRangeForFocus(focusIdx, fullWidths[focusIdx], sepLen, budgetW)
	if start > end {
		return tabLayout{}
	}

	visibleCount := end - start + 1
	cellBudget := budgetW - sepLen*(visibleCount-1)
	if cellBudget < 0 {
		cellBudget = 0
	}

	widths := make([]int, len(t.tabs))
	focusWidth := fullWidths[focusIdx]
	if focusWidth > cellBudget {
		focusWidth = cellBudget
	}
	if focusWidth < 0 {
		focusWidth = 0
	}
	widths[focusIdx] = focusWidth

	remaining := cellBudget - focusWidth
	nonFocusedCount := visibleCount - 1
	if nonFocusedCount > 0 {
		fairShare := remaining / nonFocusedCount
		leftover := remaining - fairShare*nonFocusedCount

		// First pass: equal share + the first `leftover` non-focused
		// tabs (in visible order) get +1, all capped at full width.
		used := 0
		nfIdx := 0
		for i := start; i <= end; i++ {
			if i == focusIdx {
				continue
			}
			w := fairShare
			if nfIdx < leftover {
				w++
			}
			if w < minTabCellWidth {
				w = minTabCellWidth
			}
			if w > fullWidths[i] {
				w = fullWidths[i]
			}
			widths[i] = w
			used += w
			nfIdx++
		}

		// Second pass: distribute any surplus from caps to the leftmost
		// non-focused tabs that still have room.
		surplus := remaining - used
		for surplus > 0 {
			progressed := false
			for i := start; i <= end && surplus > 0; i++ {
				if i == focusIdx {
					continue
				}
				if widths[i] < fullWidths[i] {
					widths[i]++
					surplus--
					progressed = true
				}
			}
			if !progressed {
				break
			}
		}
	}

	layout := tabLayout{cells: make([]tabCellLayout, 0, visibleCount)}
	used := 0
	for i := start; i <= end; i++ {
		w := widths[i]
		if w <= 0 {
			continue
		}
		layout.cells = append(layout.cells, tabCellLayout{idx: i, width: w})
		used += w
	}
	used += sepLen * max(0, len(layout.cells)-1)
	if used < innerW {
		layout.padAfter = innerW - used
	}
	return layout
}

// visibleRangeForFocus returns the inclusive [start, end] indices of
// tabs that should be visible. Tabs farther from focusIdx are dropped
// first when even minTabCellWidth cannot fit all of them. On ties the
// start side is dropped.
func (t *Tabs) visibleRangeForFocus(focusIdx, focusFullWidth, sepLen, budgetW int) (int, int) {
	start, end := 0, len(t.tabs)-1
	for start < end {
		count := end - start + 1
		minTotal := focusFullWidth + (count-1)*minTabCellWidth + (count-1)*sepLen
		if minTotal <= budgetW {
			break
		}
		if focusIdx-start >= end-focusIdx {
			start++
		} else {
			end--
		}
	}
	return start, end
}

func (t *Tabs) focusedIndex() int {
	if t.focusIdx >= 0 && t.focusIdx < len(t.tabs) {
		if t.tabs[t.focusIdx].focus {
			return t.focusIdx
		}
	}
	for i, tab := range t.tabs {
		if tab.focus {
			return i
		}
	}
	return 0
}

func (t *Tabs) currentFocusTab() *tab {
	if t.focusIdx >= 0 && t.focusIdx < len(t.tabs) {
		return t.tabs[t.focusIdx]
	}
	return nil
}

func (t *Tabs) currentHighlightTab() *tab {
	if t.highlightIdx >= 0 && t.highlightIdx < len(t.tabs) {
		return t.tabs[t.highlightIdx]
	}
	return nil
}

func (t *Tabs) restoreFocusIdx(focused *tab) {
	if focused != nil {
		for i, tab := range t.tabs {
			if tab == focused {
				t.focusIdx = i
				return
			}
		}
	}
	for i, tab := range t.tabs {
		if tab.focus {
			t.focusIdx = i
			return
		}
	}
	if t.focusIdx >= len(t.tabs) {
		if len(t.tabs) == 0 {
			t.focusIdx = 0
		} else {
			t.focusIdx = len(t.tabs) - 1
		}
	}
}

func (t *Tabs) restoreHighlightIdx(highlighted *tab) {
	if highlighted == nil {
		t.highlightIdx = -1
		return
	}
	for i, tab := range t.tabs {
		if tab == highlighted {
			t.highlightIdx = i
			return
		}
	}
	t.highlightIdx = -1
}

func (t *Tabs) innerWidth() int {
	if t.border {
		return max(0, t.width-2)
	}
	return t.width
}

func tabFullWidth(t *tab) int {
	if t.icon != 0 {
		return stringCellWidth(t.name) + runeCellWidth(t.icon) + 1
	}
	return stringCellWidth(t.name)
}

func stringCellWidth(s string) int {
	return graphemecluster.StringWidth(s)
}

func runeCellWidth(r rune) int {
	return stringCellWidth(string(r))
}

// runesThatFit returns how many runes from the prefix fit into budget
// cells, and the actual cell width consumed.
func runesThatFit(runes []rune, budget int) (int, int) {
	used := 0
	for i, r := range runes {
		w := runeCellWidth(r)
		if used+w > budget {
			return i, used
		}
		used += w
	}
	return len(runes), used
}

func (t *Tabs) doRemoveTab(idx int) *tab {
	ret := t.tabs[idx]
	// slices.Delete clears the tail slot so the removed *tab is not
	// retained in the backing array past len.
	t.tabs = slices.Delete(t.tabs, idx, idx+1)
	return ret
}

func (t *Tabs) doInsertTab(idx int, tt *tab) {
	t.tabs = append(t.tabs, nil)
	copy(t.tabs[idx+1:], t.tabs[idx:])
	t.tabs[idx] = tt
}
