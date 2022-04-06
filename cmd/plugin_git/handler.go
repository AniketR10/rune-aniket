package main

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/component"
	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/plugin"
	plugutil "github.com/ernestrc/go-tui/plugin/util"
	"github.com/ernestrc/go-tui/proto"
	"github.com/ernestrc/go-tui/term"
	"github.com/ernestrc/go-tui/text"
	log "github.com/sirupsen/logrus"
	"github.com/sourcegraph/go-diff/diff"
)

const (
	defaultGitDiffListID = "git_diff"
	commandNextChange    = "gitNextChange"
	commandPrevChange    = "gitPrevChange"
)

var (
	gitHandlerCommands = []string{commandNextChange, commandPrevChange}
	gitHandlerEvents   = []text.EventType{
		text.EventTypeOpen,
		text.EventTypeEdit,
		text.EventTypeFlush,
		text.EventTypeScroll,
		text.EventTypeFocus,
		text.EventTypeUnfocus,
	}
	gitHandlerPermissions = []plugin.Permission{
		plugin.PermissionBrowserWindowManager,
		plugin.PermissionBrowserEventPublisher,
		plugin.PermissionEditor,
	}

	defaultScrollAttr = term.Attributes{Fg: term.ColorBlack}
	defaultAddAttr    = term.Attributes{Bg: term.ColorGreen}
	defaultDelAttr    = term.Attributes{Bg: term.ColorRed}
)

type gitEditorHandler struct {
	ed     text.Editor
	wm     browser.WindowManager
	p      browser.EventPublisher
	exit   uint32
	ch     chan text.Event
	scroll struct {
		sync.Mutex
		scroll component.Scroll
	}
	gitDiffListID string
	delAttr       term.Attributes
	addAttr       term.Attributes
	rows          map[string]int
	offsets       map[string]term.Coordinates
	lastLocs      map[string][]text.Location
}

func newGitHandler(
	ed text.Editor, grants []plugin.Grant,
	broker proto.MuxBroker, pconfig plugin.Config,

) (plugutil.CommandEventHandler, error) {
	ret := new(gitEditorHandler)
	ret.ed = ed
	ret.rows = make(map[string]int)
	ret.offsets = make(map[string]term.Coordinates)
	ret.ch = make(chan text.Event)
	ret.lastLocs = make(map[string][]text.Location)
	ret.scroll.scroll.Init()

	var err error
	ret.scroll.scroll.Attributes, err = plugin.GetAttributes(pconfig, "bar_attr")
	if err != nil {
		if err != plugin.ErrNotFound {
			log.Warningf("failed to get 'bar_attr' from config: %v", err)
		}
		ret.scroll.scroll.Attributes = defaultScrollAttr
	}

	for _, grant := range grants {
		switch grant.Permission {
		case plugin.PermissionBrowserEventPublisher:
			ret.p, err = plugin.EventPublisher(grant.Token, broker)
			if err != nil {
				return nil, err
			}
		case plugin.PermissionBrowserWindowManager:
			ret.wm, err = plugin.WindowManager(grant.Token, broker)
			if err != nil {
				return nil, err
			}
			comp := component.WithBackground(&ret.scroll.scroll,
				term.Cell{Bg: ret.scroll.scroll.Attributes.Bg,
					Fg: ret.scroll.scroll.Attributes.Fg})
			syncComp := component.Sync(&ret.scroll, comp)
			err = ret.wm.Bar(browser.OrientationLeft, handler.Nop(syncComp))
			if err != nil {
				return nil, err
			}
		}
	}

	ret.gitDiffListID, err = pconfig.GetString("git_diff_list_id")
	if err != nil {
		if err != plugin.ErrNotFound {
			log.Warningf("failed to get 'git_diff_list_id' from config: %v", err)
		}
		ret.gitDiffListID = defaultGitDiffListID
	}

	ret.addAttr, err = plugin.GetAttributes(pconfig, "add_attr")
	if err != nil {
		if err != plugin.ErrNotFound {
			log.Warningf("failed to get 'add_attr' from config: %v", err)
		}
		ret.addAttr = defaultAddAttr
	}

	ret.delAttr, err = plugin.GetAttributes(pconfig, "del_attr")
	if err != nil {
		if err != plugin.ErrNotFound {
			log.Warningf("failed to get 'del_attr' from config: %v", err)
		}
		ret.delAttr = defaultDelAttr
	}

	go ret.handleEvents()

	return ret, nil
}

