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

package standard

import (
	"errors"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/term/graphemecluster"
	"unstable.build/go-tui/cell"
	inputhandler "unstable.build/go-tui/handler/input"
	"unstable.build/go-tui/text"
)

const (
	searchInputWidth = 32
	searchPaddingX   = 1
	searchPaddingY   = 1
	searchRowGap     = 1
	searchColGap     = 1
)

type searchMode uint8

const (
	searchModeFind searchMode = iota
	searchModeReplace
)

type searchFocus uint8

const (
	searchFocusQuery searchFocus = iota
	searchFocusReplacement
)

type searchButton uint8

const (
	searchButtonNone searchButton = iota
	searchButtonUpgrade
	searchButtonNext
	searchButtonAll
)

type searchRect struct {
	x, y, width, height int
}

func (r searchRect) contains(x, y int) bool {
	return x >= r.x && x < r.x+r.width && y >= r.y && y < r.y+r.height
}

type searchLayout struct {
	width, height, contentHeight int
	query, replacement           searchRect
	upgrade, next, all           searchRect
}

type searchHandler struct {
	text.Handler
	controller searchController
	config     SearchConfig
	floating   *searchFloating
	window     browserapi.Window
}

func keyMatches(ev term.Event, key term.KeyComb) bool {
	return ev.Type == term.EventKey && ev.Mod == key.Mod && ev.Key == key.Key && ev.Ch == key.Ch
}

func findKeyMatches(ev term.Event, key term.KeyComb) bool {
	return keyMatches(ev, key) ||
		(key == (term.KeyComb{Mod: term.ModMeta, Ch: 'f'}) &&
			keyMatches(ev, term.KeyComb{Mod: term.ModCtrl, Ch: 'f'}))
}

func (h *searchHandler) Handle(ev term.Event) (bool, bool) {
	if findKeyMatches(ev, h.config.FindKey) {
		if h.floating == nil {
			h.open(searchModeFind)
		} else {
			h.floating.setFocus(searchFocusQuery)
			h.controller.advanceSearch()
			h.floating.ensureFocusVisible()
		}
		return false, true
	}
	if keyMatches(ev, h.config.ReplaceKey) {
		if h.floating == nil {
			h.open(searchModeReplace)
		} else if h.floating.mode == searchModeFind {
			h.floating.upgrade()
		} else {
			h.floating.setFocus(searchFocusReplacement)
			h.floating.ensureFocusVisible()
		}
		return false, true
	}
	return h.Handler.Handle(ev)
}

func (h *searchHandler) open(mode searchMode) {
	origin := h.controller.searchOrigin()
	query := h.controller.searchLast()
	f := newSearchFloating(h, mode, query)
	win, err := h.config.WindowManager.Floating(f, browserapi.FloatingConfig{
		Alignment: component.AlignmentTop | component.AlignmentHorizontallyCentered,
		Offset:    term.Coordinates{Y: 2},
		Title:     "Find / Replace",
	})
	if err != nil {
		if mode == searchModeFind {
			if standard, ok := h.controller.(*standardHandler); ok {
				standard.log(log.WarnLevel, "open floating find: %v", err)
				_, _ = standard.cfg.notifications.Notify(browserapi.LevelWarn,
					"Unable to open floating find: %v", err)
				standard.startFind()
			}
		} else if standard, ok := h.controller.(*standardHandler); ok {
			standard.log(log.WarnLevel, "open floating replace: %v", err)
			_, _ = standard.cfg.notifications.Notify(browserapi.LevelWarn,
				"Unable to open floating replace: %v", err)
		}
		return
	}
	h.floating = f
	h.window = win
	h.controller.beginSearch([]rune(query), origin)
}

func (h *searchHandler) finish(closeWindow bool) error {
	if h.floating == nil {
		return nil
	}
	h.floating = nil
	win := h.window
	h.window = nil
	h.controller.finishSearch()
	if closeWindow && win != nil {
		return h.config.WindowManager.CloseWindow(win)
	}
	return nil
}

func (h *searchHandler) Close() error {
	return errors.Join(h.finish(true), h.Handler.Close())
}

type searchInput struct {
	box             *inputhandler.Box
	buf             *cell.Buffer
	placeholder     string
	attr            term.Attributes
	placeholderAttr term.Attributes
	width, height   int
}

