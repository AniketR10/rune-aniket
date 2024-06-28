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
package extension

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/sourcegraph/go-diff/diff"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/blue/logging"
	"github.com/unstablebuild/tcell/v3"
	browserapi "unstable.build/go-tui/api/browser"
	browserextension "unstable.build/go-tui/api/browser/extension"
	"unstable.build/go-tui/api/config"
	configextension "unstable.build/go-tui/api/config/extension"
	textapi "unstable.build/go-tui/api/text"
	workspaceapi "unstable.build/go-tui/api/workspace"
	workspaceextension "unstable.build/go-tui/api/workspace/extension"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/extension"
	extutil "unstable.build/go-tui/extension/util"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/rpc"
	"unstable.build/go-tui/term"
)

const (
	defaultGitDiffListID = "git_diff"
	commandNextChange    = "gitNextChange"
	commandPrevChange    = "gitPrevChange"
)

// Grantee returns this extension's Grantee and the permissions required to run it.
func Grantee() (extension.Grantee, []extension.Permission) {
	return extutil.NewEditorEventHandler(GitHandlerCommands, newGitHandler,
		GitHandlerEvents, GitHandlerPermissions...)
}

var (
	// GitHandlerCommands returns the commands that this extension is
	// interested in registering.
	GitHandlerCommands = []textapi.CommandManual{
		{
			Name:    commandNextChange,
			Summary: "Moves cursor to the next diff hunk emitted by git.",
		},
		{
			Name:    commandPrevChange,
			Summary: "Moves cursor to the previous diff hunk emitted by git.",
		},
	}
	// GitHandlerEvents returns the events that this extension is
	// interested in subscribing to.
	GitHandlerEvents = []textapi.EventType{
		textapi.EventTypeOpen,
		textapi.EventTypeEdit,
		textapi.EventTypeFlush,
		textapi.EventTypeScroll,
		textapi.EventTypeFocus,
		textapi.EventTypeUnfocus,
	}
	// GitHandlerPermissions are the required permissions for this
	// extension to run.
	GitHandlerPermissions = []extension.Permission{
		extension.PermissionBrowserWindowManager,
		extension.PermissionBrowserEventPublisher,
		extension.PermissionEditor,
		extension.PermissionExecute,
		extension.PermissionFileSystem,
		extension.PermissionConfig,
	}

	defaultScrollAttr = term.Attributes{Fg: tcell.ColorBlack}
	defaultAddAttr    = term.Attributes{Bg: tcell.ColorGreen}
	defaultDelAttr    = term.Attributes{Bg: tcell.ColorRed}
)

type gitEditorHandler struct {
	ed      textapi.Editor
	wm      browserapi.WindowManager
	p       browserapi.EventPublisher
	exec    workspaceapi.Executor
	tracker extutil.ResourceTracker
	exit    atomic.Uint32
	ch      chan textapi.Event
	scroll  struct {
		sync.Mutex
		scroll component.Scroll
	}
	git           gitService
	gitDiffListID string
	delAttr       term.Attributes
	addAttr       term.Attributes
}

