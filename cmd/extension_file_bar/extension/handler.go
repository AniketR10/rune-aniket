package extension

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui"
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

// Grantee returns this extension's Grantee.
func Grantee() (extension.Grantee, []extension.Permission) {
	return extutil.NewEditorEventHandler(FileBarHandlerCommands,
		newFileBarEditorHandler, FileBarHandlerEvents,
		FileBarHandlerPermissions...)
}

var (
	// FileBarHandlerCommands returns the commands that this extension is
	// interested in registering.
	FileBarHandlerCommands = []textapi.CommandManual{}

	// FileBarHandlerEvents returns the events that this extension is
	// interested in subscribing to.
	FileBarHandlerEvents = []textapi.EventType{
		textapi.EventTypeOpen,
		textapi.EventTypeEdit,
		textapi.EventTypeFlush,
		textapi.EventTypeCursor,
		textapi.EventTypeFocus,
		textapi.EventTypeUnfocus,
	}

	// FileBarHandlerPermissions are the required permissions for this
	// extension to run.
	FileBarHandlerPermissions = []extension.Permission{
		extension.Permission(extension.PermissionBrowserWindowManager),
		extension.Permission(extension.PermissionBrowserEventPublisher),
		extension.Permission(extension.PermissionEditor),
		extension.PermissionConfig,
		extension.Permission(extension.PermissionFileSystem),
	}

	defaultScrollAttr     = term.Attributes{Fg: term.ColorDefault}
	defaultBackgroundAttr = term.Attributes{Fg: term.ColorDefault}
	defaultDirtyAttr      = term.Attributes{Fg: term.ColorYellow}
)

type fileInfo struct {
	cells  [][]term.Cell
	offset term.Coordinates
	dirty  bool
}

type fileBarEditorHandler struct {
	wm   browserapi.WindowManager
	p    browserapi.EventPublisher
	cwd  workspaceapi.URI
	exit uint32
	ch   chan textapi.Event

	filenameAttributes      term.Attributes
	filenameDirtyAttributes term.Attributes
	backgroundAttributes    term.Attributes
	showDirty               bool
	tabspaces               int

	bar struct {
		sync.Mutex
		comp     component.Reference
		filename component.Scroll
		coords   component.Scroll
	}
	files map[string]*fileInfo
}