func newSearchInput(value, placeholder string, cfg SearchConfig) *searchInput {
	buf := cell.NewBuffer()
	buf.WriteString(value)
	box := inputhandler.NewBox(buf, Editor(), inputhandler.BoxConfig{
		Placeholder:       placeholder,
		PlaceholderConfig: component.StringConfig{Attributes: cfg.PlaceholderAttr},
		DefaultFrameAttr:  cfg.FrameAttr,
		ContentConfig:     cfg.InputAttr,
	})
	box.Resize(searchInputWidth, 3)
	lines := strings.Split(value, "\n")
	if len(value) > 0 {
		_ = box.SetCursorAtScroll(term.Coordinates{
			X: len([]rune(lines[len(lines)-1])),
			Y: len(lines) - 1,
		})
	}
	return &searchInput{
		box:             box,
		buf:             buf,
		placeholder:     placeholder,
		attr:            cfg.InputAttr,
		placeholderAttr: cfg.PlaceholderAttr,
	}
}

func (i *searchInput) text() string { return i.buf.String() }

func (i *searchInput) Resize(width, height int) {
	i.width, i.height = max(0, width), max(0, height)
	i.box.Resize(i.width+2, i.height+2)
}

func displayRows(value string, width int) int {
	if width <= 0 {
		return 0
	}
	if value == "" {
		return 1
	}
	rows, x := 1, 0
	for _, ch := range value {
		if ch == '\n' {
			rows, x = rows+1, 0
			continue
		}
		cw := max(1, graphemecluster.StringWidth(string(ch)))
		if x > 0 && x+cw > width {
			rows, x = rows+1, 0
		}
		x += cw
		if x >= width {
			rows, x = rows+1, 0
		}
	}
	return max(1, rows)
}

func (i *searchInput) Draw(w term.Writer) {
	for y := range i.height {
		for x := range i.width {
			w.UnionAttributes(term.Coordinates{X: x, Y: y}, i.attr)
		}
	}
	value, attr := i.text(), i.attr
	if value == "" {
		value, attr = i.placeholder, i.placeholderAttr
	}
	from, to, selected := i.box.SelectionBounds()
	if selected {
		from, to = term.CoordinatesSort(from, to)
	}
	x, y, logicalX, logicalY := 0, 0, 0, 0
	for _, ch := range value {
		if y >= i.height {
			break
		}
		if ch == '\n' {
			x, y = 0, y+1
			logicalX, logicalY = 0, logicalY+1
			continue
		}
		cw := max(1, graphemecluster.StringWidth(string(ch)))
		if x > 0 && x+cw > i.width {
			x, y = 0, y+1
		}
		if y >= i.height || x >= i.width {
			break
		}
		w.SetCell(term.Coordinates{X: x, Y: y}, term.NewCell(ch, uint8(cw), attr))
		logical := term.Coordinates{X: logicalX, Y: logicalY}
		if selected && coordinatesInRange(logical, from, to) {
			for dx := range cw {
				w.UnionAttributes(term.Coordinates{X: x + dx, Y: y}, term.Attributes{Attrs: term.AttrReverse})
			}
		}
		logicalX++
		x += cw
		if x >= i.width {
			x, y = 0, y+1
		}
	}
}

func coordinatesInRange(pos, from, to term.Coordinates) bool {
	if pos.Y < from.Y || pos.Y > to.Y {
		return false
	}
	if pos.Y == from.Y && pos.X < from.X {
		return false
	}
	if pos.Y == to.Y && pos.X >= to.X {
		return false
	}
	return true
}

func (i *searchInput) Handle(ev term.Event) (bool, bool) {
	return i.box.Handle(ev)
}

func (i *searchInput) cursor() (term.Coordinates, term.CursorStyle, bool) {
	logical := i.box.CursorAtScroll()
	idx := 0
	for y, line := range strings.Split(i.text(), "\n") {
		if y == logical.Y {
			idx += min(logical.X, len([]rune(line)))
			break
		}
		idx += len([]rune(line)) + 1
	}
	x, y := 0, 0
	for n, ch := range []rune(i.text()) {
		if n >= idx {
			break
		}
		if ch == '\n' {
			x, y = 0, y+1
			continue
		}
		cw := max(1, graphemecluster.StringWidth(string(ch)))
		if x > 0 && x+cw > i.width {
			x, y = 0, y+1
		}
		x += cw
		if x >= i.width {
			x, y = 0, y+1
		}
	}
	_, style, show := i.box.Cursor()
	return term.Coordinates{X: x, Y: y}, style, show
}

