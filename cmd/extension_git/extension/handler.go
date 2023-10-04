package extension

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	multierr "github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	"github.com/sourcegraph/go-diff/diff"
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
	"unstable.build/go-tui/proto"
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
	GitHandlerCommands = []string{commandNextChange, commandPrevChange}
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

	defaultScrollAttr = term.Attributes{Fg: term.ColorBlack}
	defaultAddAttr    = term.Attributes{Bg: term.ColorGreen}
	defaultDelAttr    = term.Attributes{Bg: term.ColorRed}
)

type gitEditorHandler struct {
	ed     textapi.Editor
	wm     browserapi.WindowManager
	p      browserapi.EventPublisher
	exec   workspaceapi.Executor
	exit   atomic.Uint32
	ch     chan textapi.Event
	scroll struct {
		sync.Mutex
		scroll component.Scroll
	}
	gitDiffListID string
	delAttr       term.Attributes
	addAttr       term.Attributes
	tabspaces     int
	rows          map[string]int
	offsets       map[string]term.Coordinates
	lastLocs      map[string][]textapi.Location
}

func newGitHandler(
	ed textapi.Editor, grants []extension.Grant,
	broker proto.MuxBroker, pconfig config.Config,

) (extutil.CommandEventHandler, error) {
	ret := new(gitEditorHandler)
	ret.ed = ed
	ret.rows = make(map[string]int)
	ret.offsets = make(map[string]term.Coordinates)
	ret.ch = make(chan textapi.Event)
	ret.lastLocs = make(map[string][]textapi.Location)
	ret.scroll.scroll.Init(cell.NewBuffer())

	var err error
	ret.scroll.scroll.Attributes, err = config.GetAttributes(pconfig, "bar_attr")
	if err != nil {
		if err != config.ErrNotFound {
			log.Warningf("failed to get 'bar_attr' from config: %v", err)
		}
		ret.scroll.scroll.Attributes = defaultScrollAttr
	}

	var cwd workspaceapi.URI
	for _, grant := range grants {
		switch grant.Permission {
		case extension.PermissionFileSystem:
			fs, err := workspaceextension.FileSystem(grant, broker)
			if err != nil {
				return nil, err
			}
			cwdURI, err := fs.URI(".")
			if err != nil {
				return nil, err
			}
			cwd = cwdURI
		case extension.PermissionExecute:
			ret.exec, err = workspaceextension.Executor(grant, broker)
			if err != nil {
				return nil, err
			}
		case extension.PermissionBrowserEventPublisher:
			ret.p, err = browserextension.EventPublisher(grant, broker)
			if err != nil {
				return nil, err
			}
		case extension.PermissionBrowserWindowManager:
			ret.wm, err = browserextension.WindowManager(grant, broker)
			if err != nil {
				return nil, err
			}
			syncComp := component.Sync(&ret.scroll, &ret.scroll.scroll)
			err = ret.wm.Bar(browserapi.OrientationLeft, handler.Nop(syncComp))
			if err != nil {
				return nil, err
			}
		case extension.PermissionConfig:
			config, err := configextension.FetchConfig(grant, broker)
			if err != nil {
				return nil, err
			}
			ret.tabspaces, err = extutil.Tabspaces(config)
			if err != nil {
				ret.tabspaces = cell.DefaultTabspaces
				log.Warnf("Could not get tabspaces from config: %s.. Using default of %d",
					err, ret.tabspaces)
			}
		}
	}

	ret.gitDiffListID, err = pconfig.GetString("git_diff_list_id")
	if err != nil {
		if err != config.ErrNotFound {
			log.Warningf("failed to get 'git_diff_list_id' from config: %v", err)
		}
		ret.gitDiffListID = defaultGitDiffListID
	}

	ret.addAttr, err = config.GetAttributes(pconfig, "add_attr")
	if err != nil {
		if err != config.ErrNotFound {
			log.Warningf("failed to get 'add_attr' from config: %v", err)
		}
		ret.addAttr = defaultAddAttr
	}

	ret.delAttr, err = config.GetAttributes(pconfig, "del_attr")
	if err != nil {
		if err != config.ErrNotFound {
			log.Warningf("failed to get 'del_attr' from config: %v", err)
		}
		ret.delAttr = defaultDelAttr
	}

	go ret.handleEvents(cwd)

	return ret, nil
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
			err = fmt.Errorf("ed.MoveToNextLocation(%s): %v", cmd.Name, err)
		}
	case commandPrevChange:
		err = h.ed.MoveToPrevLocation(cmd.Resource, h.gitDiffListID)
		if err != nil {
			err = fmt.Errorf("ed.MoveToPrevLocation(%s): %v", cmd.Name, err)
		}
	}

	return
}

func (h *gitEditorHandler) parseDiff(diff *diff.FileDiff) []textapi.Location {
	h.scroll.Lock()
	defer h.scroll.Unlock()

	var locs []textapi.Location
	for _, hunk := range diff.Hunks {
		log.Tracef("Read file diff hunk: %#v", hunk)
		y := int(math.Max(0, float64(hunk.NewStartLine-1)))
		if hunk.NewLines == 0 {
			// FIXME https://github.com/ernestrc/go-tui/issues/59
			at := term.Coordinates{Y: y}
			locs = append(locs, textapi.Location{
				From: at,
				To:   term.Coordinates{Y: at.Y, X: 1},
				// Attr: deleteAttr,
			})
			h.scroll.scroll.Buffer().DeleteCell(at)
			h.scroll.scroll.Buffer().InsertStringWithAttr(at, "-", h.delAttr)
			continue
		}

		from := term.Coordinates{Y: y}
		to := term.Coordinates{Y: int(hunk.NewStartLine - 1 + hunk.NewLines)}
		locs = append(locs, textapi.Location{From: from, To: to})

		for y := from.Y; y < to.Y; y++ {
			at := term.Coordinates{Y: y}
			h.scroll.scroll.Buffer().DeleteCell(at)
			h.scroll.scroll.Buffer().InsertStringWithAttr(at, "+", h.addAttr)
		}
	}

	return locs
}

