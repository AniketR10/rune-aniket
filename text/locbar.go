// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package text

import (
	"context"
	"errors"
	"sort"

	"github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/blue/logging"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/handler"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/term/graphemecluster"
	"unstable.build/rune/cell"
	"unstable.build/rune/component"
	"unstable.build/rune/debug"
	"unstable.build/rune/ide/vctrl"
)

// IconsBarConfig holds configuration for the icons bar created
// by WithIconsBar.
type IconsBarConfig struct {
	ScheduleNextTick func(func()) bool
	DelAttr          term.Attributes
	AddAttr          term.Attributes
	DelOverlayAttr   term.Attributes
	AddOverlayAttr   term.Attributes
	Publisher        EventPublisher
	CommandRegistry  FileCommandRegistry
}

// WithIconsBar wraps the given editor with an icons bar. The given buffer,
// and scroll should correspond to the buffer and scroll used by the given editor.
func WithIconsBar(
	svc vctrl.Service, gitEnabled bool, handler Handler,
	buf *cell.Buffer, scroll *component.Scroll,
	cfg IconsBarConfig,
) Handler {
	if cfg.ScheduleNextTick == nil {
		panic("icons bar configuration is missing key dependencies")
	}
	if gitEnabled && (cfg.CommandRegistry == nil || cfg.Publisher == nil) {
		panic("icons bar configuration is missing git dependencies")
	}
	ret := new(iconsBar)
	ret.buf = buf
	ret.scroll = scroll
	ret.Handler = handler
	ret.vhandler.C = handler
	ret.scheduleNextTick = cfg.ScheduleNextTick
	ret.svc = svc
	ret.gitEnabled = gitEnabled

	if cfg.DelAttr == (term.Attributes{}) {
		cfg.DelAttr = term.Attributes{Fg: term.ColorWhite, Bg: term.ColorRed}
	}
	if cfg.AddAttr == (term.Attributes{}) {
		cfg.AddAttr = term.Attributes{Fg: term.ColorWhite, Bg: term.ColorGreen}
	}
	if cfg.DelOverlayAttr == (term.Attributes{}) {
		cfg.DelOverlayAttr = term.Attributes{Bg: term.ColorMaroon}
	}
	if cfg.AddOverlayAttr == (term.Attributes{}) {
		cfg.AddOverlayAttr = term.Attributes{Bg: term.ColorGreen}
	}
	ret.delAttr = cfg.DelAttr
	ret.addAttr = cfg.AddAttr

	b := new(cell.Buffer)
	b.InitPerformance(buf.Rows(), 3, ' ')

	ret.bar = new(component.Scroll)
	ret.bar.InitPerformance(b)
	ret.bar.SetOffset(term.Coordinates{Y: scroll.Offset().Y})
	ret.file = handler.Resource()
	ret.pub = cfg.Publisher
	ret.registry = cfg.CommandRegistry
	ret.config = cfg
	ret.iconColumns = 0
	ret.iconsByID = make(map[string][]iconsBarLineIcon)
	ret.locationBufByID = make(map[string]locationListBuffers)
	ret.iconBufByID = make(map[string][]iconsBarLineIcon)

	scroll.Subscribe(ret)
	buf.Subscribe(ret)
	ret.dirty = true
	ret.cancelBuild = func() {}
	if ret.gitEnabled {
		evs := []textapi.EventType{textapi.EventTypeFlush, textapi.EventTypeFocus}
		_ = ret.pub.SubscribeEvents(evs, (*iconsBarSubscriber)(ret))
		for _, cmd := range gitCommands {
			// could return error if auxbar is enabled
			_ = ret.registry.SubscribeCommandForFile(ret.file, cmd, ret)
		}
	}

	if ret.dirty {
		ret.rebuildBar(context.Background())
	}
	return ret
}

// WithGitBar wraps the given editor with a git-backed icons bar.
// Deprecated: use WithIconsBar.
func WithGitBar(
	svc vctrl.Service, handler Handler,
	buf *cell.Buffer, scroll *component.Scroll,
	cfg IconsBarConfig,
) Handler {
	return WithIconsBar(svc, true, handler, buf, scroll, cfg)
}