type searchFloating struct {
	owner       *searchHandler
	mode        searchMode
	focus       searchFocus
	query       *searchInput
	replacement *searchInput
	layout      searchLayout
	seek        int
	hover       searchButton
	pressed     searchButton
	mouseInput  *searchInput
	closed      bool
}

func newSearchFloating(owner *searchHandler, mode searchMode, query string) *searchFloating {
	f := &searchFloating{owner: owner, mode: mode, query: newSearchInput(query, "Find", owner.config)}
	if mode == searchModeReplace {
		f.replacement = newSearchInput("", "Replace with", owner.config)
	}
	f.setFocus(searchFocusQuery)
	return f
}

func (f *searchFloating) setFocus(focus searchFocus) {
	f.focus = focus
}

func (f *searchFloating) upgrade() {
	if f.mode == searchModeReplace {
		return
	}
	f.mode = searchModeReplace
	f.replacement = newSearchInput("", "Replace with", f.owner.config)
	f.setFocus(searchFocusQuery)
	f.Resize(f.layout.width, f.layout.height)
}

func naturalInputWidth(f *searchFloating) int {
	w := graphemecluster.StringWidth("Find")
	if query := f.query.text(); query != "" {
		w = graphemecluster.StringWidth(query) + 1
	}
	if f.mode == searchModeReplace {
		replacementWidth := graphemecluster.StringWidth("Replace with")
		if replacement := f.replacement.text(); replacement != "" {
			replacementWidth = graphemecluster.StringWidth(replacement) + 1
		}
		w = max(w, replacementWidth)
	}
	return max(searchInputWidth, w+2)
}

func (f *searchFloating) controlsWidth() int {
	if f.mode == searchModeReplace {
		return 15
	}
	return 9
}

func (f *searchFloating) Dimensions() (int, int) {
	width := 2*searchPaddingX + naturalInputWidth(f) +
		searchColGap + f.controlsWidth()
	return width, f.measure(width, 0).contentHeight
}

func (f *searchFloating) Height(width int) int {
	return f.measure(max(0, width), 0).contentHeight
}

func (f *searchFloating) measure(width, height int) searchLayout {
	ret := searchLayout{width: width, height: height}
	available := max(0, width-2*searchPaddingX-searchColGap)
	controls := f.controlsWidth()
	inputWidth := min(naturalInputWidth(f), max(3, available-controls))
	inputWidth = min(inputWidth, max(0, width-searchPaddingX))
	innerWidth := max(1, inputWidth-2)
	queryH := displayRows(f.query.text(), innerWidth) + 2
	ret.query = searchRect{x: searchPaddingX, y: searchPaddingY, width: inputWidth, height: queryH}
	buttonX := ret.query.x + inputWidth + searchColGap
	ret.upgrade = searchRect{x: buttonX, y: ret.query.y + (queryH-1)/2, width: 9, height: 1}
	ret.contentHeight = searchPaddingY + queryH
	if f.mode == searchModeReplace {
		replaceH := displayRows(f.replacement.text(), innerWidth) + 2
		ret.replacement = searchRect{x: searchPaddingX, y: ret.query.y + queryH + searchRowGap,
			width: inputWidth, height: replaceH}
		by := ret.replacement.y + (replaceH-1)/2
		ret.next = searchRect{x: buttonX, y: by, width: 9, height: 1}
		ret.all = searchRect{x: buttonX + 10, y: by, width: 5, height: 1}
		ret.contentHeight = ret.replacement.y + replaceH
	}
	return ret
}

func (f *searchFloating) Resize(width, height int) {
	f.layout = f.measure(max(0, width), max(0, height))
	f.query.Resize(max(0, f.layout.query.width-2), max(0, f.layout.query.height-2))
	if f.replacement != nil {
		f.replacement.Resize(max(0, f.layout.replacement.width-2),
			max(0, f.layout.replacement.height-2))
	}
	f.seek = min(f.seek, f.MaxSeekOffset())
	f.ensureFocusVisible()
}

func (f *searchFloating) focusedRect() searchRect {
	if f.focus == searchFocusReplacement && f.mode == searchModeReplace {
		return f.layout.replacement
	}
	return f.layout.query
}