func (h *gitEditorHandler) HandleCommand(ctx context.Context, cmd text.Command) (
	exit bool,
) {
	if cmd.Resource == nil {
		return
	}
	switch cmd.Name {
	case commandNextChange:
		err := h.ed.MoveToNextLocation(cmd.Resource, h.gitDiffListID)
		if err != nil {
			log.Errorf("lspEditorHandler.MoveToNextLocation(%s): %v", cmd.Name, err)
		}
	case commandPrevChange:
		err := h.ed.MoveToPrevLocation(cmd.Resource, h.gitDiffListID)
		if err != nil {
			log.Errorf("lspEditorHandler.MoveToNextLocation(%s): %v", cmd.Name, err)
		}
	}

	return false
}

func (h *gitEditorHandler) parseDiff(diff *diff.FileDiff) []text.Location {
	h.scroll.Lock()
	defer h.scroll.Unlock()

	var locs []text.Location
	for _, hunk := range diff.Hunks {
		log.Tracef("Read file diff hunk: %#v", hunk)
		if hunk.NewLines == 0 {
			// FIXME https://github.com/ernestrc/go-tui/issues/59
			at := term.Coordinates{Y: int(hunk.NewStartLine - 1)}
			locs = append(locs, text.Location{
				From: at,
				To:   term.Coordinates{Y: at.Y, X: 1},
				// Attr: deleteAttr,
			})
			h.scroll.scroll.Buffer().DeleteCell(at)
			h.scroll.scroll.Buffer().InsertStringWithAttr(at, "-", h.delAttr)
			continue
		}

		from := term.Coordinates{Y: int(hunk.NewStartLine - 1)}
		to := term.Coordinates{Y: int(hunk.NewStartLine - 1 + hunk.NewLines)}
		locs = append(locs, text.Location{From: from, To: to})

		for y := from.Y; y < to.Y; y++ {
			at := term.Coordinates{Y: y}
			h.scroll.scroll.Buffer().DeleteCell(at)
			h.scroll.scroll.Buffer().InsertStringWithAttr(at, "+", h.addAttr)
		}
	}

	return locs
}

func (h *gitEditorHandler) pushNewDiffLocations(
	filename string, resource text.Handler,
) error {
	c := exec.Command("git", "diff", "-U0", filename)

	stdout, err := c.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %v", err)
	}

	err = c.Start()
	if err != nil {
		return fmt.Errorf("failed to start executable: %v", err)
	}

	h.initScroll(filename)

	r := diff.NewFileDiffReader(stdout)
	diff, err := r.Read()
	if err != nil {
		if strings.Contains(err.Error(), io.EOF.Error()) {
			// make sure that an interrupt is called
			// so the new scroll bar is updated, when
			// focus switched to a file with no changes.
			return h.p.PublishInterrupt()
		}
		return fmt.Errorf("failed to read from stdout: %v", err)
	}

	// releases associated resources; error is ignored because
	err = c.Wait()
	if err != nil {
		return fmt.Errorf("git process error: %v", err)
	}

	locs := h.parseDiff(diff)
	h.lastLocs[filename] = locs

	return h.ed.SetLocationList(resource, h.gitDiffListID, text.LocationSlice(locs))
}

func (h *gitEditorHandler) resetScroll() {
	h.scroll.Lock()
	defer h.scroll.Unlock()
	h.scroll.scroll.Init()
}

