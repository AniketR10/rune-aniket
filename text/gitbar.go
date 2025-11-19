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

	"github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
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

// GitBarConfig holds configuration for the auxiliary bar created
// by WithGitBar.
type GitBarConfig struct {
	ScheduleNextTick func(func()) bool
	DelAttr          term.Attributes
	AddAttr          term.Attributes
	DelOverlayAttr   term.Attributes
	AddOverlayAttr   term.Attributes
	Publisher        EventPublisher
}

// WithGitBar wraps the given editor with an git bar. The given buffer,
// and scroll should correspond to the buffer and scroll used by the given editor.
func WithGitBar(
	ed Editor, svc vctrl.Service, handler Handler,
	buf *cell.Buffer, scroll *component.Scroll,
	cfg GitBarConfig,
) Handler {
	ret := new(gitBar)
	ret.buf = buf
	ret.ed = ed
	ret.scroll = scroll
	ret.handler = handler
	ret.vhandler.C = handler
	ret.scheduleNextTick = cfg.ScheduleNextTick
	ret.svc = svc

	if cfg.DelAttr == (term.Attributes{}) {
		cfg.DelAttr = term.Attributes{Fg: tcell.ColorWhite, Bg: tcell.ColorRed}
	}
	if cfg.AddAttr == (term.Attributes{}) {
		cfg.AddAttr = term.Attributes{Fg: tcell.ColorWhite, Bg: tcell.ColorGreen}
	}
	if cfg.DelOverlayAttr == (term.Attributes{}) {
		cfg.DelOverlayAttr = term.Attributes{Bg: tcell.ColorMaroon}
	}
	if cfg.AddOverlayAttr == (term.Attributes{}) {
		cfg.AddOverlayAttr = term.Attributes{Bg: tcell.ColorGreen}
	}
	ret.delAttr = cfg.DelAttr
	ret.addAttr = cfg.AddAttr

	b := new(cell.Buffer)
	b.InitPerformance(1, buf.Rows(), 1, ' ')

	ret.bar = new(component.Scroll)
	ret.bar.InitPerformance(b)
	ret.bar.SetOffset(scroll.Offset())
	ret.file = handler.Resource()
	ret.pub = cfg.Publisher

	scroll.Subscribe(ret)
	buf.Subscribe(ret)
	evs := []textapi.EventType{textapi.EventTypeFlush}
	_ = ret.pub.SubscribeEvents(evs, (*gitBarSubscriber)(ret))

	ret.rebuildBar(context.Background())
	return ret
}

// shared amongst bars
const gitLocationsID = "_gitLocID"

// UnwrapGitBar unwraps the underlying handler from
// a Handler returned by WithGitBar. This function
// panics if the given handler was not returned by WithAuxBar.
func UnwrapGitBar(h Handler) Handler {
	// if auxBar passed itself without wrapping to SetLocationList
	// then editor assumes incorrectly that handler will be a git bar.
	if ret, ok := h.(*gitBar); ok {
		return ret.handler
	}
	return h
}

const (
	addIcon = "+"
	delIcon = "-"
)

type gitBar struct {
	ed               Editor
	svc              vctrl.Service
	scheduleNextTick func(func()) bool
	pub              EventPublisher

	file    workspaceapi.URI
	buf     *cell.Buffer
	scroll  *component.Scroll
	handler Handler
	delAttr term.Attributes
	addAttr term.Attributes

	vhandler   handler.Virtual[Handler]
	bar        *component.Scroll
	addLocAttr term.Attributes
	delLocAttr term.Attributes
}

func (b *gitBar) Selection() (string, bool) {
	return b.vhandler.Selection()
}

func (b *gitBar) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return b.vhandler.Cursor()
}

func (b *gitBar) Draw(w term.Writer) {
	b.vhandler.Draw(w)
	b.bar.Draw(w)
}

func (b *gitBar) Man() tui.Manual {
	return b.vhandler.Man()
}

func (b *gitBar) Handle(ev term.Event) (quit, handled bool) {
	// TODO handle mouse events
	//fold := ev.Type == term.EventMouse && ev.Key == term.MouseLeft &&
	//	ev.MouseX >= 0 && ev.MouseX < 1
	return b.vhandler.Handle(ev)
}

func (b *gitBar) Resize(width, height int) {
	const minSpaceForMain = 4
	barWidth := 2
	if width < minSpaceForMain+barWidth {
		barWidth = 0
	}
	b.bar.Resize(barWidth, height)
	b.vhandler.Move(term.Coordinates{X: barWidth})
	b.vhandler.Resize(width-barWidth, height)
}