func (f *searchFloating) ensureFocusVisible() {
	r := f.focusedRect()
	if r.y < f.seek {
		f.seek = r.y
	} else if f.layout.height > 0 && r.y+r.height > f.seek+f.layout.height {
		f.seek = r.y + r.height - f.layout.height
	}
	input := f.query
	if f.focus == searchFocusReplacement && f.replacement != nil {
		input = f.replacement
	}
	pos, _, _ := input.cursor()
	cursorY := r.y + 1 + pos.Y
	if cursorY < f.seek {
		f.seek = cursorY
	} else if f.layout.height > 0 && cursorY >= f.seek+f.layout.height {
		f.seek = cursorY - f.layout.height + 1
	}
	f.seek = max(0, min(f.seek, f.MaxSeekOffset()))
}

func drawFrame(w term.Writer, r searchRect, attr term.Attributes) {
	if r.width < 3 || r.height < 3 {
		return
	}
	component.DrawFrame(w, component.FrameCharSetDefault(), attr, r.width-1, r.height-1)
}

func translatedWriter(w term.Writer, r searchRect, seek int) component.VirtualWriter {
	return component.VirtualWriter{Writer: w, Offset: term.Coordinates{X: r.x, Y: r.y - seek},
		Width: r.width, Height: r.height}
}

func (f *searchFloating) Draw(w term.Writer) {
	viewport := component.VirtualWriter{Writer: w, Width: f.layout.width, Height: f.layout.height}
	for y := range f.layout.height {
		for x := range f.layout.width {
			viewport.UnionAttributes(term.Coordinates{X: x, Y: y}, f.owner.config.Attr)
		}
	}
	f.drawInput(&viewport, f.query, f.layout.query)
	if f.mode == searchModeFind {
		f.drawButton(&viewport, f.layout.upgrade, " Replace ", searchButtonUpgrade)
		return
	}
	f.drawInput(&viewport, f.replacement, f.layout.replacement)
	f.drawButton(&viewport, f.layout.next, " Replace ", searchButtonNext)
	f.drawButton(&viewport, f.layout.all, " All ", searchButtonAll)
}

func (f *searchFloating) drawInput(w term.Writer, input *searchInput, r searchRect) {
	frameAttr := f.owner.config.FrameAttr
	if (input == f.query && f.focus == searchFocusQuery) ||
		(input == f.replacement && f.focus == searchFocusReplacement) {
		frameAttr = f.owner.config.FocusFrameAttr
	}
	fw := translatedWriter(w, r, f.seek)
	drawFrame(&fw, searchRect{width: r.width, height: r.height}, frameAttr)
	inner := searchRect{x: r.x + 1, y: r.y + 1, width: max(0, r.width-2), height: max(0, r.height-2)}
	iw := translatedWriter(w, inner, f.seek)
	input.Draw(&iw)
}

func (f *searchFloating) drawButton(w term.Writer, r searchRect, label string, button searchButton) {
	if r.x+r.width > f.layout.width || r.y-f.seek < 0 || r.y-f.seek >= f.layout.height {
		return
	}
	attr := f.owner.config.ButtonAttr
	if f.hover == button {
		attr = f.owner.config.ButtonHoverAttr
	}
	x := r.x
	for _, ch := range label {
		w.SetCell(term.Coordinates{X: x, Y: r.y - f.seek},
			term.NewCell(ch, 1, attr))
		x++
	}
}

func (f *searchFloating) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	input, r := f.query, f.layout.query
	if f.focus == searchFocusReplacement && f.replacement != nil {
		input, r = f.replacement, f.layout.replacement
	}
	pos, style, show := input.cursor()
	pos.X += r.x + 1
	pos.Y += r.y + 1 - f.seek
	if pos.Y < 0 || pos.Y >= f.layout.height || pos.X >= f.layout.width {
		show = false
	}
	return pos, style, show
}

func (f *searchFloating) Selection() (string, bool) {
	input := f.query
	if f.focus == searchFocusReplacement && f.replacement != nil {
		input = f.replacement
	}
	return input.box.Selection()
}

