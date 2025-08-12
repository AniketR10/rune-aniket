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
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/blue/logging"
	"github.com/unstablebuild/tcell/v3"
	"unstable.build/go-tui"
	"unstable.build/go-tui/api/browserapi"
	"unstable.build/go-tui/api/browserapi/browserext"
	"unstable.build/go-tui/api/config"
	configextension "unstable.build/go-tui/api/config/extension"
	"unstable.build/go-tui/api/extensionapi"
	"unstable.build/go-tui/api/textapi"
	"unstable.build/go-tui/api/workspaceapi"
	"unstable.build/go-tui/api/workspaceapi/workspaceext"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/extension"
	"unstable.build/go-tui/extension/extutil"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/rpc"
	"unstable.build/go-tui/term"
)

// Grantee returns this extension's Grantee.
func Grantee() (extension.Grantee, []extensionapi.Permission) {
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
		textapi.EventTypeClose,
		textapi.EventTypeEdit,
		textapi.EventTypeFlush,
		textapi.EventTypeCursor,
		textapi.EventTypeFocus,
		textapi.EventTypeUnfocus,
	}

	// FileBarHandlerPermissions are the required permissions for this
	// extension to run.
	FileBarHandlerPermissions = []extensionapi.Permission{
		extensionapi.Permission(extensionapi.PermissionBrowserWindowManager),
		extensionapi.Permission(extensionapi.PermissionInterrupt),
		extensionapi.Permission(extensionapi.PermissionEditor),
		extensionapi.PermissionCommands,
		extensionapi.PermissionConfig,
		extensionapi.Permission(extensionapi.PermissionFileSystem),
	}

	defaultScrollAttr     = term.Attributes{Fg: tcell.ColorDefault}
	defaultBackgroundAttr = term.Attributes{Fg: tcell.ColorDefault}
	defaultDirtyAttr      = term.Attributes{Fg: tcell.ColorYellow}
)

type truncatedScroll struct {
	component.Scroll
}

func (s *truncatedScroll) Resize(width, height int) {
	s.Scroll.Resize(width, height)
	text := s.Buffer().String()
	truncatedText := s.truncateLeft(text, s.Width(), '<')
	if truncatedText != text {
		s.Buffer().Reset()
		s.Init(s.Buffer())
		s.Buffer().WriteString(truncatedText)
	}
}

func (s *truncatedScroll) truncateLeft(text string, width int, overflowSymbol rune) string {
	if len(text) <= width || width < 2 {
		return text
	}
	idx := len(text) - width
	newText := text[idx:]
	if overflowSymbol != 0 {
		newText = string(overflowSymbol) + newText[1:]
	}
	return newText
}

const defaultDirtyIcon = "[+]"

type fileInfo struct {
	dirty bool
}

type symbRegexDef struct {
	start *regexp.Regexp // match where a symbol starts
	end   *regexp.Regexp // match where the symbol ends line(s) below the symbRegexDef.start match
}

var regexMap = map[string]symbRegexDef{
	".go": {
		start: regexp.MustCompile(`^func\s*(?:\([^\)]+\)\s*)?([a-zA-Z_][a-zA-Z0-9_]*)\s*\(`),
		end:   regexp.MustCompile(`^}`),
	},
	".ts": {
		start: regexp.MustCompile(
			`^(?:export\s+)?(?:async\s+)?(?:\w+\s*[:=]\s*)?(?:function\s+)?(\w+)\s*\(`),
		end: regexp.MustCompile(`^}`),
	},
}

func parseExtension(resourceURIPath string) (def symbRegexDef, ok bool) {
	ext := filepath.Ext(resourceURIPath)
	if v, ok := regexMap[ext]; ok {
		return v, true
	}
	return symbRegexDef{}, false
}

