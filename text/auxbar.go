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

package text

import (
	"context"
	"math"
	"strconv"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/blue/logging"
	"github.com/unstablebuild/tcell/v3"
	"unstable.build/go-tui"
	"unstable.build/go-tui/api/workspaceapi"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/term"
)

// WithAuxBar wraps the given editor with an auxiliary bar, which uses exactly
// one column of the available space, to draw an auxiliary bar. The given buffer,
// and scroll should correspond to the buffer and scroll used by the given editor.
func WithAuxBar(
	handler Handler, buf *cell.Buffer, scroll *component.Scroll,
	foldsEnabled, linesEnabled, absoluteLines, highlightCursor bool,
	scheduleNextTick func(func()) bool,
) Handler {
	ret := new(auxBar)
	ret.buf = buf
	ret.scroll = scroll
	ret.editor = handler
	ret.vhandler.C = handler
	ret.scheduleNextTick = scheduleNextTick
	ret.foldsEnabled = foldsEnabled
	ret.linesEnabled = linesEnabled
	ret.absoluteLines = absoluteLines
	ret.highlightCursor = highlightCursor
	ret.folds = make(map[term.Coordinates]term.Coordinates)
	ret.setLinesWidth(10 /* good height for calculating width of lines */)

	b := new(cell.Buffer)
	b.InitPerformance(1, buf.Rows(), foldsWidth+ret.linesWidth, ' ')

	ret.bar = new(component.Scroll)
	ret.bar.InitPerformance(b)
	ret.bar.SetOffset(scroll.Offset())

	scroll.Subscribe(ret)
	buf.Subscribe(ret)

	ret.rebuildBar(context.Background(), term.Coordinates{})
	return ret
}

const foldsWidth = 2

var (
	fgAttr = term.Attributes{Fg: tcell.ColorGray}
	bgAttr = term.Attributes{Fg: tcell.ColorWhite, Bg: tcell.ColorGray}
)

// UnwrapAuxBar unwraps the underlying handler from
// a Handler returned by WithAuxBar. This function
// panics if the given handler was not returned by WithAuxBar.
func UnwrapAuxBar(h Handler) Handler {
	return h.(*auxBar).editor
}

const (
	hiddenFoldIcon  = ''
	visibleFoldIcon = ''
)

type auxBar struct {
	scheduleNextTick func(func()) bool
	foldsEnabled     bool
	linesEnabled     bool
	absoluteLines    bool
	highlightCursor  bool

	buf    *cell.Buffer
	scroll *component.Scroll
	editor Handler

	vhandler   handler.Virtual[Handler]
	bar        *component.Scroll
	folds      map[term.Coordinates]term.Coordinates
	linesWidth int
	height     int
	barWidth   int
	prevCursor term.Coordinates
}

func (b *auxBar) Selection() (string, bool) {
	return b.vhandler.Selection()
}

func (b *auxBar) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return b.vhandler.Cursor()
}

func (b *auxBar) Draw(w term.Writer) {
	cursor, _, _ := b.vhandler.Cursor()
	if b.linesEnabled && cursor.Y != b.prevCursor.Y {
		b.rebuildBar(w.Context(), cursor)
	}
	b.vhandler.Draw(w)
	b.bar.Draw(w)
	if b.highlightCursor {
		y := b.prevCursor.Y
		for x := range b.barWidth {
			w.UnionAttributes(term.Coordinates{Y: y, X: x}, bgAttr)
		}
	}
}

func (b *auxBar) Man() tui.Manual {
	return b.vhandler.Man()
}

func (b *auxBar) Handle(ev term.Event) (quit, handled bool) {
	fold := b.foldsEnabled && ev.Type == term.EventMouse && ev.Key == term.MouseLeft &&
		ev.MouseX >= b.linesWidth && ev.MouseX < b.linesWidth+foldsWidth
	if !fold {
		return b.vhandler.Handle(ev)
	}

	posAtWindow := term.Coordinates{Y: ev.MouseY}
	posAtBar := b.windowToBarCoordinates(posAtWindow)
	posAtScroll := b.scroll.WindowToScrollCoordinates(posAtWindow)
	folded, ok := b.foldAt(posAtBar)
	b.log(log.TraceLevel, "received mouse click at bar,"+
		" win pos: %+v, bar pos: %+v, scroll pos: %+v, folded: %t, ok: %t",
		posAtWindow, posAtBar, posAtScroll, folded, ok)
	if ok && folded {
		handled = b.scroll.MarkVisible(posAtScroll.Y)
	} else if ok {
		if end, ok := b.folds[posAtScroll]; ok {
			handled = b.scroll.MarkHidden(posAtScroll.Y, end.Y)
		} else {
			b.log(log.DebugLevel, "received mouse click at drawn fold: %+v,"+
				" but no fold in map", posAtScroll)
		}
	}
	return
}

