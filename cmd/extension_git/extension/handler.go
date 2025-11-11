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
	"unstable.build/go-tui/api/browserapi"
	"unstable.build/go-tui/api/browserapi/browserext"
	"unstable.build/go-tui/api/config"
	configextension "unstable.build/go-tui/api/config/extension"
	"unstable.build/go-tui/api/extensionapi"
	"unstable.build/go-tui/api/textapi"
	"unstable.build/go-tui/api/workspaceapi"
	"unstable.build/go-tui/api/workspaceapi/workspaceext"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/clipboard"
	"unstable.build/go-tui/clipboard/sysclip"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/component/notifications"
	"unstable.build/go-tui/extension"
	"unstable.build/go-tui/extension/extutil"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/ide/vctrl"
	"unstable.build/go-tui/rpc"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text/textrpc"
)

const (
	defaultGitDiffListID = "git_diff"
	commandNextChange    = "gitnextchange"
	commandPrevChange    = "gitprevchange"
	commandToggleOverlay = "gittoggleoverlay"
	commandCopyRemoteURL = "gitcopyremoteurl"
)

var (
	commandCopyRemoteURLManual = textapi.CommandManual{
		Name:     commandCopyRemoteURL,
		Summary:  "Copies to clipboard the web permalink of the remote repository at that line.",
		Synopsis: "[remote]",
	}
)

// Grantee returns this extension's Grantee and the permissions required to run it.
func Grantee() (extension.Grantee, []extensionapi.Permission) {
	return extutil.NewEditorEventHandler(GitHandlerCommands, newGitHandler,
		GitHandlerEvents, GitHandlerPermissions...)
}

var (
	commandNextChangeManual = textapi.CommandManual{
		Name:    commandNextChange,
		Summary: "Moves cursor to the next diff hunk emitted by git.",
	}
	commandPrevChangeManual = textapi.CommandManual{
		Name:    commandPrevChange,
		Summary: "Moves cursor to the previous diff hunk emitted by git.",
	}
	commandToggleOverlayManual = textapi.CommandManual{
		Name:    commandToggleOverlay,
		Summary: "Shows or hides the git diff hunks overlay.",
	}
	// GitHandlerCommands returns the commands that this extension is
	// interested in registering.
	GitHandlerCommands = []textapi.CommandManual{
		commandNextChangeManual,
		commandPrevChangeManual,
		commandToggleOverlayManual,
	}
	// GitHandlerEvents returns the events that this extension is
	// interested in subscribing to.
	GitHandlerEvents = []textapi.EventType{
		textapi.EventTypeOpen,
		textapi.EventTypeEdit,
		textapi.EventTypeFlush,
		textapi.EventTypeScroll,
		textapi.EventTypeVisible,
		textapi.EventTypeHidden,
		textapi.EventTypeFocus,
		textapi.EventTypeUnfocus,
	}
	// GitHandlerPermissions are the required permissions for this
	// extension to run.
	GitHandlerPermissions = []extensionapi.Permission{
		extensionapi.PermissionBrowserWindowManager,
		extensionapi.PermissionNotifications,
		extensionapi.PermissionInterrupt,
		extensionapi.PermissionEditor,
		extensionapi.PermissionCommands,
		extensionapi.PermissionExecute,
		extensionapi.PermissionFileSystem,
		extensionapi.PermissionConfig,
	}

	defaultScrollAttr = term.Attributes{Fg: tcell.ColorBlack}
	defaultAddAttr    = term.Attributes{Bg: tcell.ColorGreen}
	defaultDelAttr    = term.Attributes{Bg: tcell.ColorRed}
	defaultAddLocAttr = term.Attributes{Bg: tcell.ColorDarkSlateGray}
	defaultDelLocAttr = term.Attributes{Bg: tcell.ColorDarkRed}
)