func (h *gitEditorHandler) pushNewDiffLocations(
	filename string, resource textapi.Handler,
) error {
	var out bytes.Buffer
	ch := make(chan error)
	cmd := workspaceapi.Cmd{
		Path:    "git",
		Args:    []string{"diff", "-U0", filename},
		Watcher: workspaceapi.ChanWatcher(ch),
		Stdout:  &out,
	}
	if _, err := h.exec.Start(cmd); err != nil {
		return fmt.Errorf("failed to start process: %v", err)
	}

	h.initScroll(filename)

	if err := <-ch; err != nil {
		return fmt.Errorf("process exit with non-zero status: %v", err)
	}

	r := diff.NewFileDiffReader(&out)
	diff, err := r.Read()
	if err != nil {
		// unfortunately diff lib doesn't chain errors properly
		if strings.Contains(err.Error(), io.EOF.Error()) {
			// make sure that an interrupt is called
			// so the new scroll bar is updated, when
			// focus switched to a file with no changes.
			err = h.p.Interrupt()
		} else {
			err = fmt.Errorf("failed to read from stdout: %w", err)
		}
	}

	if diff == nil {
		return err
	}

	locs := h.parseDiff(diff)
	h.lastLocs[filename] = locs

	serr := h.ed.SetLocationList(resource,
		textapi.LocationPriorityInfo, h.gitDiffListID, textapi.LocationSlice(locs))
	if err != nil {
		err = multierr.Append(err, fmt.Errorf("set locations: %w", serr))
	}
	return err
}

func (h *gitEditorHandler) resetScroll() {
	h.scroll.Lock()
	defer h.scroll.Unlock()
	h.scroll.scroll.Init(cell.NewBuffer())
}

func (h *gitEditorHandler) initScroll(name string) {
	h.scroll.Lock()
	defer h.scroll.Unlock()

	h.scroll.scroll.Init(cell.NewBuffer())
	rows, ok := h.rows[name]
	if !ok {
		log.Debugf("could not find max rows for file %s", name)
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
func (h *gitEditorHandler) setScrollMaxContent(resourceName string, ev textapi.Event) {
	rows := len(cell.StringToCells(ev.Content, h.tabspaces))
	// best effort until #59 is resolved
	rows *= 2
	rows += 100
	h.rows[resourceName] = rows
	log.Debugf("setScrollMaxContent(%s): %d", ev.URI, rows)
}

func (h *gitEditorHandler) setScrollOffset(filename string, pos term.Coordinates) {
	h.scroll.Lock()
	defer h.scroll.Unlock()
	defer log.Tracef("setScrollOffset(%s): %#v OK", filename, pos)

	// we're only interested in the vertical scroll
	pos.X = 0
	h.offsets[filename] = pos

	// TODO should set h.offsets anyway
	if h.scroll.scroll.Offset().Y == pos.Y {
		return
	}

	ok := h.scroll.scroll.SetOffset(pos)
	if !ok {
		log.Errorf("setScrollOffset(%s): %#v: could not set offset", filename, pos)
		return
	}
}

func (h *gitEditorHandler) pushLastDiffLocations(filename string, resource textapi.Handler) error {
	locs, ok := h.lastLocs[filename]
	if !ok {
		log.Debugf("pushLastDiffLocations(%s): no locations found", filename)
		return nil
	}

	log.Tracef("pushLastDiffLocations(%s): locations found: %#v", filename, locs)
	return h.ed.SetLocationList(resource, textapi.LocationPriorityInfo,
		h.gitDiffListID, textapi.LocationSlice(locs))
}

func (h *gitEditorHandler) handleEvents(cwd workspaceapi.URI) {
	for ev := range h.ch {
		var start time.Time
		if log.IsLevelEnabled(log.TraceLevel) {
			start = time.Now()
			log.Tracef("Handle(%#v)", ev.Type)
		}

		resourceName := ev.URI.Path()

		if cwd != (workspaceapi.URI{}) && !workspaceapi.HasPrefix(ev.URI, cwd) {
			log.Debugf("ignoring file that's not in active workspace %v", resourceName)
			h.resetScroll()
			if err := h.p.Interrupt(); err != nil {
				log.Errorf("interrupt: %v", err)
			}
			continue
		}

		var err error
		switch ev.Type {
		case textapi.EventTypeEdit:
			err = h.pushLastDiffLocations(resourceName, ev.Resource)
		case textapi.EventTypeOpen:
			h.setScrollOffset(resourceName, term.Coordinates{})
			fallthrough
		case textapi.EventTypeFlush:
			h.setScrollMaxContent(resourceName, ev)
			fallthrough
		case textapi.EventTypeFocus:
			err = h.pushNewDiffLocations(resourceName, ev.Resource)
		case textapi.EventTypeUnfocus:
			h.resetScroll()
			err = h.p.Interrupt()
		case textapi.EventTypeScroll:
			h.setScrollOffset(resourceName, ev.Start)
			err = h.p.Interrupt()
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
		return nil // already closed
	}
	close(h.ch)
	return nil
}