func newFileBarEditorHandler(
	ed textapi.Editor, grants []extension.Grant,
	broker proto.MuxBroker, pconfig config.Config,

) (extutil.CommandEventHandler, error) {
	ret := new(fileBarEditorHandler)
	ret.files = make(map[string]*fileInfo)
	ret.ch = make(chan textapi.Event)

	var err error
	ret.filenameAttributes, err = config.GetAttributes(pconfig, "filename_attr")
	if err != nil {
		if err != config.ErrNotFound {
			log.Warningf("failed to get 'filename_attr' from config: %v", err)
		}
		ret.filenameAttributes = defaultScrollAttr
	}

	ret.backgroundAttributes, err = config.GetAttributes(pconfig, "background_attr")
	if err != nil {
		if err != config.ErrNotFound {
			log.Warningf("failed to get 'background_attr' from config: %v", err)
		}
		ret.backgroundAttributes = defaultBackgroundAttr
	}

	ret.showDirty, err = pconfig.GetBool("show_dirty")
	if err != nil {
		if err != config.ErrNotFound {
			log.Warningf("failed to get 'show_dirty' from config: %v", err)
		}
		ret.showDirty = true
	}

	ret.filenameDirtyAttributes, err = config.GetAttributes(pconfig, "filename_dirty_attr")
	if err != nil {
		if err != config.ErrNotFound {
			log.Warningf("failed to get 'filename_dirty_attr' from config: %v", err)
		}
		ret.filenameDirtyAttributes = defaultDirtyAttr
	}

	ret.bar.coords.Attributes, err = config.GetAttributes(pconfig, "coordinates_attr")
	if err != nil {
		if err != config.ErrNotFound {
			log.Warningf("failed to get 'coordinates_attr' from config: %v", err)
		}
		ret.bar.coords.Attributes = defaultScrollAttr
	}

	for _, grant := range grants {
		switch grant.Permission {
		case extension.Permission(extension.PermissionBrowserEventPublisher):
			ret.p, err = browserextension.EventPublisher(grant, broker)
			if err != nil {
				return nil, err
			}
		case extension.Permission(extension.PermissionBrowserWindowManager):
			ret.wm, err = browserextension.WindowManager(grant, broker)
			if err != nil {
				return nil, err
			}
			comp := component.Sync(&ret.bar, &ret.bar.comp)
			err = ret.wm.Bar(browserapi.OrientationBottom, handler.Nop(comp))
			if err != nil {
				return nil, err
			}
		case extension.Permission(extension.PermissionFileSystem):
			w, err := workspaceextension.FileSystem(grant, broker)
			if err != nil {
				return nil, err
			}
			ret.cwd, err = w.URI(".")
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

	ret.bar.filename.Init(cell.NewBuffer())
	ret.bar.coords.Init(cell.NewBuffer())

	go ret.handleEvents()

	return ret, nil
}

func (h *fileBarEditorHandler) HandleCommand(ctx context.Context, cmd textapi.Command) (
	exit bool, err error,
) {
	return
}

func (h *fileBarEditorHandler) prettyFileName(resource workspaceapi.URI) string {
	return workspaceapi.RelPath(h.cwd, resource)
}

func (h *fileBarEditorHandler) resetBarContent() {
	h.bar.Lock()
	defer h.bar.Unlock()

	h.bar.filename.Buffer().Reset()
	h.bar.filename.Init(h.bar.filename.Buffer())
	h.bar.coords.Buffer().Reset()
	h.bar.coords.Init(h.bar.coords.Buffer())
}

func (h *fileBarEditorHandler) refreshBarContent(resource workspaceapi.URI) {
	h.resetBarContent()

	h.bar.Lock()
	defer h.bar.Unlock()

	file, ok := h.files[resource.String()]
	if !ok || file == nil {
		log.Debugf("could not find file info for file %q", resource.String())
		return
	}

	name := h.prettyFileName(resource)
	rows := len(file.cells)
	var cols int
	if file.offset.Y < len(file.cells) {
		cols = len(file.cells[file.offset.Y])
	}
	coords := fmt.Sprintf("%d/%d %d/%d", file.offset.X+1, cols, file.offset.Y+1, rows)

	h.bar.filename.Buffer().WriteString(name)
	h.bar.coords.Buffer().WriteString(coords)

	if h.showDirty && file.dirty {
		h.bar.filename.Attributes = h.filenameDirtyAttributes
		h.bar.filename.Buffer().WriteString("[+]")
	} else {
		h.bar.filename.Attributes = h.filenameAttributes
	}

	fileSpanCfg := component.SpanConfig{
		ContentAlignment: component.SpanAlignmentLeft,
		PadHorizontal:    -h.bar.filename.Buffer().Columns(0),
	}
	coordsSpanCfg := component.SpanConfig{
		ContentAlignment: component.SpanAlignmentRight,
		PadHorizontal:    -len(coords),
	}
	fileSpan := component.NewSpan(&h.bar.filename, fileSpanCfg)
	coordsSpan := component.NewSpan(&h.bar.coords, coordsSpanCfg)
	h.bar.comp.Init(component.WithBackground(
		component.Grid([][]tui.Component{{fileSpan, coordsSpan}}),
		term.Cell{Bg: h.backgroundAttributes.Bg, Fg: h.backgroundAttributes.Fg},
	))
}

func (h *fileBarEditorHandler) getFileInfo(resource workspaceapi.URI) *fileInfo {
	id := resource.String()
	f, ok := h.files[id]
	if ok {
		return f
	}
	ret := new(fileInfo)
	h.files[id] = ret
	return ret
}

func (h *fileBarEditorHandler) setScrollMaxContent(resource workspaceapi.URI, ev textapi.Event) {
	cells := cell.StringToCells(ev.Content, h.tabspaces)
	h.getFileInfo(resource).cells = cells
	log.Debugf("setScrollMaxContent(%s): %d", resource, len(cells))
}

func (h *fileBarEditorHandler) setCursorOffset(resource workspaceapi.URI, pos term.Coordinates) {
	h.getFileInfo(resource).offset = pos
	log.Tracef("setScrollOffset(%s): %#v OK", resource, pos)
}

func (h *fileBarEditorHandler) setFileDirty(resource workspaceapi.URI, dirty bool) {
	h.getFileInfo(resource).dirty = dirty
	log.Tracef("setFileDirty(%s): %#v OK", resource, dirty)
}

func (h *fileBarEditorHandler) handleEvents() {
	for ev := range h.ch {
		if ev.URI == (workspaceapi.URI{}) {
			continue
		}

		var start time.Time
		if log.IsLevelEnabled(log.TraceLevel) {
			start = time.Now()
			log.Tracef("Handle(%#v)", ev.Type)
		}

		resourceName := ev.URI
		var err error
		switch ev.Type {
		case textapi.EventTypeEdit:
			h.setFileDirty(resourceName, true)
			h.refreshBarContent(resourceName)
			err = h.p.Interrupt()
		case textapi.EventTypeOpen:
			h.setCursorOffset(resourceName, term.Coordinates{})
			h.setScrollMaxContent(resourceName, ev)
			h.refreshBarContent(resourceName)
			err = h.p.Interrupt()
		case textapi.EventTypeFlush:
			h.setFileDirty(resourceName, false)
			h.setScrollMaxContent(resourceName, ev)
			fallthrough
		case textapi.EventTypeFocus:
			h.refreshBarContent(resourceName)
			err = h.p.Interrupt()
		case textapi.EventTypeUnfocus:
			h.refreshBarContent(workspaceapi.URI{})
			err = h.p.Interrupt()
		case textapi.EventTypeCursor:
			h.setCursorOffset(resourceName, ev.From)
			h.refreshBarContent(resourceName)
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

func (h *fileBarEditorHandler) Handle(
	ctx context.Context, ev textapi.Event,
) (exit bool) {
	uexit := atomic.LoadUint32(&h.exit)
	exit = uexit != 0
	if exit {
		return
	}

	h.ch <- ev
	return
}

func (h *fileBarEditorHandler) Close() error {
	closing := atomic.CompareAndSwapUint32(&h.exit, 0, 1)
	if !closing {
		return nil // already closed
	}
	close(h.ch)
	return nil
}