func (b *auxBar) Resize(width, height int) {
	b.height = height
	b.setLinesWidth(b.height)
	b.barWidth = b.linesWidth
	if b.foldsEnabled {
		b.barWidth += foldsWidth
	}
	const padding = 2
	if width < padding+b.barWidth {
		b.barWidth = 0
	}
	b.bar.Resize(b.barWidth, height)
	b.vhandler.Move(term.Coordinates{X: b.barWidth})
	b.vhandler.Resize(width-b.barWidth, height)
}

func (b *auxBar) Close() error {
	return b.vhandler.C.Close()
}

func (b *auxBar) foldAt(pos term.Coordinates) (folded, ok bool) {
	cells := b.bar.Buffer().RawCells()
	if pos.Y >= len(cells) {
		return
	}
	if len(cells[pos.Y]) == 0 {
		return
	}
	switch cells[pos.Y][b.linesWidth].Ch {
	case hiddenFoldIcon:
		ok = true
		folded = true
	case visibleFoldIcon:
		ok = true
	default:
	}
	return
}

func (b *auxBar) rebuildBar(ctx context.Context, cursor term.Coordinates) {
	b.prevCursor = cursor
	b.bar.Buffer().ResetPerformance()
	if b.linesEnabled && b.absoluteLines {
		b.rebuildLinesAbsolute(ctx)
	} else if b.linesEnabled {
		b.rebuildLinesRelative(ctx)
	}
	if b.foldsEnabled {
		b.rebuildFolds(ctx)
	}
}

func (b *auxBar) rebuildLinesAbsolute(ctx context.Context) {
	for y := range b.buf.View().Rows() {
		number := strconv.Itoa(y + 1)
		from := term.Coordinates{Y: y}
		to := term.Coordinates{Y: y}
		if y < b.bar.Buffer().Rows() {
			cols := b.bar.Buffer().Columns(y)
			if cols > 0 {
				to.X = cols - 1
			}
		}
		b.bar.Buffer().EditWithAttr(ctx, from, to, number, fgAttr)
	}
}

func (b *auxBar) rebuildLinesRelative(ctx context.Context) {
	cursorAtWindow := b.windowToBarCoordinates(b.prevCursor)
	for y := range b.buf.View().Rows() {
		number := strconv.Itoa(int(math.Abs(float64(cursorAtWindow.Y - y))))
		if number == "0" {
			cursorAtScroll := b.scroll.WindowToScrollCoordinates(b.prevCursor)
			number = strconv.Itoa(cursorAtScroll.Y + 1)
		}
		from := term.Coordinates{Y: y}
		to := term.Coordinates{Y: y}
		if y < b.bar.Buffer().Rows() {
			cols := b.bar.Buffer().Columns(y)
			if cols > 0 {
				to.X = cols - 1
			}
		}
		b.bar.Buffer().EditWithAttr(ctx, from, to, number, fgAttr)
	}
}