func newGitHandler(
	ctx context.Context, ed textapi.Editor, grants []extension.Grant,
	broker rpc.MuxBroker, pconfig config.Config,
) (extutil.CommandEventHandler, error) {
	ret := new(gitEditorHandler)
	ret.ed = ed
	ret.ch = make(chan textapi.Event)
	ret.scroll.scroll.Init(cell.NewBuffer())

	var err error
	ret.scroll.scroll.Attributes, err = config.GetAttributes(pconfig, "bar_attr")
	if err != nil {
		if err != config.ErrNotFound {
			ret.log(log.WarnLevel, "failed to get 'bar_attr' from config: %v", err)
		}
		ret.scroll.scroll.Attributes = defaultScrollAttr
	}

	var cwd workspaceapi.URI
	for _, grant := range grants {
		switch grant.Permission {
		case extension.PermissionFileSystem:
			fs, err := workspaceextension.FileSystem(ctx, grant, broker)
			if err != nil {
				return nil, err
			}
			cwdURI, err := fs.URI(".")
			if err != nil {
				return nil, err
			}
			cwd = cwdURI
		case extension.PermissionExecute:
			ret.exec, err = workspaceextension.Executor(ctx, grant, broker)
			if err != nil {
				return nil, err
			}
		case extension.PermissionBrowserEventPublisher:
			ret.p, err = browserextension.EventPublisher(ctx, grant, broker)
			if err != nil {
				return nil, err
			}
		case extension.PermissionBrowserWindowManager:
			ret.wm, err = browserextension.WindowManager(ctx, grant, broker)
			if err != nil {
				return nil, err
			}
			syncComp := component.Sync(&ret.scroll, component.WithLogging(&ret.scroll.scroll, log.Tracef))
			cfg := browserapi.BarConfig{
				Frame:       browserapi.BarFrameDefault,
				Size:        1,
				Orientation: browserapi.OrientationLeft,
			}
			err = ret.wm.Bar(cfg, handler.Nop(syncComp))
			if err != nil {
				return nil, err
			}
		case extension.PermissionConfig:
			config, err := configextension.FetchConfig(ctx, grant, broker)
			if err != nil {
				return nil, err
			}
			tabspaces, err := extutil.Tabspaces(config)
			if err != nil {
				return nil, fmt.Errorf("get configured tabspaces: %v", err)
			}
			wrap, err := extutil.Wrap(config)
			if err != nil {
				return nil, fmt.Errorf("get configured wrap mode: %v", err)
			}
			ret.tracker.Init(tabspaces, wrap)
			ret.log(log.DebugLevel, "initialized content tracker with tabspaces: %d and wrap mode: %v",
				tabspaces, ret.scroll.scroll.Wrap)

			// could have been already initialized as a bar,
			// depending on order of permissions
			ret.scroll.Lock()
			ret.scroll.scroll.Wrap = wrap
			ret.scroll.Unlock()
		}
	}

	ret.git = newCmdGitService(ret.exec, cwd)

	ret.gitDiffListID, err = pconfig.GetString("git_diff_list_id")
	if err != nil {
		if err != config.ErrNotFound {
			ret.log(log.WarnLevel, "failed to get 'git_diff_list_id' from config: %v", err)
		}
		ret.gitDiffListID = defaultGitDiffListID
	}

	ret.addAttr, err = config.GetAttributes(pconfig, "add_attr")
	if err != nil {
		if err != config.ErrNotFound {
			ret.log(log.WarnLevel, "failed to get 'add_attr' from config: %v", err)
		}
		ret.addAttr = defaultAddAttr
	}

	ret.delAttr, err = config.GetAttributes(pconfig, "del_attr")
	if err != nil {
		if err != config.ErrNotFound {
			ret.log(log.WarnLevel, "failed to get 'del_attr' from config: %v", err)
		}
		ret.delAttr = defaultDelAttr
	}

	go ret.handleEvents(cwd)

	return ret, nil
}

func (h *gitEditorHandler) Complete(ctx context.Context, name string, args []string) (
	iterator.Iterator[string], error,
) {
	return iterator.FromSlice[string](nil), nil
}

func (h *gitEditorHandler) HandleCommand(ctx context.Context, cmd textapi.Command) (
	exit bool, err error,
) {
	if cmd.Resource == nil {
		return
	}
	switch cmd.Name {
	case commandNextChange:
		err = h.ed.MoveToNextLocation(cmd.Resource, h.gitDiffListID)
		if err != nil {
			err = fmt.Errorf("move to next location: %v", err)
		}
	case commandPrevChange:
		err = h.ed.MoveToPrevLocation(cmd.Resource, h.gitDiffListID)
		if err != nil {
			err = fmt.Errorf("move to prev location: %v", err)
		}
	}

	return
}

func (h *gitEditorHandler) Handle(
	ctx context.Context, ev textapi.Event,
) (exit bool) {
	uexit := h.exit.Load()
	exit = uexit != 0
	if exit {
		return
	}

	h.ch <- ev
	return
}