type gitEditorHandler struct {
	ed      textapi.Editor
	m       browserapi.Notifications
	wm      browserapi.WindowManager
	p       browserapi.EventPublisher
	exec    workspaceapi.Executor
	tracker extutil.ResourceTracker
	exit    atomic.Uint32
	ch      chan textapi.Event
	pconfig config.Config
	scroll  struct {
		sync.Mutex
		scroll component.Scroll
	}
	git           vctrl.Service
	gitDiffListID string
	delAttr       term.Attributes
	addAttr       term.Attributes
	delLocAttr    term.Attributes
	addLocAttr    term.Attributes
	clip          clipboard.Register
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

	cwd, fs, err := ret.processGrants(ctx, grants, broker)
	if err != nil {
		return nil, fmt.Errorf("process grants: %w", err)
	}

	ret.initializeConfigValues(pconfig)

	ret.git = vctrl.NewGitCommand(cwd, ret.exec, fs)

	clip, err := sysclip.NewRegister()
	if err != nil {
		ret.log(log.ErrorLevel, "failed to get system clipboard: %v", err)
	} else {
		err = ret.setupCopyRemoteURL(clip, cwd, fs)
	}

	if err != nil {
		ret.log(log.ErrorLevel, "setup copyRemoteURL command: %v", err)
		_, _ = ret.m.Notify(notifications.LevelError,
			"could not install copyRemoteURL command: %v", err)
	}

	go ret.handleEvents(cwd)

	return ret, nil
}

func (h *gitEditorHandler) processGrants(
	ctx context.Context, grants []extension.Grant, broker rpc.MuxBroker,
) (
	cwd workspaceapi.URI, fs workspaceapi.FileSystem, err error,
) {
	for _, grant := range grants {
		switch grant.Permission {
		case extensionapi.PermissionFileSystem:
			fs, err = workspaceext.FileSystem(ctx, grant, broker)
			if err != nil {
				return
			}
			var cwdURI workspaceapi.URI
			cwdURI, err = fs.URI(".")
			if err != nil {
				return
			}
			cwd = cwdURI
		case extensionapi.PermissionExecute:
			h.exec, err = workspaceext.Executor(ctx, grant, broker)
			if err != nil {
				return
			}
		case extensionapi.PermissionInterrupt:
			h.p, err = browserext.EventPublisher(ctx, grant, broker)
			if err != nil {
				return
			}
		case extensionapi.PermissionNotifications:
			var m browserapi.Notifications
			m, err = browserext.Notifications(ctx, grant, broker)
			if err != nil {
				err = fmt.Errorf("acquire browser notifications: %w ", err)
				return
			}
			h.m = m
		case extensionapi.PermissionBrowserWindowManager:
			h.wm, err = browserext.WindowManager(ctx, grant, broker)
			if err != nil {
				return
			}

			syncComp := component.Sync(&h.scroll, component.WithLogging(&h.scroll.scroll, log.Tracef))
			cfg := browserapi.BarConfig{
				Frame:       browserapi.BarFrameNever,
				Size:        1,
				Orientation: browserapi.OrientationLeft,
			}
			err = h.wm.Bar(cfg, handler.Nop(syncComp))
			if err != nil {
				return
			}
		case extensionapi.PermissionConfig:
			var config config.Config
			config, err = configextension.FetchConfig(ctx, grant, broker)
			if err != nil {
				return
			}
			var tabspaces int
			tabspaces, err = extutil.Tabspaces(config)
			if err != nil {
				err = fmt.Errorf("get configured tabspaces: %v", err)
				return
			}
			var wrap bool
			wrap, err = extutil.Wrap(config)
			if err != nil {
				err = fmt.Errorf("get configured wrap mode: %v", err)
				return
			}
			h.tracker.Init(tabspaces, wrap)
			h.log(log.DebugLevel, "initialized content tracker with tabspaces: %d and wrap mode: %v",
				tabspaces, h.scroll.scroll.Wrap)

			// could have been already initialized as a bar,
			// depending on order of permissions
			h.scroll.Lock()
			h.scroll.scroll.Wrap = wrap
			h.scroll.Unlock()
		}
	}
	return
}