const (
	// shared amongst bars
	gitLocationsID       = "gitchange"
	addIcon              = "+"
	delIcon              = "-"
	commandToggleOverlay = "gittoggleoverlay"
)

var (
	commandToggleOverlayManual = textapi.CommandManual{
		Name:    commandToggleOverlay,
		Summary: "Shows or hides the git diff hunks overlay.",
	}
	gitCommands = []textapi.CommandManual{
		commandToggleOverlayManual,
	}
)

type iconsBar struct {
	Handler
	svc              vctrl.Service
	gitEnabled       bool
	scheduleNextTick func(func()) bool
	pub              EventPublisher
	registry         FileCommandRegistry
	config           IconsBarConfig

	file    workspaceapi.URI
	buf     *cell.Buffer
	scroll  *component.Scroll
	delAttr term.Attributes
	addAttr term.Attributes

	dirty           bool
	cancelBuild     func()
	closed          bool
	vhandler        handler.Virtual[Handler]
	bar             *component.Scroll
	addLocAttr      term.Attributes
	delLocAttr      term.Attributes
	width           int
	height          int
	iconColumns     int
	iconsByID       map[string][]iconsBarLineIcon
	locationBufByID map[string]locationListBuffers
	iconBufByID     map[string][]iconsBarLineIcon
}

type locationListBuffers struct {
	published []textapi.Location
	spare     []textapi.Location
}

func (b *iconsBar) Selection() (string, bool) {
	return b.vhandler.Selection()
}

func (b *iconsBar) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return b.vhandler.Cursor()
}

func (b *iconsBar) Draw(w term.Writer) {
	b.vhandler.Draw(w)
	b.bar.Draw(w)
	if b.bar.Width() != 0 {
		bg := term.Attributes{Bg: b.scroll.Attributes.Bg}
		for y := range b.bar.SizeHeight() {
			for x := range b.bar.Width() {
				w.UnionAttributes(term.Coordinates{Y: y, X: x}, bg)
			}
		}
	}
}

func (b *iconsBar) Handle(ev term.Event) (quit, handled bool) {
	// TODO handle mouse events
	//fold := ev.Type == term.EventMouse && ev.Key == term.MouseLeft &&
	//	ev.MouseX >= 0 && ev.MouseX < 1
	return b.vhandler.Handle(ev)
}

func (b *iconsBar) Resize(width, height int) {
	const minSpaceForMain = 4
	b.width = width
	b.height = height
	barWidth := b.barWidth()
	if width < minSpaceForMain+barWidth {
		barWidth = 0
	}
	b.bar.Resize(barWidth, height)
	b.vhandler.Move(term.Coordinates{X: barWidth})
	b.vhandler.Resize(width-barWidth, height)
}

func (b *iconsBar) barWidth() int {
	return max(1, b.iconColumns) + 1
}

// Dimensions satisfies text.Handler: reports the ideal dimensions
// of the inner handler plus the horizontal cells claimed by the
// icons bar.
func (b *iconsBar) Dimensions() (int, int) {
	w, h := b.vhandler.C.Dimensions()
	return w + b.barWidth(), h
}