func (h *gitEditorHandler) Close() error {
	closing := h.exit.CompareAndSwap(0, 1)
	if !closing {
		return nil
	}
	close(h.ch)
	return nil
}

func (h *gitEditorHandler) parseDiff(
	res *extutil.TrackedResource, diff *diff.FileDiff,
) []textapi.Location {
	h.scroll.Lock()
	defer h.scroll.Unlock()

	var locs []textapi.Location
	for _, hunk := range diff.Hunks {
		h.log(log.TraceLevel, "read file diff hunk: %#v", hunk)
		y := int(math.Max(0, float64(hunk.NewStartLine-1)))
		if hunk.NewLines == 0 {
			// convert coordinates into content coordinates with wraps
			at := term.Coordinates{Y: y}
			locs = append(locs, textapi.Location{
				From: at,
				To:   term.Coordinates{Y: at.Y, X: 1},
				// Attr: deleteAttr,
			})
			at = contentCoordinatesWithWraps(res, at)
			h.log(log.TraceLevel, "adding deletion at %+v", at)
			h.scroll.scroll.Buffer().DeleteCell(at)
			h.scroll.scroll.Buffer().InsertStringWithAttr(at, "-", h.delAttr)
			continue
		}

		from := term.Coordinates{Y: y}
		to := term.Coordinates{Y: int(hunk.NewStartLine - 1 + hunk.NewLines)}
		locs = append(locs, textapi.Location{From: from, To: to})

		for y := from.Y; y < to.Y; y++ {
			at := contentCoordinatesWithWraps(res, term.Coordinates{Y: y})
			h.log(log.TraceLevel, "adding addition at %+v", at)
			h.scroll.scroll.Buffer().DeleteCell(at)
			h.scroll.scroll.Buffer().InsertStringWithAttr(at, "+", h.addAttr)
		}
	}

	return locs
}

func (h *gitEditorHandler) runDiff(ctx context.Context, ev textapi.Event) {
	res, ok := h.getResource(ev)
	if !ok {
		return
	}

	h.initBar(ev)

	diff, err := h.git.diff(ev.URI.Path())
	if err != nil {
		h.resetBar()
		h.interrupt(ctx)
	}
	if errors.Is(err, errDiffNoChanges) {
		h.log(log.TraceLevel, "reset git bar because no diff changes: %v", err)
		err = nil
	} else if err != nil {
		err = fmt.Errorf("git service diff: %w", err)
	}

	if diff == nil {
		if err != nil {
			h.log(log.ErrorLevel, err.Error())
		}
		return
	}

	locs := h.parseDiff(res, diff)
	res.Metadata.(*metadata).locations = locs

	h.setLocationList(ev, locs)
}

func (h *gitEditorHandler) resetBar() {
	h.scroll.Lock()
	defer h.scroll.Unlock()
	h.scroll.scroll.Init(cell.NewBuffer())
	h.log(log.TraceLevel, "reset bar")
}

// allow scroll to seek to same positions as editor buffer
func (h *gitEditorHandler) setLastLocationList(ev textapi.Event) {
	res, ok := h.getResource(ev)
	if !ok {
		return
	}
	locs := res.Metadata.(*metadata).locations
	if locs != nil {
		h.log(log.DebugLevel, "set location list: %s: no locations found", ev.URI)
		return
	}

	h.log(log.TraceLevel, "set location list: %s: locations found: %#v", ev.URI, locs)
	h.setLocationList(ev, locs)
}

func (h *gitEditorHandler) setLocationList(ev textapi.Event, locs []textapi.Location) {
	err := h.ed.SetLocationList(ev.Resource, textapi.LocationPriorityInfo,
		h.gitDiffListID, textapi.LocationSlice(locs))
	if err != nil {
		h.log(log.ErrorLevel, "set location list: %v", err)
	}
}