func (h *gitEditorHandler) initializeConfigValues(pconfig config.Config) {
	h.pconfig = pconfig
	var err error

	h.gitDiffListID, err = pconfig.GetString("git_diff_list_id")
	if err != nil {
		if err != config.ErrNotFound {
			h.log(log.WarnLevel, "failed to get 'git_diff_list_id' from config: %v", err)
		}
		h.gitDiffListID = defaultGitDiffListID
	}

	h.addAttr, err = config.GetAttributes(pconfig, "add_attr")
	if err != nil {
		if err != config.ErrNotFound {
			h.log(log.WarnLevel, "failed to get 'add_attr' from config: %v", err)
		}
		h.addAttr = defaultAddAttr
	}

	h.delAttr, err = config.GetAttributes(pconfig, "del_attr")
	if err != nil {
		if err != config.ErrNotFound {
			h.log(log.WarnLevel, "failed to get 'del_attr' from config: %v", err)
		}
		h.delAttr = defaultDelAttr
	}
}

func (h *gitEditorHandler) loadLocAttr() {
	var err error
	h.addLocAttr, err = config.GetAttributes(h.pconfig, "add_loc_attr")
	if err != nil {
		if err != config.ErrNotFound {
			h.log(log.WarnLevel, "failed to get 'add_attr' from config: %v", err)
		}
		h.addLocAttr = defaultAddLocAttr
	}

	h.delLocAttr, err = config.GetAttributes(h.pconfig, "del_loc_attr")
	if err != nil {
		if err != config.ErrNotFound {
			h.log(log.WarnLevel, "failed to get 'del_attr' from config: %v", err)
		}
		h.delLocAttr = defaultDelLocAttr
	}
}

func (h *gitEditorHandler) setupCopyRemoteURL(
	clip clipboard.Register, cwd workspaceapi.URI, fs workspaceapi.FileSystem,
) (err error) {
	h.clip = clip
	// TODO refactor to use workspace API
	err = h.ed.(*textrpc.Client).SubscribeCommand(
		commandCopyRemoteURLManual,
		newCopyRemoteURL(h.git, h.clip, h.m),
	)

	if err != nil {
		err = fmt.Errorf("subscribe command: %w", err)
		return
	}
	return

}

func (h *gitEditorHandler) Complete(ctx context.Context, name string, args []string) (
	iterator.Iterator[string], error,
) {
	return iterator.FromSlice[string](nil), nil
}

func (h *gitEditorHandler) HandleCommand(ctx context.Context, cmd textapi.Command) (
	err error,
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
	case commandToggleOverlay:
		if h.delLocAttr == (term.Attributes{}) {
			h.loadLocAttr()
		} else {
			h.delLocAttr = term.Attributes{}
			h.addLocAttr = term.Attributes{}
		}
		h.runDiff(ctx, cmd.URI, cmd.Resource)
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
		y := int(math.Max(0, float64(hunk.NewStartLine)))
		if hunk.NewLines == 0 {
			// convert coordinates into content coordinates with wraps
			at := term.Coordinates{Y: y}
			locs = append(locs, textapi.Location{
				From: term.Coordinates{Y: at.Y - 1, X: 0},
				To:   term.Coordinates{Y: at.Y - 1, X: 1},
				Attr: h.delLocAttr,
			})
			at, ok := contentCoordinatesWithWraps(res, at)
			if !ok { // hidden
				continue
			}
			h.log(log.TraceLevel, "adding deletion at %+v", at)
			h.scroll.scroll.Buffer().DeleteCell(at)
			h.scroll.scroll.Buffer().InsertStringWithAttr(at, "-", h.delAttr)
			continue
		}

		from := term.Coordinates{Y: y}
		to := term.Coordinates{Y: int(hunk.NewStartLine + hunk.NewLines)}
		locs = append(locs, textapi.Location{
			From: term.Coordinates{Y: from.Y - 1},
			To:   term.Coordinates{Y: to.Y - 1},
			Attr: h.addLocAttr,
		})

		for y := from.Y; y < to.Y; y++ {
			at, ok := contentCoordinatesWithWraps(res, term.Coordinates{Y: y})
			if !ok { // hidden
				continue
			}
			h.log(log.TraceLevel, "adding addition at %+v", at)
			icon := "+"
			h.scroll.scroll.Buffer().DeleteCell(at)
			if hunk.OrigLines != 0 {
				icon = "󰦒"
			}
			h.scroll.scroll.Buffer().InsertStringWithAttr(at, icon, h.addAttr)
		}
	}

	return locs
}