func (f *searchFloating) Handle(ev term.Event) (bool, bool) {
	if findKeyMatches(ev, f.owner.config.FindKey) {
		f.setFocus(searchFocusQuery)
		f.owner.controller.advanceSearch()
		f.ensureFocusVisible()
		return false, true
	}
	if keyMatches(ev, f.owner.config.ReplaceKey) {
		if f.mode == searchModeFind {
			f.upgrade()
		} else {
			f.setFocus(searchFocusReplacement)
			f.ensureFocusVisible()
		}
		return false, true
	}
	if ev.Type == term.EventKey {
		if ev.Key == term.KeyEsc {
			return true, true
		}
		if ev.Key == term.KeyTab {
			if f.mode == searchModeReplace {
				if f.focus == searchFocusQuery {
					f.setFocus(searchFocusReplacement)
				} else {
					f.setFocus(searchFocusQuery)
				}
				f.ensureFocusVisible()
			}
			return false, true
		}
		if ev.Key == term.KeyEnter {
			f.owner.controller.advanceSearch()
			return false, true
		}
	}
	if ev.Type == term.EventMouse {
		return f.handleMouse(ev)
	}
	input := f.query
	if f.focus == searchFocusReplacement && f.replacement != nil {
		input = f.replacement
	}
	before := input.text()
	_, handled := input.Handle(ev)
	if input.text() != before {
		if input == f.query {
			f.owner.controller.setSearchQuery(input.text())
		}
		f.Resize(f.layout.width, f.layout.height)
	}
	return false, handled
}

func (f *searchFloating) buttonAt(x, y int) searchButton {
	y += f.seek
	for _, item := range []struct {
		button searchButton
		rect   searchRect
	}{{searchButtonUpgrade, f.layout.upgrade}, {searchButtonNext, f.layout.next}, {searchButtonAll, f.layout.all}} {
		if item.rect.width > 0 && item.rect.x+item.rect.width <= f.layout.width && item.rect.contains(x, y) {
			return item.button
		}
	}
	return searchButtonNone
}

func (f *searchFloating) handleMouse(ev term.Event) (bool, bool) {
	button := f.buttonAt(ev.MouseX, ev.MouseY)
	contentY := ev.MouseY + f.seek
	if f.mouseInput != nil && (ev.Key == term.MouseLeft || ev.Key == term.MouseRelease) {
		input := f.mouseInput
		rect := f.layout.query
		if input == f.replacement {
			rect = f.layout.replacement
		}
		ev.MouseX -= rect.x
		ev.MouseY = contentY - rect.y
		_, handled := input.Handle(ev)
		if ev.Key == term.MouseRelease {
			input.clearCollapsedSelection()
			f.mouseInput = nil
		}
		f.ensureFocusVisible()
		return false, handled
	}
	if button == searchButtonNone && ev.Key == term.MouseLeft {
		var input *searchInput
		var rect searchRect
		switch {
		case f.layout.query.contains(ev.MouseX, contentY):
			f.setFocus(searchFocusQuery)
			input, rect = f.query, f.layout.query
		case f.mode == searchModeReplace && f.layout.replacement.contains(ev.MouseX, contentY):
			f.setFocus(searchFocusReplacement)
			input, rect = f.replacement, f.layout.replacement
		}
		if input != nil {
			ev.MouseX -= rect.x
			ev.MouseY = contentY - rect.y
			if ev.MouseX > 0 && ev.MouseY > 0 && ev.MouseX < rect.width-1 && ev.MouseY < rect.height-1 {
				f.mouseInput = input
				_, handled := input.Handle(ev)
				f.ensureFocusVisible()
				return false, handled
			}
		}
	}
	if ev.Key == 0 {
		f.hover = button
		return false, button != searchButtonNone
	}
	if ev.Key == term.MouseLeft {
		f.pressed = button
		return false, button != searchButtonNone
	}
	if ev.Key != term.MouseRelease {
		return false, false
	}
	pressed := f.pressed
	f.pressed = searchButtonNone
	if pressed == searchButtonNone || pressed != button {
		return false, button != searchButtonNone
	}
	switch button {
	case searchButtonUpgrade:
		f.upgrade()
	case searchButtonNext:
		f.owner.controller.replaceNext(f.replacement.text())
	case searchButtonAll:
		f.owner.controller.replaceAll(f.replacement.text())
	}
	return false, true
}

func (i *searchInput) clearCollapsedSelection() {
	from, to, ok := i.box.SelectionBounds()
	if ok && from == to {
		_, _ = i.Handle(term.Event{Type: term.EventKey, Key: term.KeyEsc})
	}
}

func (f *searchFloating) Close() error {
	if f.closed {
		return nil
	}
	f.closed = true
	return f.owner.finish(true)
}

func (f *searchFloating) SeekUp() bool {
	if f.seek == 0 {
		return false
	}
	f.seek--
	return true
}

func (f *searchFloating) SeekDown() bool {
	if f.seek >= f.MaxSeekOffset() {
		return false
	}
	f.seek++
	return true
}

func (f *searchFloating) SeekOffset() int { return f.seek }

func (f *searchFloating) MaxSeekOffset() int {
	return max(0, f.layout.contentHeight-f.layout.height)
}