func (b *iconsBar) HandleCommand(ctx context.Context, cmd textapi.Command) error {
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

func (b *iconsBar) Complete(ctx context.Context, cmd textapi.Command) (
	iterator.Iterator[string], string, error,
) {
	return iterator.Empty[string](), "", nil
}

func (b *iconsBar) Close() (ret error) {
	if b.closed {
		return nil
	}
	b.closed = true
	if b.gitEnabled {
		ok, err := b.pub.UnsubscribeEvents((*iconsBarSubscriber)(b))
		if err != nil {
			ret = multierror.Append(ret, err)
		} else if !ok {
			ret = multierror.Append(ret, errors.New("could not unsubscribe icons bar"))
		}
		for _, cmd := range gitCommands {
			// could return error if auxbar is enabled
			_ = b.registry.UnsubscribeCommandForFile(b.file, cmd.Name)
		}
	}
	if err := b.vhandler.C.Close(); err != nil {
		ret = multierror.Append(ret, err)
	}
	return
}

func (b *iconsBar) rebuildBar(ctx context.Context) {
	b.dirty = false
	b.cancelBuild()
	ctx, b.cancelBuild = context.WithCancel(ctx)
	b.renderIcons()
	if !b.gitEnabled {
		return
	}
	uri := b.Handler.Resource()
	delLocAttr := b.delLocAttr
	addLocAttr := b.addLocAttr
	go debug.CapturePanicReport(func() {
		filediff, err := b.svc.Diff(ctx, uri)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				b.log(log.ErrorLevel, "compute diff: %v", err)
			}
			return
		}
		b.log(log.TraceLevel, "computed diff: %v", filediff)

		ll := filediff.LocationList(delLocAttr, addLocAttr)
		b.scheduleNextTick(func() {
			select {
			case <-ctx.Done():
				return
			default:
			}
			b.setLocationList(textapi.LocationPriorityInfo, gitLocationsID, ll)
			b.renderIcons()
		})
	})
}

type iconsBarLineIcon struct {
	icon     string
	width    int
	attr     term.Attributes
	priority textapi.LocationPriority
	id       string
	line     int
}

func (b *iconsBar) SetLocationList(pri textapi.LocationPriority, ID string, loc LocationList) {
	b.setLocationList(pri, ID, loc)
	b.renderIcons()
}

func (b *iconsBar) setLocationList(pri textapi.LocationPriority, ID string, loc LocationList) {
	buffers := b.locationBufByID[ID]
	locations := materializeLocationList(buffers.spare, loc)
	var forward LocationList
	if len(locations) > 0 {
		forward = LocationSlice(locations)
	}
	b.storeLocationListIcons(ID, b.locationListIcons(b.iconBufByID[ID], locations, pri, ID))
	b.Handler.SetLocationList(pri, ID, forward)
	buffers.published, buffers.spare = locations, buffers.published
	b.locationBufByID[ID] = buffers
}

func (b *iconsBar) renderIcons() {
	lineIcons, iconColumns := b.mergeLocationIcons()
	if iconColumns != b.iconColumns {
		b.iconColumns = iconColumns
		b.Resize(b.width, b.height)
	}
	b.bar.Buffer().ResetPerformance()
	b.drawLocationIcons(lineIcons)
}

func (b *iconsBar) mergeLocationIcons() (map[int][]iconsBarLineIcon, int) {
	lineIcons := make(map[int][]iconsBarLineIcon)
	iconColumns := 0
	for _, icons := range b.iconsByID {
		for _, icon := range icons {
			iconColumns = max(iconColumns, 1, icon.width)
			lineIcons[icon.line] = append(lineIcons[icon.line], icon)
		}
	}
	for y, icons := range lineIcons {
		sort.SliceStable(icons, func(i, j int) bool {
			if icons[i].priority != icons[j].priority {
				return icons[i].priority > icons[j].priority
			}
			return icons[i].id < icons[j].id
		})
		if len(icons) > 1 {
			icons = icons[:1]
		}
		lineIcons[y] = icons
	}
	return lineIcons, iconColumns
}

func materializeLocationList(dst []textapi.Location, loc LocationList) []textapi.Location {
	if loc == nil {
		return nil
	}
	scrollStartList(loc)
	clear(dst)
	ret := dst[:0]
	for curr, ok := loc.Current(); ok; curr, ok = loc.Next() {
		ret = append(ret, curr)
	}
	return ret
}

func (b *iconsBar) storeLocationListIcons(ID string, icons []iconsBarLineIcon) {
	b.iconBufByID[ID] = icons
	if len(icons) == 0 {
		delete(b.iconsByID, ID)
		return
	}
	b.iconsByID[ID] = icons
}