func (b *gitBar) Close() (ret error) {
	ok, err := b.pub.UnsubscribeEvents((*gitBarSubscriber)(b))
	if err != nil {
		ret = multierror.Append(ret, err)
	} else if !ok {
		ret = multierror.Append(ret, errors.New("could not unsubscribe auxiliary bar"))
	}
	if err := b.vhandler.C.Close(); err != nil {
		ret = multierror.Append(ret, err)
	}
	return
}

func (b *gitBar) rebuildBar(ctx context.Context) {
	b.bar.Buffer().ResetPerformance()
	uri := b.handler.Resource()

	go debug.CapturePanicReport(func() {
		filediff, err := b.svc.Diff(ctx, uri)
		if err != nil {
			b.log(log.ErrorLevel, "compute diff: %v", err)
			return
		}
		b.log(log.TraceLevel, "computed diff: %v", filediff)

		ll := filediff.LocationList(b.delLocAttr, b.addLocAttr)
		b.scheduleNextTick(func() {
			for loc, ok := ll.Current(); ok; loc, ok = ll.Next() {
				from := term.Coordinates{Y: loc.From.Y}
				to := term.Coordinates{Y: loc.To.Y}
				if from == to {
					at, ok := b.scrollToBarCoordinates(from)
					if !ok { // hidden
						continue
					}
					b.bar.Buffer().DeleteCell(at)
					b.bar.Buffer().InsertStringWithAttr(at, delIcon, b.delAttr)
					continue
				}

				for y := from.Y; y < to.Y; y++ {
					at, ok := b.scrollToBarCoordinates(term.Coordinates{Y: y})
					if !ok { // hidden
						continue
					}
					icon := addIcon
					b.bar.Buffer().DeleteCell(at)
					b.bar.Buffer().InsertStringWithAttr(at, icon, b.addAttr)
				}
			}
			_ = b.ed.SetLocationList(b, textapi.LocationPriorityInfo, gitLocationsID, ll)
		})
	})
}

type gitBarSubscriber gitBar

func (b *gitBarSubscriber) Handle(ctx context.Context, ev textapi.Event) bool {
	if !ev.URI.Equal(b.file) || ev.Type != textapi.EventTypeFlush {
		return false
	}
	(*gitBar)(b).log(log.TraceLevel, "received event: %s", ev.Type.String())
	(*gitBar)(b).rebuildBar(ctx)
	return false
}

func (b *gitBar) OnDidSeek(_, to term.Coordinates) {
	b.bar.SetOffset(to)
}

func (b *gitBar) OnWillSeek(_ term.Coordinates) {
}

func (b *gitBar) OnHide(start, end int) {
	b.rebuildBar(context.Background())
}

func (b *gitBar) OnVisible(start int) {
	b.rebuildBar(context.Background())
}

func (b *gitBar) OnWillEdit(
	ctx context.Context, from, to term.Coordinates, str string,
) {
}

func (b *gitBar) OnDidEdit(
	ctx context.Context, start, end term.Coordinates, old string,
) {
}

func (b *gitBar) scrollToBarCoordinates(pos term.Coordinates) (ret term.Coordinates, ok bool) {
	pos, ok = b.scroll.ScrollToWindowCoordinates(pos)
	if !ok {
		return
	}
	ret = pos
	ret.Y += b.scroll.Offset().Y
	return
}

func (b *gitBar) SeekUp() bool {
	return b.handler.SeekUp()
}

func (b *gitBar) SeekDown() bool {
	return b.handler.SeekDown()
}

func (b *gitBar) SeekOffset() int {
	return b.handler.SeekOffset()
}

func (b *gitBar) MaxSeekOffset() int {
	return b.handler.MaxSeekOffset()
}

func (b *gitBar) Resource() workspaceapi.URI {
	return b.handler.Resource()
}

func (b *gitBar) SetWrap(wrap bool) {
	b.handler.SetWrap(wrap)
}

func (b *gitBar) ShowCommandBar(show bool) {
	b.handler.ShowCommandBar(show)
}

func (b *gitBar) SetCursorAtScroll(pos term.Coordinates) bool {
	return b.handler.SetCursorAtScroll(pos)
}

func (b *gitBar) log(level log.Level, msg string, args ...any) {
	if !log.IsLevelEnabled(level) {
		return
	}
	log.WithField(logging.KeyClass, "text.gitBar").Logf(level, msg, args...)
}