type symbInfo struct {
	name  string
	start int
	end   int
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
	tracker                 extutil.ResourceTracker
	dirtyIcon               string

	showFunctionName bool
	currSymbolName   string
	symbols          map[string][]symbInfo

	bar struct {
		sync.Mutex
		comp       component.Reference
		filename   truncatedScroll
		symbolname truncatedScroll
		coords     component.Scroll
	}
}

func newFileBarEditorHandler(
	ctx context.Context, ed textapi.Editor, grants []extension.Grant,
	broker rpc.MuxBroker, pconfig config.Config,
) (extutil.CommandEventHandler, error) {
	ret := new(fileBarEditorHandler)
	ret.ch = make(chan textapi.Event)

	var err error
	ret.filenameAttributes, err = config.GetAttributes(pconfig, "filename_attr")
	if err != nil {
		if err != config.ErrNotFound {
			log.Warningf("failed to get 'filename_attr' from config: %v", err)
		}
		ret.filenameAttributes = defaultScrollAttr
	}

	ret.dirtyIcon, err = pconfig.GetString("dirty_icon")
	if err != nil {
		if err != config.ErrNotFound {
			log.Warningf("failed to get 'dirty_icon' from config: %v", err)
		}
		ret.dirtyIcon = defaultDirtyIcon
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

	ret.showFunctionName, err = pconfig.GetBool("show_function_name")
	if err != nil {
		if err != config.ErrNotFound {
			log.Warningf("failed to get 'show_function_name' from config: %v", err)
		}
	}

	ret.initBar(component.Nop())

	for _, grant := range grants {
		switch grant.Permission {
		case extensionapi.Permission(extensionapi.PermissionInterrupt):
			ret.p, err = browserext.EventPublisher(ctx, grant, broker)
			if err != nil {
				return nil, err
			}
		case extensionapi.Permission(extensionapi.PermissionBrowserWindowManager):
			ret.wm, err = browserext.WindowManager(ctx, grant, broker)
			if err != nil {
				return nil, err
			}
			comp := component.Sync(&ret.bar, &ret.bar.comp)
			cfg := browserapi.BarConfig{
				Frame:       browserapi.BarFrameDefault,
				Size:        1,
				Orientation: browserapi.OrientationBottom,
			}
			err = ret.wm.Bar(cfg, handler.Nop(comp))
			if err != nil {
				return nil, err
			}
		case extensionapi.Permission(extensionapi.PermissionFileSystem):
			w, err := workspaceext.FileSystem(ctx, grant, broker)
			if err != nil {
				return nil, err
			}
			ret.cwd, err = w.URI(".")
			if err != nil {
				return nil, err
			}
		case extensionapi.PermissionConfig:
			config, err := configextension.FetchConfig(ctx, grant, broker)
			if err != nil {
				return nil, err
			}
			tabspaces, err := extutil.Tabspaces(config)
			if err != nil {
				return nil, fmt.Errorf("could not get tabspaces from config: %v", err)
			}
			wrap, err := extutil.Wrap(config)
			if err != nil {
				return nil, fmt.Errorf("could not get wrap mode from config: %v", err)
			}
			ret.tracker.Init(tabspaces, wrap)
		}
	}

	ret.bar.filename.Init(cell.NewBuffer())
	ret.bar.coords.Init(cell.NewBuffer())
	ret.bar.symbolname.Init(cell.NewBuffer())
	ret.symbols = make(map[string][]symbInfo, 0)

	go ret.handleEvents()

	return ret, nil
}

func (t *fileBarEditorHandler) Complete(ctx context.Context, name string, args []string) (
	iterator.Iterator[string], error,
) {
	return iterator.FromSlice[string](nil), nil
}

func (h *fileBarEditorHandler) HandleCommand(ctx context.Context, cmd textapi.Command) (
	err error,
) {
	return
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

func (h *fileBarEditorHandler) prettyFileName(resource workspaceapi.URI) string {
	return workspaceapi.RelPath(h.cwd, resource)
}

func (h *fileBarEditorHandler) resetBarContent() {
	h.bar.Lock()
	defer h.bar.Unlock()

	h.bar.filename.Buffer().Reset()
	h.bar.filename.Init(h.bar.filename.Buffer())
	h.bar.symbolname.Buffer().Reset()
	h.bar.symbolname.Init(h.bar.symbolname.Buffer())
	h.bar.coords.Buffer().Reset()
	h.bar.coords.Init(h.bar.coords.Buffer())
}

func (h *fileBarEditorHandler) refreshBarContent(ev textapi.Event) {
	h.resetBarContent()

	h.bar.Lock()
	defer h.bar.Unlock()

	res, ok := h.getResource(ev)
	if !ok {
		if h.showFunctionName {
			h.bar.symbolname.Buffer().Reset()
		}
		return
	}

	totalRows := res.Buffer().Rows()
	totalCols := 0
	cursor := res.Cursor()
	if cursor.Y < totalRows {
		totalCols = res.Buffer().Columns(cursor.Y)
	}
	coords := fmt.Sprintf("%d/%d %d/%d", cursor.X+1, totalCols, cursor.Y+1, totalRows)

	filename := h.prettyFileName(res.URI())
	h.bar.filename.Buffer().WriteString(filename)
	h.bar.coords.Buffer().WriteString(coords)

	if h.showDirty && res.Metadata.(*fileInfo).dirty {
		h.bar.filename.Attributes = h.filenameDirtyAttributes
		h.bar.filename.Buffer().WriteString(h.dirtyIcon)
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

	var grid tui.Component
	if h.showFunctionName {
		h.bar.symbolname.Buffer().Reset()
		h.bar.symbolname.Init(h.bar.symbolname.Buffer())
		h.bar.symbolname.Buffer().WriteString(h.currSymbolName)
		symbolSpanCfg := component.SpanConfig{
			ContentAlignment: component.SpanAlignmentHorizontallyCentered,
			PadHorizontal:    -h.bar.symbolname.Buffer().Columns(0),
		}
		symbolSpan := component.NewSpan(&h.bar.symbolname, symbolSpanCfg)
		grid = component.Grid([][]tui.Component{{fileSpan, symbolSpan, coordsSpan}})
	} else {
		grid = component.Grid([][]tui.Component{{fileSpan, coordsSpan}})
	}

	h.initBar(grid)
}

func (h *fileBarEditorHandler) initBar(c tui.Component) {
	h.bar.comp.Init(component.WithBackground(
		c, term.Cell{Attributes: h.backgroundAttributes},
	))
}

func (h *fileBarEditorHandler) setFileDirty(ev textapi.Event, dirty bool) {
	res, ok := h.getResource(ev)
	if !ok {
		return
	}
	res.Metadata.(*fileInfo).dirty = dirty
	log.Tracef("set file dirty (%s): %#v", ev.URI.Path(), dirty)
}

func (h *fileBarEditorHandler) handleEvents() {
	ctx := context.Background()
	for ev := range h.ch {
		if ev.URI == (workspaceapi.URI{}) {
			continue
		}

		var start time.Time
		if log.IsLevelEnabled(log.TraceLevel) {
			start = time.Now()
			h.log(log.TraceLevel, "handle %v", ev.Type)
		}

		h.tracker.Handle(ctx, ev)

		switch ev.Type {
		case textapi.EventTypeEdit:
			if h.showFunctionName {
				if res, ok := h.getResource(ev); ok {
					h.buildSymbolData(ev.URI, res.Scroll.Buffer().RawCells())
				}
			}
			h.setFileDirty(ev, true)
			h.refreshBarContent(ev)
			h.interrupt(ctx)
		case textapi.EventTypeOpen:
			res, _ := h.getResource(ev)
			res.Metadata = new(fileInfo)
			if h.showFunctionName {
				if res, ok := h.getResource(ev); ok {
					h.buildSymbolData(ev.URI, res.Scroll.Buffer().RawCells())
				}
			}
		case textapi.EventTypeClose:
			if h.showFunctionName {
				h.cleanSymbolData(ev.URI)
			}
		case textapi.EventTypeFlush:
			h.setFileDirty(ev, false)
			fallthrough
		case textapi.EventTypeFocus:
			if h.showFunctionName {
				h.resolveCursorSymbol(ev.URI, ev.From.Y)
			}
			h.refreshBarContent(ev)
			h.interrupt(ctx)
		case textapi.EventTypeUnfocus:
			if h.showFunctionName {
				h.currSymbolName = ""
			}
			h.resetBarContent()
			h.interrupt(ctx)
		case textapi.EventTypeCursor:
			if h.showFunctionName {
				h.resolveCursorSymbol(ev.URI, ev.From.Y)
			}
			h.refreshBarContent(ev)
			h.interrupt(ctx)
		}
		if log.IsLevelEnabled(log.TraceLevel) {
			h.log(log.TraceLevel, "handle %v in %s", ev.Type, time.Since(start))
		}
	}
}

func (h *fileBarEditorHandler) log(level log.Level, msg string, args ...any) {
	log.WithFields(log.Fields{
		logging.KeyClass: "extension.fileBarEditorHandler",
	}).Logf(level, msg, args...)
}

func (h *fileBarEditorHandler) interrupt(ctx context.Context) {
	if err := h.p.Interrupt(ctx); err != nil {
		h.log(log.ErrorLevel, "interrupt: %v", err)
	}
}

func (h *fileBarEditorHandler) getResource(ev textapi.Event) (
	*extutil.TrackedResource, bool,
) {
	res, ok := h.tracker.Resource(ev.URI)
	if !ok {
		h.log(log.ErrorLevel, "resource with uri %q not found in tracker",
			ev.URI.String())
	}
	return res, ok
}

func (h *fileBarEditorHandler) buildSymbolData(uri workspaceapi.URI, cells [][]term.Cell) {
	symbRegex, ok := parseExtension(uri.Path())
	if !ok {
		return
	}

	resourceName := uri.String()
	h.symbols[resourceName] = make([]symbInfo, 0)

	for line, row := range cells {
		rowStr := cell.CellsToString([][]term.Cell{row})

		// match for symbol start
		matches := symbRegex.start.FindStringSubmatch(rowStr)
		if len(matches) > 1 {
			h.symbols[resourceName] = append(h.symbols[resourceName], symbInfo{
				name:  matches[1],
				start: line,
			})

			// if the previous symbol was not closed, do so
			numSymb := len(h.symbols[resourceName])
			if numSymb > 1 && h.symbols[resourceName][numSymb-2].end == 0 {
				h.symbols[resourceName][numSymb-1].end = line
			}
			continue
		}

		// match for symbol end and close the last symbol
		if symbRegex.end.MatchString(rowStr) {
			numSymb := len(h.symbols[resourceName])
			if numSymb > 0 {
				h.symbols[resourceName][numSymb-1].end = line
			}
		}
	}
}

func (h *fileBarEditorHandler) resolveCursorSymbol(uri workspaceapi.URI, currLine int) {
	resourceName := uri.String()

	numSymb := len(h.symbols[resourceName])
	if numSymb == 0 {
		return
	}

	idx := sort.Search(numSymb, func(i int) bool {
		symb := h.symbols[resourceName][i]
		r := symb.end >= currLine
		return r
	})

	// sort.Search would return "numSymb" when search finds nothing.
	if idx < numSymb && h.symbols[resourceName][idx].start <= currLine {
		h.currSymbolName = h.symbols[resourceName][idx].name
	} else {
		h.currSymbolName = ""
	}
}

func (h *fileBarEditorHandler) cleanSymbolData(uri workspaceapi.URI) {
	resourceName := uri.String()
	h.symbols[resourceName] = make([]symbInfo, 0)
	h.currSymbolName = ""
}
