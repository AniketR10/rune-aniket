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
	"errors"
	"math"
	"strconv"

	"github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/blue/logging"
	"github.com/unstablebuild/tcell/v3"
	"unstable.build/go-tui"
	"unstable.build/go-tui/api/textapi"
	"unstable.build/go-tui/api/workspaceapi"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/ide/vctrl"
	"unstable.build/go-tui/term"
)

// AuxBarConfig holds configuration for the auxiliary bar created
// by WithAuxBar.
type AuxBarConfig struct {
	GitEnabled          bool
	LinesEnabled        bool
	FoldsEnabled        bool
	AbsoluteLines       bool
	HighlightCursor     bool
	ScheduleNextTick    func(func()) bool
	DelAttr             term.Attributes
	AddAttr             term.Attributes
	DelOverlayAttr      term.Attributes
	AddOverlayAttr      term.Attributes
	HighlightCursorAttr term.Attributes
	LineNumberAttr      term.Attributes
	// Publisher is used to subscribe to EventTypeFlush events.
	// It's optional if git is disabled by setting GitEnabled to false.
	Publisher EventPublisher
	// CommandRegistry is used to register commands to the parent workspace.
	// It's optional if git is disabled by setting GitEnabled to false.
	CommandRegistry FileCommandRegistry
	// Service is used to calculate diffs. It's optional if git is
	// disabled by setting GitEnabled to false.
	Service vctrl.Service
}

// WithAuxBar wraps the given editor with an auxiliary bar. The given buffer,
// and scroll should correspond to the buffer and scroll used by the given editor.
func WithAuxBar(
	handler Handler, buf *cell.Buffer,
	scroll *component.Scroll, config AuxBarConfig,
) Handler {
	if config.ScheduleNextTick == nil ||
		(config.GitEnabled && (config.Publisher == nil || config.CommandRegistry == nil)) {
		panic("auxbar configuration is missing key dependencies")
	}
	ret := new(auxBar)
	ret.pub = config.Publisher
	ret.buf = buf
	ret.file = handler.Resource()
	ret.scroll = scroll
	ret.Handler = handler
	ret.vhandler.C = handler
	ret.registry = config.CommandRegistry
	ret.scheduleNextTick = config.ScheduleNextTick
	ret.foldsEnabled = config.FoldsEnabled
	ret.gitEnabled = config.GitEnabled
	ret.linesEnabled = config.LinesEnabled
	ret.absoluteLines = config.AbsoluteLines
	ret.highlightCursor = config.HighlightCursor
	ret.folds = make(map[term.Coordinates]term.Coordinates)
	ret.setLinesWidth(10 /* good height for calculating width of lines */)
	ret.svc = config.Service
	if config.DelAttr == (term.Attributes{}) {
		config.DelAttr = term.Attributes{Fg: tcell.ColorMaroon}
	}
	if config.AddAttr == (term.Attributes{}) {
		config.AddAttr = term.Attributes{Fg: tcell.ColorGreen}
	}
	ret.delAttr = config.DelAttr
	ret.addAttr = config.AddAttr
	if config.DelOverlayAttr == (term.Attributes{}) {
		config.DelOverlayAttr = term.Attributes{Bg: tcell.ColorMaroon}
	}
	if config.AddOverlayAttr == (term.Attributes{}) {
		config.AddOverlayAttr = term.Attributes{Bg: tcell.ColorGreen}
	}
	if config.HighlightCursorAttr == (term.Attributes{}) {
		config.HighlightCursorAttr = term.Attributes{
			Fg:    tcell.ColorWhite,
			Bg:    tcell.ColorGray,
			Attrs: tcell.AttrBold,
		}
	}
	ret.cursorAttr = config.HighlightCursorAttr
	if config.LineNumberAttr == (term.Attributes{}) {
		config.LineNumberAttr = term.Attributes{Fg: tcell.ColorGray}
	}
	ret.barLineAttr = config.LineNumberAttr
	ret.config = config

	b := new(cell.Buffer)
	b.InitPerformance(1, buf.Rows(), foldsWidth+ret.linesWidth, ' ')

	ret.bar = new(component.Scroll)
	ret.bar.InitPerformance(b)
	ret.bar.SetOffset(term.Coordinates{Y: scroll.Offset().Y})
	ret.bar.Attributes.Bg = ret.barLineAttr.Bg
	ret.bar.Attributes.Fg = ret.barLineAttr.Fg

	scroll.Subscribe(ret)
	buf.Subscribe(ret)

	evs := []textapi.EventType{textapi.EventTypeFlush}
	if ret.gitEnabled {
		_ = ret.pub.SubscribeEvents(evs, (*auxBarSubscriber)(ret))
		for _, cmd := range gitCommands {
			// this could fail if gitbar is also enabled
			_ = ret.registry.SubscribeCommandForFile(ret.file, cmd, ret)
		}
	}

	ret.rebuildBar(context.Background())
	return ret
}

const foldsWidth = 2

const (
	hiddenFoldIcon  = ''
	visibleFoldIcon = ''
)