func (b *auxBar) rebuildFolds(ctx context.Context) {
	svc, ok := b.buf.View().(foldsService)
	if !ok {
		return
	}

	folds, ok := svc.Folds()
	if !ok {
		return
	}

	go debug.CapturePanicReport(func() {
		folds, isEmpty := iterator.IsEmpty(context.Background(), folds)
		defer folds.Close()
		if isEmpty {
			return
		}
		// once first fold has been returned, this should not block on I/O anymore
		// run on next loop tick, so we don't need to worry about synchronization
		b.scheduleNextTick(func() {
			clear(b.folds)
			for {
				fold, ok := folds.Next(ctx)
				if !ok {
					break
				}

				// we're not interested in these
				if fold.Start.Y >= fold.End.Y {
					continue
				}

				// avoid ambiguity at bar
				fold.Start.X = 0
				if end, exists := b.folds[fold.Start]; exists && end.Y > fold.End.Y {
					continue
				}
				b.folds[fold.Start] = fold.End

				// convert folds which are scroll coordinates
				// to window coordinates
				foldStart, startOk := b.scroll.ScrollToWindowCoordinates(fold.Start)
				foldEnd, endOk := b.scroll.ScrollToWindowCoordinates(fold.End)
				if foldStart.Y != foldEnd.Y && (!startOk || !endOk) {
					// inside hidden block
					continue
				}

				// convert to a scroll coordinates with hidden lines taken into account
				offset := b.scroll.Offset()
				foldStart.Y += offset.Y
				foldEnd.Y += offset.Y
				foldStart.X = 0

				if foldStart.Y < 0 {
					panic("invalid aux bar coordinates after conversion")
				}

				var icon rune
				if foldStart.Y == foldEnd.Y {
					icon = hiddenFoldIcon
				} else {
					icon = visibleFoldIcon
				}
				from := foldStart
				from.X += b.linesWidth
				to := from
				to.X++ // replace
				b.bar.Buffer().Edit(ctx, from, to, "")
				b.bar.Buffer().InsertWithAttr(from, icon, fgAttr)
			}
			if err := folds.Err(); err != nil {
				b.log(log.ErrorLevel, "error rebuilding auxiliary bar: %v", err)
			}
		})
	})
}

func (b *auxBar) OnDidSeek(_, to term.Coordinates) {
	b.bar.SetOffset(to)
	if b.linesEnabled {
		cursor, _, _ := b.vhandler.Cursor()
		b.rebuildBar(context.Background(), cursor)
	}
}

func (b *auxBar) OnWillSeek(_ term.Coordinates) {
}

func (b *auxBar) OnHide(start, end int) {
	cursor, _, _ := b.vhandler.Cursor()
	b.rebuildBar(context.Background(), cursor)
}

func (b *auxBar) OnVisible(start int) {
	cursor, _, _ := b.vhandler.Cursor()
	b.rebuildBar(context.Background(), cursor)
}

func (b *auxBar) OnWillEdit(
	ctx context.Context, from, to term.Coordinates, str string,
) {
}

func (b *auxBar) OnDidEdit(
	ctx context.Context, start, end term.Coordinates, old string,
) {
	b.setLinesWidth(b.height)
	cursor, _, _ := b.vhandler.Cursor()
	b.rebuildBar(ctx, cursor)
}

func (b *auxBar) setLinesWidth(height int) {
	if b.linesEnabled && b.absoluteLines {
		b.linesWidth = len(strconv.Itoa(b.buf.View().Rows())) + 1
	} else if b.linesEnabled {
		b.linesWidth = len(strconv.Itoa(height)) + 1
	} else {
		b.linesWidth = 0
	}
}

func (b *auxBar) windowToBarCoordinates(pos term.Coordinates) term.Coordinates {
	pos.Y += b.scroll.Offset().Y
	return pos
}

func (b *auxBar) SeekUp() bool {
	return b.editor.SeekUp()
}

func (b *auxBar) SeekDown() bool {
	return b.editor.SeekDown()
}

func (b *auxBar) SeekOffset() int {
	return b.editor.SeekOffset()
}

func (b *auxBar) MaxSeekOffset() int {
	return b.editor.MaxSeekOffset()
}

func (b *auxBar) Resource() workspaceapi.URI {
	return b.editor.Resource()
}

func (b *auxBar) SetWrap(wrap bool) {
	b.editor.SetWrap(wrap)
}

func (b *auxBar) ShowCommandBar(show bool) {
	b.editor.ShowCommandBar(show)
}

func (b *auxBar) SetCursorAtScroll(pos term.Coordinates) bool {
	return b.editor.SetCursorAtScroll(pos)
}

func (b *auxBar) log(level log.Level, msg string, args ...any) {
	if !log.IsLevelEnabled(level) {
		return
	}
	log.WithField(logging.KeyClass, "text.auxBar").Logf(level, msg, args...)
}

type foldsService interface {
	Folds() (iterator.Iterator[term.Range], bool)
}