func (h *gitEditorHandler) runDiff(
	ctx context.Context, uri workspaceapi.URI, resource textapi.Handler,
) {
	res, ok := h.getResource(uri)
	if !ok {
		return
	}

	h.initBar(uri)

	diff, err := h.git.Diff(ctx, uri.Path())
	if err != nil {
		h.resetBar()
		h.interrupt(ctx)
	}
	if errors.Is(err, vctrl.ErrDiffNoChanges) {
		h.log(log.TraceLevel, "reset git bar because no diff changes: %v", err)
		err = nil
	} else if err != nil {
		err = fmt.Errorf("git service diff: %w", err)
	}

	if diff == nil {
		if err != nil {
			h.log(log.ErrorLevel, "%s", err.Error())
		}
		h.setLocationList(resource, nil)
		return
	}

	locs := h.parseDiff(res, diff)
	res.Metadata.(*metadata).locations = locs

	h.setLocationList(resource, locs)
}

func (h *gitEditorHandler) resetBar() {
	h.scroll.Lock()
	defer h.scroll.Unlock()
	h.scroll.scroll.Init(cell.NewBuffer())
	h.log(log.TraceLevel, "reset bar")
}

// allow scroll to seek to same positions as editor buffer
func (h *gitEditorHandler) setLastLocationList(ev textapi.Event) {
	res, ok := h.getResource(ev.URI)
	if !ok {
		return
	}
	locs := res.Metadata.(*metadata).locations
	if locs != nil {
		h.log(log.DebugLevel, "set location list: %s: no locations found", ev.URI)
		return
	}

	h.log(log.TraceLevel, "set location list: %s: locations found: %#v", ev.URI, locs)
	h.setLocationList(ev.Resource, locs)
}

func (h *gitEditorHandler) setLocationList(resource textapi.Handler, locs []textapi.Location) {
	err := h.ed.SetLocationList(resource, textapi.LocationPriorityInfo,
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
			h.runDiff(ctx, ev.URI, ev.Resource)
		case textapi.EventTypeFocus:
			h.runDiff(ctx, ev.URI, ev.Resource)
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

	res, ok := h.getResource(ev.URI)
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

func (h *gitEditorHandler) getResource(uri workspaceapi.URI) (
	*extutil.TrackedResource, bool,
) {
	res, ok := h.tracker.Resource(uri)
	if !ok {
		h.log(log.ErrorLevel, "resource with uri %q not found in tracker",
			uri.String())
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

func (h *gitEditorHandler) initBar(uri workspaceapi.URI) {
	h.scroll.Lock()
	defer h.scroll.Unlock()

	h.scroll.scroll.Init(cell.NewBuffer())

	res, ok := h.getResource(uri)
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
			uri, rows, offset)
		return
	}

	if ok := h.scroll.scroll.SetOffset(offset); !ok {
		h.log(log.WarnLevel, "init bar: %s: rows=%d; set offset %+v not ok",
			uri, rows, offset)
	} else {
		h.log(log.DebugLevel, "init bar: %s: rows=%d; offset=%+v",
			uri, rows, offset)
	}
}

func contentCoordinatesWithWraps(
	res *extutil.TrackedResource, pos term.Coordinates,
) (at term.Coordinates, ok bool) {
	// window coordiantes subtracts offset, adds wraps
	at, ok = res.WindowCoordinates(pos)
	if !ok {
		return
	}
	// add offset and we should have conent coordinates
	// with wraps.
	at = term.CoordinatesSum(at, res.Offset())
	return
}