func (h *gitEditorHandler) handleEvents(cwd workspaceapi.URI) {
	ctx := context.Background()
	for ev := range h.ch {
		var start time.Time
		if log.IsLevelEnabled(log.TraceLevel) {
			start = time.Now()
			h.log(log.TraceLevel, "handle %v", ev.Type)
		}

		h.tracker.Handle(ctx, ev)

		switch ev.Type {
		case textapi.EventTypeEdit:
			h.setLastLocationList(ev)
		case textapi.EventTypeOpen:
			res, _ := h.tracker.Resource(ev.URI)
			res.Metadata = new(metadata)
			h.setBarOffset(ev)
		case textapi.EventTypeFlush:
			h.runDiff(ctx, ev)
		case textapi.EventTypeFocus:
			h.runDiff(ctx, ev)
		case textapi.EventTypeUnfocus:
			h.resetBar()
			h.interrupt(ctx)
		case textapi.EventTypeScroll:
			if h.setBarOffset(ev) {
				h.interrupt(ctx)
			}
		}
		if log.IsLevelEnabled(log.TraceLevel) {
			h.log(log.TraceLevel, "handle(%#v) in %s", ev.Type, time.Since(start))
		}
	}
}

func (h *gitEditorHandler) setBarOffset(ev textapi.Event) bool {
	h.scroll.Lock()
	defer h.scroll.Unlock()

	res, ok := h.getResource(ev)
	if !ok {
		return false
	}

	pos := res.Offset()
	pos.X = 0

	if h.scroll.scroll.Offset().Y == pos.Y {
		h.log(log.TraceLevel, "set bar offset: %s: %+v: already set to offset",
			ev.URI, pos)
		return false
	}

	ok = h.scroll.scroll.SetOffset(pos)
	if !ok {
		h.log(log.WarnLevel, "set bar offset: %s: %+v: not ok",
			ev.URI, pos)
	} else {
		h.log(log.DebugLevel, "set bar offset: %s: %+v: ok",
			ev.URI, pos)
	}
	return ok
}

func (h *gitEditorHandler) interrupt(ctx context.Context) {
	if err := h.p.Interrupt(ctx); err != nil {
		h.log(log.ErrorLevel, "interrupt: %v", err)
	}
}

func (h *gitEditorHandler) getResource(ev textapi.Event) (
	*extutil.TrackedResource, bool,
) {
	res, ok := h.tracker.Resource(ev.URI)
	if !ok {
		h.log(log.ErrorLevel, "resource with uri %q not found in tracker",
			ev.URI.String())
	}
	return res, ok
}

func (h *gitEditorHandler) log(level log.Level, msg string, args ...any) {
	log.WithFields(log.Fields{
		logging.KeyClass: "extension.gitEditorHandler",
	}).Logf(level, msg, args...)
}

type metadata struct {
	locations []textapi.Location
}

func (h *gitEditorHandler) initBar(ev textapi.Event) {
	h.scroll.Lock()
	defer h.scroll.Unlock()

	h.scroll.scroll.Init(cell.NewBuffer())

	res, ok := h.getResource(ev)
	if !ok {
		return
	}

	rows := res.RowsWithWraps()
	for y := 0; y < rows+1; y++ {
		h.scroll.scroll.Buffer().Insert(term.Coordinates{Y: y}, ' ')
	}

	offset := res.Offset()
	if offset == h.scroll.scroll.Offset() {
		h.log(log.TraceLevel, "init bar: %s: rows=%d; offset=%+v: already at offset",
			ev.URI, rows, offset)
		return
	}

	if ok := h.scroll.scroll.SetOffset(offset); !ok {
		h.log(log.WarnLevel, "init bar: %s: rows=%d; set offset %+v not ok",
			ev.URI, rows, offset)
	} else {
		h.log(log.DebugLevel, "init bar: %s: rows=%d; offset=%+v",
			ev.URI, rows, offset)
	}
}

func contentCoordinatesWithWraps(
	res *extutil.TrackedResource, at term.Coordinates,
) term.Coordinates {
	// window coordiantes subtracts offset, adds wraps
	at = res.WindowCoordinates(at)
	// add offset and we should have conent coordinates
	// with wraps.
	at = term.CoordinatesSum(at, res.Offset())
	return at
}