func (b *iconsBar) locationListIcons(
	dst []iconsBarLineIcon, locations []textapi.Location, pri textapi.LocationPriority, ID string,
) []iconsBarLineIcon {
	if locations == nil {
		return nil
	}
	clear(dst)
	ret := dst[:0]
	for _, curr := range locations {
		if curr.Icon == "" {
			continue
		}
		y, ok := locationLine(curr)
		if !ok {
			continue
		}
		width := graphemecluster.StringWidth(curr.Icon)
		if width == 0 {
			continue
		}
		ret = append(ret, iconsBarLineIcon{
			icon:     curr.Icon,
			width:    width,
			attr:     b.iconAttr(ID, curr),
			priority: pri,
			id:       ID,
			line:     y,
		})
	}
	return ret
}

func (b *iconsBar) drawLocationIcons(lineIcons map[int][]iconsBarLineIcon) {
	for y, icons := range lineIcons {
		at, _ := b.scrollToBarCoordinates(term.Coordinates{Y: y})
		if at.Y < 0 {
			continue
		}
		x := b.iconColumns
		for _, icon := range icons {
			x -= icon.width
			if x < 0 || x >= b.bar.Width() {
				continue
			}
			cellAt := at
			cellAt.X = x
			b.bar.Buffer().DeleteCell(cellAt)
			b.bar.Buffer().InsertStringWithAttr(cellAt, icon.icon, icon.attr)
		}
	}
}

func (b *iconsBar) iconAttr(id string, loc textapi.Location) term.Attributes {
	attr := loc.Attr
	if id == gitLocationsID {
		switch loc.Icon {
		case addIcon:
			attr = b.addAttr
		case delIcon:
			attr = b.delAttr
		}
	}
	fg := attr.Fg
	if attr.Bg != term.ColorDefault {
		fg = attr.Bg
	}
	return term.Attributes{Fg: fg, Attrs: attr.Attrs}
}

func locationLine(loc textapi.Location) (int, bool) {
	from, _ := term.CoordinatesSort(loc.From, loc.To)
	return from.Y, true
}

type iconsBarSubscriber iconsBar

func (b *iconsBarSubscriber) Handle(ctx context.Context, ev textapi.Event) bool {
	if !ev.URI.Equal(b.file) || (ev.Type != textapi.EventTypeFlush &&
		ev.Type != textapi.EventTypeFocus) {
		return false
	}
	(*iconsBar)(b).log(log.TraceLevel, "received event: %s", ev.Type.String())
	(*iconsBar)(b).rebuildBar(ctx)
	return false
}

func (b *iconsBar) OnDidSeek(_, to term.Coordinates) {
	b.bar.SetOffset(term.Coordinates{Y: to.Y})
}

func (b *iconsBar) OnWillSeek(_ term.Coordinates) {
}

func (b *iconsBar) OnWillHide(start, end int) {
}

func (b *iconsBar) OnWillVisible(start int) {
}

func (b *iconsBar) OnDidHide(start, end int) {
	b.rebuildBar(context.Background())
}

func (b *iconsBar) OnDidVisible(start int) {
	b.rebuildBar(context.Background())
}

func (b *iconsBar) OnWillEdit(
	ctx context.Context, from, to term.Coordinates, str string,
) {
}

func (b *iconsBar) OnDidEdit(
	ctx context.Context, start, end term.Coordinates, old string,
) {
}

func (b *iconsBar) scrollToBarCoordinates(pos term.Coordinates) (ret term.Coordinates, ok bool) {
	pos, ok = b.scroll.ScrollToWindowCoordinates(pos)
	ret = pos
	ret.Y += b.scroll.Offset().Y
	ret.X += b.scroll.Offset().X
	return
}

func (b *iconsBar) log(level log.Level, msg string, args ...any) {
	if !log.IsLevelEnabled(level) {
		return
	}
	log.WithField(logging.KeyClass, "text.iconsBar").Logf(level, msg, args...)
}