func (h *gitEditorHandler) initScroll(name string) {
	h.scroll.Lock()
	defer h.scroll.Unlock()

	h.scroll.scroll.Init()
	rows, ok := h.rows[name]
	if !ok {
		log.Warnf("could not find max rows for file %s", name)
		return
	}

	for y := 0; y < rows+1; y++ {
		h.scroll.scroll.Buffer().Insert(term.Coordinates{Y: y}, ' ')
	}

	offset, ok := h.offsets[name]
	if !ok {
		log.Warnf("could not find last scroll offset for file %s", name)
		return
	}

	h.scroll.scroll.SetOffset(offset)

	log.Debugf("initScroll(%s): lenScroll=%d; rows=%d; offset=%#v",
		name, h.scroll.scroll.Buffer().Rows(), rows, offset)
}

// allow scroll to seek to same positions as editor buffer
func (h *gitEditorHandler) setScrollMaxContent(resourceName string, ev text.Event) {
	rows := len(cell.StringToCells(ev.Content))
	// best effort until #59 is resolved
	rows *= 2
	rows += 100
	h.rows[resourceName] = rows
	log.Debugf("setScrollMaxContent(%s): %d", ev.URI, rows)
}

func (h *gitEditorHandler) setScrollOffset(filename string, pos term.Coordinates) {
	h.scroll.Lock()
	defer h.scroll.Unlock()

	if h.scroll.scroll.Offset().Y == pos.Y {
		return
	}

	// we're only interested in the vertical scroll
	pos.X = 0

	ok := h.scroll.scroll.SetOffset(pos)
	h.offsets[filename] = pos

	if !ok {
		log.Errorf("setScrollOffset(%s): %#v: could not set offset", filename, pos)
		return
	}
	log.Tracef("setScrollOffset(%s): %#v OK", filename, pos)
}

func (h *gitEditorHandler) pushLastDiffLocations(filename string, resource text.Handler) error {
	locs, ok := h.lastLocs[filename]
	if !ok {
		log.Debugf("pushLastDiffLocations(%s): no locations found", filename)
		return nil
	}

	log.Tracef("pushLastDiffLocations(%s): locations found: %#v", filename, locs)
	return h.ed.SetLocationList(resource, h.gitDiffListID, text.LocationSlice(locs))
}

func (h *gitEditorHandler) handleEvents() {
	for ev := range h.ch {
		var start time.Time
		if log.IsLevelEnabled(log.TraceLevel) {
			start = time.Now()
			log.Tracef("Handle(%#v)", ev.Type)
		}

		resourceName := ev.URI.Path()

		var err error
		switch ev.Type {
		case text.EventTypeEdit:
			err = h.pushLastDiffLocations(resourceName, ev.Resource)
		case text.EventTypeOpen:
			h.setScrollOffset(resourceName, term.Coordinates{})
			fallthrough
		case text.EventTypeFlush:
			h.setScrollMaxContent(resourceName, ev)
			fallthrough
		case text.EventTypeFocus:
			err = h.pushNewDiffLocations(resourceName, ev.Resource)
		case text.EventTypeUnfocus:
			h.resetScroll()
			err = h.p.PublishInterrupt()
		case text.EventTypeScroll:
			h.setScrollOffset(resourceName, ev.Start)
			err = h.p.PublishInterrupt()
		}
		if err != nil {
			log.Errorf("Handle(%#v): %v", ev.Type, err)
		}
		if log.IsLevelEnabled(log.TraceLevel) {
			log.Tracef("Handle(%#v) in %s", ev.Type, time.Since(start))
		}
	}
}

func (h *gitEditorHandler) Handle(
	ctx context.Context, ev text.Event,
) (exit bool) {
	uexit := atomic.LoadUint32(&h.exit)
	exit = uexit != 0
	if exit {
		return
	}

	h.ch <- ev
	return
}

func (h *gitEditorHandler) Close() error {
	atomic.StoreUint32(&h.exit, 1)
	close(h.ch)
	return nil
}