type auxBar struct {
	Handler
	pub              EventPublisher
	scheduleNextTick func(func()) bool
	file             workspaceapi.URI
	svc              vctrl.Service
	registry         FileCommandRegistry
	foldsEnabled     bool
	linesEnabled     bool
	gitEnabled       bool
	absoluteLines    bool
	highlightCursor  bool
	config           AuxBarConfig

	buf    *cell.Buffer
	scroll *component.Scroll

	closed      bool
	vhandler    handler.Virtual[Handler]
	bar         *component.Scroll
	folds       map[term.Coordinates]term.Coordinates
	linesWidth  int
	height      int
	barWidth    int
	prevCursor  term.Coordinates
	delLocAttr  term.Attributes
	addLocAttr  term.Attributes
	delAttr     term.Attributes
	addAttr     term.Attributes
	cursorAttr  term.Attributes
	barLineAttr term.Attributes
}

func (b *auxBar) Selection() (string, bool) {
	return b.vhandler.Selection()
}

func (b *auxBar) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return b.vhandler.Cursor()
}

func (b *auxBar) Draw(w term.Writer) {
	b.vhandler.Draw(w)
	b.bar.Draw(w)
	if b.highlightCursor {
		y := b.prevCursor.Y
		for x := range b.barWidth {
			w.UnionAttributes(term.Coordinates{Y: y, X: x}, b.cursorAttr)
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
		quit, handled = b.vhandler.Handle(ev)
		if !handled {
			return
		}
		prevCursor := b.prevCursor
		b.prevCursor, _, _ = b.vhandler.Cursor()
		if b.linesEnabled && !b.absoluteLines && prevCursor.Y != b.prevCursor.Y {
			b.rebuildBar(context.Background())
		}
		return
	}

	posAtWindow := term.Coordinates{Y: ev.MouseY, X: ev.MouseX}
	posAtBar := b.windowToBarCoordinates(posAtWindow)
	posAtScroll := b.scroll.WindowToScrollCoordinates(posAtWindow)
	posAtScroll.X = 0
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

func (b *auxBar) HandleCommand(ctx context.Context, cmd textapi.Command) error {
	switch cmd.Name {
	case commandToggleOverlay:
		if b.addLocAttr == (term.Attributes{}) {
			b.addLocAttr = b.config.AddOverlayAttr
		} else {
			b.addLocAttr = term.Attributes{}
		}
		if b.delLocAttr == (term.Attributes{}) {
			b.delLocAttr = b.config.DelOverlayAttr
		} else {
			b.delLocAttr = term.Attributes{}
		}
		b.rebuildBar(ctx)
		return nil
	default:
		return nil
	}
}

func (b *auxBar) Complete(ctx context.Context, cmd textapi.Command) (
	iterator.Iterator[string], string, error,
) {
	return iterator.Empty[string](), "", nil
}

func (b *auxBar) Close() (ret error) {
	if b.closed {
		return nil
	}
	b.closed = true
	if b.gitEnabled {
		ok, err := b.pub.UnsubscribeEvents((*auxBarSubscriber)(b))
		if err != nil {
			ret = multierror.Append(ret, err)
		} else if !ok {
			ret = multierror.Append(ret, errors.New("could not unsubscribe auxiliary bar"))
		}
		for _, cmd := range gitCommands {
			// could return error if git bar is enabled
			_ = b.registry.UnsubscribeCommandForFile(b.file, cmd.Name)
		}
	}
	if err := b.vhandler.C.Close(); err != nil {
		ret = multierror.Append(ret, err)
	}
	return
}

type auxBarSubscriber auxBar

func (b *auxBarSubscriber) Handle(ctx context.Context, ev textapi.Event) bool {
	if !ev.URI.Equal(b.file) || ev.Type != textapi.EventTypeFlush {
		return false
	}
	(*auxBar)(b).log(log.TraceLevel, "received event: %s", ev.Type.String())
	(*auxBar)(b).rebuildBar(ctx)
	return false
}

func (b *auxBar) foldAt(pos term.Coordinates) (folded, ok bool) {
	cells := b.bar.Buffer().RawCells()
	if pos.Y >= len(cells) {
		return
	}
	if pos.X >= len(cells[pos.Y]) {
		return
	}
	switch cells[pos.Y][pos.X].Ch {
	case hiddenFoldIcon:
		ok = true
		folded = true
	case visibleFoldIcon:
		ok = true
	default:
	}
	return
}

func (b *auxBar) rebuildBar(ctx context.Context) {
	b.bar.Buffer().ResetPerformance()
	if b.linesEnabled && b.absoluteLines {
		b.rebuildLinesAbsolute(ctx)
	} else if b.linesEnabled {
		b.rebuildLinesRelative(ctx)
	}
	if b.linesEnabled && b.gitEnabled {
		b.rebuildGit(ctx)
	}
	if b.foldsEnabled {
		b.rebuildFolds(ctx)
	}
}

func (b *auxBar) rebuildGit(ctx context.Context) {
	uri := b.Handler.Resource()
	go debug.CapturePanicReport(func() {
		// perform diff in a separate gouroutine in case
		// scheme is remote and diff performs network I/O.
		filediff, err := b.svc.Diff(ctx, uri)
		if err != nil {
			b.log(log.ErrorLevel, "compute diff: %v", err)
			return
		}
		b.log(log.TraceLevel, "computed diff: %v", filediff)

		ll := filediff.LocationList(b.delLocAttr, b.addLocAttr)

		// serialize back into event loop
		b.scheduleNextTick(func() {
			cells := b.bar.Buffer().RawCells()
			for loc, ok := ll.Current(); ok; loc, ok = ll.Next() {
				from := term.Coordinates{Y: loc.From.Y}
				to := term.Coordinates{Y: loc.To.Y}
				if from == to {
					at, ok := b.scrollToBarCoordinates(from)
					// hidden || lines removed between ticks
					if !ok || at.Y >= len(cells) {
						continue
					}
					for x := 0; x < b.linesWidth; x++ {
						if x >= len(cells[at.Y]) {
							break
						}
						cells[at.Y][x].Attributes = term.AttributesUnion(
							cells[at.Y][x].Attributes, b.delAttr)
					}
					continue
				}

				for y := from.Y; y < to.Y; y++ {
					at, ok := b.scrollToBarCoordinates(term.Coordinates{Y: y})
					// hidden || lines removed between ticks
					if !ok || at.Y >= len(cells) {
						continue
					}
					for x := 0; x < b.linesWidth; x++ {
						if x >= len(cells[at.Y]) {
							break
						}
						cells[at.Y][x].Attributes = term.AttributesUnion(
							cells[at.Y][x].Attributes, b.addAttr)
					}
				}
			}
			b.Handler.SetLocationList(textapi.LocationPriorityInfo, gitLocationsID, ll)
		})
	})
}

func (b *auxBar) rebuildLinesAbsolute(ctx context.Context) {
	for y := range b.buf.View().Rows() {
		from, ok := b.scrollToBarCoordinates(term.Coordinates{Y: y})
		if !ok {
			// inside hidden block
			continue
		}
		to := from
		if y < b.bar.Buffer().Rows() {
			cols := b.bar.Buffer().Columns(y)
			if cols > 0 {
				to.X = cols - 1
			}
		}
		number := strconv.Itoa(y + 1)
		b.bar.Buffer().EditWithAttr(ctx, from, to, number, b.barLineAttr)
	}
}

func (b *auxBar) rebuildLinesRelative(ctx context.Context) {
	cursorAtWindow := b.windowToBarCoordinates(b.prevCursor)
	for y := range b.buf.View().Rows() {
		n := int(math.Abs(float64(cursorAtWindow.Y - y)))
		var number string
		if n == 0 {
			cursorAtScroll := b.scroll.WindowToScrollCoordinates(b.prevCursor)
			number = strconv.Itoa(cursorAtScroll.Y + 1)
		} else {
			number = strconv.Itoa(n)
		}
		from := term.Coordinates{Y: y}
		to := term.Coordinates{Y: y}
		if y < b.bar.Buffer().Rows() {
			cols := b.bar.Buffer().Columns(y)
			if cols > 0 {
				to.X = cols - 1
			}
		}
		b.bar.Buffer().EditWithAttr(ctx, from, to, number, b.barLineAttr)
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
				b.bar.Buffer().InsertWithAttr(from, icon, b.barLineAttr)
			}
			if err := folds.Err(); err != nil {
				b.log(log.ErrorLevel, "error rebuilding auxiliary bar: %v", err)
			}
		})
	})
}

func (b *auxBar) OnDidSeek(_, to term.Coordinates) {
	b.bar.SetOffset(term.Coordinates{Y: to.Y})
	if b.linesEnabled && !b.absoluteLines {
		b.rebuildBar(context.Background())
	}
}

func (b *auxBar) OnWillSeek(_ term.Coordinates) {
}

func (b *auxBar) OnHide(start, end int) {
	b.rebuildBar(context.Background())
}

func (b *auxBar) OnVisible(start int) {
	b.rebuildBar(context.Background())
}

func (b *auxBar) OnWillEdit(
	ctx context.Context, from, to term.Coordinates, str string,
) {
}

func (b *auxBar) OnDidEdit(
	ctx context.Context, start, end term.Coordinates, old string,
) {
	b.setLinesWidth(b.height)
	b.rebuildBar(ctx)
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
	// this method is used for cursor coordinates,
	// which hovers across both bar and content
	if pos.X >= b.barWidth {
		pos.X += b.scroll.Offset().X
	}
	return pos
}

func (b *auxBar) scrollToBarCoordinates(pos term.Coordinates) (ret term.Coordinates, ok bool) {
	pos, ok = b.scroll.ScrollToWindowCoordinates(pos)
	if !ok {
		return
	}
	ret = pos
	ret.Y += b.scroll.Offset().Y
	ret.X += b.scroll.Offset().X
	return
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
