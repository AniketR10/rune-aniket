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
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alecthomas/chroma"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
	multierr "github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/tcell/v3"
	browserapi "unstable.build/go-tui/api/browser"
	browserextension "unstable.build/go-tui/api/browser/extension"
	"unstable.build/go-tui/api/config"
	configextension "unstable.build/go-tui/api/config/extension"
	textapi "unstable.build/go-tui/api/text"
	workspaceapi "unstable.build/go-tui/api/workspace"
	workspaceextension "unstable.build/go-tui/api/workspace/extension"
	"unstable.build/go-tui/cell"
	syntaxExtension "unstable.build/go-tui/cmd/extension_fuzzy_syntax/extension"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/component/notifications"
	"unstable.build/go-tui/extension"
	extutil "unstable.build/go-tui/extension/util"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/handler/search"
	"unstable.build/go-tui/rpc"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text/vi"
)

const (
	defaultSemanticTokensListID = "chroma_syntax_highlighting"
	cmdSyntaxQuery              = "syntaxQuery"
)

var (
	errNotAvailable = errors.New("syntax tree parser not yet available for this language")
)

// Grantee returns this extension's Grantee and the permissions required to run it.
func Grantee() (extension.Grantee, []extension.Permission) {
	return extutil.NewEditorEventHandler(SyntaxHandlerCommands,
		newSyntaxHandler, SyntaxHandlerEvents,
		SyntaxHandlerPermissions...)
}

var (
	// SyntaxHandlerCommands returns the commands that this extension is
	// interested in registering.
	SyntaxHandlerCommands = []textapi.CommandManual{
		{
			Name: cmdSyntaxQuery,
			Summary: "Fuzzy search custom symbols in the workspace's AST, using the given query. " +
				"Check tree-sitter's manual for more details " +
				"https://tree-sitter.github.io/tree-sitter/using-parsers#query-syntax.",
			Synopsis: "query",
		},
	}

	// SyntaxHandlerEvents returns the events that this extension is
	// interested in subscribing to.
	SyntaxHandlerEvents = []textapi.EventType{
		textapi.EventTypeOpen,
		textapi.EventTypeClose,
		textapi.EventTypeEdit,
	}

	// SyntaxHandlerPermissions are the required permissions for this
	// extension to run.
	SyntaxHandlerPermissions = []extension.Permission{
		extension.PermissionBrowserWindowManager,
		extension.PermissionBrowserResourceOpener,
		extension.PermissionBrowserEventPublisher,
		extension.PermissionBrowserNotifications,
		extension.PermissionConfig,
		extension.PermissionFileSystem,
		extension.PermissionEditor,
	}
)

type file struct {
	cell.Buffer
	component.Scroll
	uri      workspaceapi.URI
	handler  textapi.Handler
	parser   *sitter.Parser
	tree     *sitter.Tree
	language *sitter.Language
}

type syntaxHandler struct {
	ed                   textapi.Editor
	wm                   browserapi.WindowManager
	m                    browserapi.Notifications
	o                    browserapi.ResourceOpener
	p                    browserapi.EventPublisher
	fs                   workspaceapi.FileSystem
	semanticTokensListID string
	setBackgroundAttr    bool
	tabspaces            int
	frame                bool
	cwd                  string

	styles map[string]*chroma.Style
	files  map[string]*file
	lexers map[string]chroma.Lexer
}

func newSyntaxHandler(
	ctx context.Context, ed textapi.Editor, grants []extension.Grant,
	broker rpc.MuxBroker, pconfig config.Config,

) (extutil.CommandEventHandler, error) {
	ret := new(syntaxHandler)
	ret.ed = ed
	ret.files = make(map[string]*file)
	ret.lexers = make(map[string]chroma.Lexer)
	var configErrs, err error
	ret.semanticTokensListID, err = pconfig.GetString("location_list_id")
	if err != nil {
		if err != config.ErrNotFound {
			configErrs = multierr.Append(configErrs,
				fmt.Errorf("failed to get 'location_list_id' from config: %v", err))
		}
		ret.semanticTokensListID = defaultSemanticTokensListID
	}
	ret.setBackgroundAttr, err = pconfig.GetBool("set_background_attr")
	if err != nil {
		if err != config.ErrNotFound {
			configErrs = multierr.Append(configErrs,
				fmt.Errorf("failed to get 'set_background_attr' from config: %v", err))
		}
	}
	defaultStyle, err := pconfig.GetString("default_style")
	if err != nil {
		if err != config.ErrNotFound {
			configErrs = multierr.Append(configErrs,
				fmt.Errorf("failed to get 'default_style' from config: %v", err))
		}
	}

	styles.Fallback = newDefaultStyle()
	if defaultStyle != "" {
		style := styles.Get(defaultStyle)
		if style != nil {
			styles.Fallback = style
		} else {
			log.Warnf("Default style %q not found. Using default of %q instead",
				defaultStyle, styles.Fallback.Name)
		}
	}

	chromaStyles, err := pconfig.GetMap("styles")
	if err != nil {
		if err != config.ErrNotFound {
			configErrs = multierr.Append(configErrs,
				fmt.Errorf("failed to get 'styles' from config: %v", err))
		}
		chromaStyles = make(map[string]interface{})
	}
	ret.styles = make(map[string]*chroma.Style)
	ret.styles[".sixrc"] = styles.Get("doom-one")
	ret.styles[".sixdevrc"] = styles.Get("doom-one")
	ret.styles[".oxrc"] = styles.Get("doom-one")
	ret.styles[".oxdevrc"] = styles.Get("doom-one")

	for ext, styleIfc := range chromaStyles {
		styleStr, ok := styleIfc.(string)
		if !ok {
			configErrs = multierr.Append(configErrs,
				fmt.Errorf("expected 'styles' to be a map of file "+
					"extension string to style string: %v", err))
		}
		if styleStr == "disabled" {
			ret.styles[ext] = nil
			continue
		}
		style := styles.Get(styleStr)
		if style == nil {
			style = styles.Fallback
		}
		ret.styles[ext] = style
	}
	if configErrs != nil {
		log.Warning(configErrs)
	}
	for _, g := range grants {
		switch g.Permission {
		case extension.PermissionBrowserEventPublisher:
			ret.p, err = browserextension.EventPublisher(ctx, g, broker)
			if err != nil {
				return nil, err
			}
		case extension.PermissionBrowserResourceOpener:
			ret.o, err = browserextension.ResourceOpener(ctx, g, broker)
			if err != nil {
				return nil, err
			}
		case extension.PermissionBrowserWindowManager:
			ret.wm, err = browserextension.WindowManager(ctx, g, broker)
			if err != nil {
				return nil, err
			}
		case extension.PermissionBrowserNotifications:
			ret.m, err = browserextension.Notifications(ctx, g, broker)
			if err != nil {
				return nil, err
			}
		case extension.PermissionFileSystem:
			ret.fs, err = workspaceextension.FileSystem(ctx, g, broker)
			if err != nil {
				return nil, err
			}
			cwdURI, err := ret.fs.URI(".")
			if err != nil {
				return nil, err
			}
			ret.cwd = cwdURI.Path()
		case extension.PermissionConfig:
			config, err := configextension.FetchConfig(ctx, g, broker)
			if err != nil {
				return nil, err
			}
			ret.tabspaces, err = extutil.Tabspaces(config)
			if err != nil {
				ret.tabspaces = cell.DefaultTabspaces
				log.Warnf("Could not get tabspaces from config: %s.. Using default of %d",
					err, ret.tabspaces)
			}
			ret.frame, err = extutil.WindowManagerFrame(config)
			if err != nil {
				ret.tabspaces = cell.DefaultTabspaces
				log.Warnf("Could not get tabspaces from config: %s.. Using default of %d",
					err, ret.tabspaces)
			}
		}
	}

	err = setupConfigLexer()
	if err != nil {
		log.Warningf("Could not setup .sixrc config lexer")
	}
	log.Debugf("Using tabspaces %d:", ret.tabspaces)
	return ret, nil
}

func (h *syntaxHandler) Handle(
	ctx context.Context, ev textapi.Event,
) (exit bool) {
	var start time.Time
	if log.IsLevelEnabled(log.TraceLevel) {
		start = time.Now()
		log.Tracef("Handle(%#v)", ev.Type)
	}
	var err error
	switch ev.Type {
	case textapi.EventTypeClose:
		delete(h.files, ev.URI.String())
	case textapi.EventTypeOpen:
		err = h.handleOpen(ctx, ev)
	case textapi.EventTypeEdit:
		err = h.handleEdit(ctx, ev)
	}
	if err != nil {
		log.Error(err)
	}
	if log.IsLevelEnabled(log.TraceLevel) {
		log.Tracef("Handle(%#v) in %s", ev.Type, time.Since(start))
	}
	return
}

func (h *syntaxHandler) Complete(ctx context.Context, name string, args []string) (
	iterator.Iterator[string], error,
) {
	return iterator.FromSlice[string](nil), nil
}

func (h *syntaxHandler) HandleCommand(ctx context.Context, cmd textapi.Command) (
	err error,
) {
	switch cmd.Name {
	case cmdSyntaxQuery:
		return h.handleQuerySyntax(ctx, cmd)
	}
	return
}

func (h *syntaxHandler) convertStartEndPoints(
	n *sitter.Node, buf *cell.Buffer,
) (from, to term.Coordinates, err error) {
	c := buf.RawCells()
	start, end := n.StartPoint(), n.EndPoint()
	from, ok := cell.ConvertRuneCoordinates(c, int(start.Row), int(start.Column))
	if !ok {
		err = fmt.Errorf("convert points: failed to convert sitter 'start point "+
			" to term 'from' coordinates: point: %v", start)
		return
	}
	log.Tracef("convert points: converted sitter 'start' point "+
		" to term 'from' coordinates: point: %v, result: %v", start, from)

	to, ok = cell.ConvertRuneCoordinates(c, int(end.Row), int(end.Column))
	if !ok {
		err = fmt.Errorf("convert points: failed to convert sitter 'end' point "+
			" to term 'to' coordinates: point: %v", end)
		return
	}
	log.Tracef("convert points: converted sitter 'end' point "+
		" to term 'to' coordinates: point: %v, result: %v", end, to)
	return
}

func (h *syntaxHandler) handleGoTo(f *file, n *sitter.Node) error {
	pos, _, err := h.convertStartEndPoints(n, &f.Buffer)
	if err != nil {
		return err
	}

	if err := h.ed.SetCursor(f.handler, pos); err != nil {
		err = fmt.Errorf("set cursor %v: %v", f.uri, err)
		return err
	}
	return nil
}

func (h *syntaxHandler) goToLocation(
	win browserapi.Window, uri workspaceapi.URI, n *sitter.Node,
) error {
	f, ok := h.files[uri.String()]
	if !ok {
		return errors.New("file is not open")
	}

	buf, err := h.o.Open(uri)
	if err != nil {
		err = fmt.Errorf("browser open %v: %v", uri, err)
		return err
	}

	if err := h.handleGoTo(f, n); err != nil {
		return err
	}

	err = win.SetContent(buf)
	if err != nil && err != browserapi.ErrTabNotFree {
		err = fmt.Errorf("win.SetContent: %v", err)
		return err
	}
	return nil
}

func (h *syntaxHandler) browseNodes(
	uri workspaceapi.URI, win browserapi.Window,
	language *sitter.Language, nodes []*sitter.Node,
	buf *cell.Buffer,
) error {
	const locID = "sitter_highlight_loc"
	var (
		longestLocation int
		bottom, top     browserapi.Window
		done            bool
	)
	cfg := search.ListConfig{
		Algo:          search.FuzzyMatch,
		Interrupter:   h.p,
		CaseSensitive: false,
	}
	list := search.NewList(cfg)
	textToLocation := make(map[string]*sitter.Node)
	viewBuffer := cell.NewBuffer()
	ed := vi.Editor()
	edh, err := ed.Edit(workspaceapi.RandomURI("lexer"), viewBuffer)
	if err != nil {
		err = fmt.Errorf("edit temporary buffer: %s", err)
		return err
	}

	filename := uri.Path()
	relative, err := filepath.Rel(h.cwd, filename)
	if err == nil && len(relative) < len(filename) {
		filename = relative
	}

	for _, n := range nodes {
		from, to, err := h.convertStartEndPoints(n, buf)
		if err != nil {
			return err
		}

		// use firts line as helper text to display
		// along coordinates
		to.Y = from.Y
		to.X = buf.Columns(from.Y)
		cells, _, ok := buf.Select(from, to)

		// best effort
		var textToDisplay string
		if ok {
			textToDisplay = cell.CellsToString(cells)
		}
		text := fmt.Sprintf("%s:%d:%d-%d:%d: %s", filename,
			n.StartPoint().Row, n.StartPoint().Column,
			n.EndPoint().Row, n.EndPoint().Column, textToDisplay)
		list.PushSync([]byte(text))
		textToLocation[text] = n
		if len(text) > longestLocation {
			longestLocation = len(text)
		}
	}

	closeWin := func(win browserapi.Window) func() error {
		return func() error {
			shouldClose := done && win != nil
			closeWin := win
			if shouldClose {
				return closeWin.Close()
			}
			return nil
		}
	}

	renderFile := func(n *sitter.Node) {
		// Open assumes path in current workspace
		f, oerr := h.fs.Open(filename, os.O_RDONLY, 0)
		if oerr != nil {
			log.Errorf("could not render preview file: Open: %v", oerr)
			return
		}
		defer f.Close()
		data, err := io.ReadAll(f)
		if err != nil {
			log.Errorf("could not render preview file: Read: %v", err)
			return
		}
		content := string(data)

		_ = ed.SetCursor(edh, term.Coordinates{})
		viewBuffer.Reset()
		viewBuffer.WriteString(content)

		from, to, err := h.convertStartEndPoints(n, viewBuffer)
		if err != nil {
			log.Errorf("convert start end points: %v", err)
			return
		}

		attrs := term.Attributes{Attrs: tcell.AttrReverse}
		loc := textapi.Location{From: from, To: to, Attr: attrs}
		if err := ed.SetLocationList(edh, textapi.LocationPriorityInfo,
			locID, textapi.LocationSlice([]textapi.Location{loc})); err != nil {
			log.Errorf("editor set location list: %v", err)
			return
		}
		if err := ed.MoveToPrevLocation(edh, locID); err != nil {
			log.Errorf("editor move to prev location: %v", err)
			return
		}
	}

	sh := search.Handler(list, func(text string) {
		done = true

		// liberate all tabs
		_ = closeWin(top)()
		_ = closeWin(bottom)()

		err := h.goToLocation(win, uri, textToLocation[text])
		if err != nil {
			_ = h.m.Notify(notifications.LevelError, "go to location: %v", err)
		}
	})

	// wrap to detect when focus has changed
	// and re-render window.
	bh := handler.Wrap(sh, func(ev term.Event) (bool, bool) {
		before, _ := list.Focus()
		exit, handle := sh.Handle(ev)
		list.Wait()
		after, _ := list.Focus()
		afterStr := string(after.Data())
		if exit {
			done = true
		}
		if string(before.Data()) != afterStr {
			node, ok := textToLocation[afterStr]
			if ok {
				renderFile(node)
			}
		}
		return exit, handle
	})

	eh := handler.Wrap(edh, func(ev term.Event) (bool, bool) {
		if ev.Type == term.EventResize {
			list.Wait()
			focus, _ := list.Focus()
			node, ok := textToLocation[string(focus.Data())]
			if ok {
				renderFile(node)
			}
			return false, true
		}

		return edh.Handle(ev)
	})

	bhtop := browserapi.FuncHandler(eh, closeWin(bottom))
	top, err = h.wm.Split(browserapi.OrientationBottom, win, bhtop)
	if err != nil {
		err = fmt.Errorf("wm.split: %v", err)
		return err
	}

	bhbottom := browserapi.FuncHandler(bh, closeWin(top))
	bottom, err = h.wm.Split(browserapi.OrientationBottom, top, bhbottom)
	if err != nil {
		err = fmt.Errorf("wm.Split: %v", err)
		return err
	}
	return nil
}

func (h *syntaxHandler) handleQuerySyntax(ctx context.Context, cmd textapi.Command) (
	err error,
) {
	query := strings.Join(cmd.Args, " ")
	if query == "" {
		return errors.New("empty query")
	}
	uri := cmd.URI.String()

	f, ok := h.files[uri]
	if !ok {
		return fmt.Errorf("could not find buffer for file %s", uri)
	}

	if f.tree == nil {
		return errNotAvailable
	}

	q, err := sitter.NewQuery([]byte(query), f.language)
	if err != nil {
		return fmt.Errorf("new query: %v", err)
	}
	qc := sitter.NewQueryCursor()
	qc.Exec(q, f.tree.RootNode())

	var nodes []*sitter.Node
	for {
		m, ok := qc.NextMatch()
		if !ok {
			break
		}
		for _, c := range m.Captures {
			nodes = append(nodes, c.Node)
		}
	}
	return h.browseNodes(cmd.URI, cmd.Window, f.language, nodes, &f.Buffer)
}

func (h *syntaxHandler) handleOpen(ctx context.Context, ev textapi.Event) error {
	filename := ev.URI.String()
	ext := filepath.Ext(filename)
	h.files[filename] = h.newFile(ctx, ev)
	if _, ok := h.lexers[ext]; !ok {
		lexer := lexers.Match(filename)
		if lexer == nil {
			lexer = lexers.Fallback
		}
		h.lexers[ext] = lexer
		log.Debugf("Loaded lexer %v for extension %s", lexer, ext)
	}
	if h.setBackgroundAttr {
		h.setBackground(ev.URI, ev.Resource)
	}
	return h.checkSyntaxWithLexer(ev)
}

func (h *syntaxHandler) handleEdit(ctx context.Context, ev textapi.Event) error {
	err := h.editFile(ctx, ev)
	if err != nil {
		return err
	}
	return h.checkSyntaxWithLexer(ev)
}

func (h *syntaxHandler) checkSyntaxWithLexer(ev textapi.Event) error {
	f, ok := h.files[ev.URI.String()]
	if !ok {
		return fmt.Errorf("could not find buffer for file %s", ev.URI.String())
	}
	filename := ev.URI.Path()
	ext := filepath.Ext(filename)
	lexer, ok := h.lexers[ext]
	if !ok {
		log.Warningf("lexer for %s not found. Using fallback..", ext)
		lexer = lexers.Fallback
	}
	iterator, err := lexer.Tokenise(nil, f.String())
	if err != nil {
		log.Errorf("lexer tokenize %v: %v", ev.URI.Path(), err)
		return err
	}
	return h.setChromaTokenPositions(f, iterator)
}

func (h *syntaxHandler) setChromaTokenPositions(f *file, it chroma.Iterator) error {
	style, ok := h.getChromaStyle(f.uri)
	if !ok {
		log.Tracef("Not running lexer for file %s: extension disabled", f.uri)
		return nil
	}

	attrMap := styleToAttrMap(style, h.setBackgroundAttr)
	cells := f.RawCells()
	lines := chroma.SplitTokensIntoLines(it.Tokens())

	var x int
	var locations []textapi.Location
	for y, row := range lines {
		for _, token := range row {
			str := token.Value
			from, ok1 := cell.ConvertRuneCoordinates(cells, y, x)
			x += len(str)
			to, ok2 := cell.ConvertRuneCoordinates(cells, y, x)
			if !ok1 || !ok2 {
				log.Warnf("Failed to convert coordinates for file %s "+
					"at line %d col %d", f.uri, y, x)
				continue
			}
			attr := attrForToken(attrMap, token.Type)
			location := textapi.Location{Attr: attr, From: from, To: to}
			locations = append(locations, location)
		}
		x = 0
	}
	log.Debugf("Setting location list with %d locations", len(locations))
	err := h.ed.SetLocationList(f.handler, textapi.LocationPriorityInfo,
		h.semanticTokensListID, textapi.LocationSlice(locations))
	if err != nil {
		return fmt.Errorf("set location list %v: %v", f.uri, err)
	}
	return nil
}

func (h *syntaxHandler) getChromaStyle(file workspaceapi.URI) (*chroma.Style, bool) {
	ext := filepath.Ext(file.Path())
	style, ok := h.styles[ext]
	if !ok {
		style = styles.Fallback
	}
	// disabled for this extension
	if style == nil {
		return nil, false
	}
	return style, true
}

func (h *syntaxHandler) setBackground(file workspaceapi.URI, ed textapi.Handler) {
	style, ok := h.getChromaStyle(file)
	if !ok {
		log.Debugf("Not running lexer for file %s: extension disabled", file)
		return
	}
	bg := style.Get(chroma.Background)
	if bg.IsZero() {
		return
	}

	color := tcell.NewColor(int32(bg.Background.Red()), int32(bg.Background.Green()), int32(bg.Background.Blue()))
	err := h.ed.SetDefaultAttributes(ed, term.Attributes{Bg: color})
	if err != nil {
		log.Errorf("SetDefaultAttributes: %v", err)
	}
}

func (h *syntaxHandler) editFile(ctx context.Context, ev textapi.Event) (err error) {
	f, ok := h.files[ev.URI.String()]
	if !ok {
		err = fmt.Errorf("could not find buffer for file %s", ev.URI.String())
		return
	}
	f.Edit(ctx, ev.Start, ev.End, ev.Content)
	if f.parser != nil {
		// TODO edit tree rather than re-parsing everything every time
		f.tree, err = f.parser.ParseCtx(ctx, nil /*f.tree*/, []byte(f.Buffer.String()))
	}
	return
}

func (h *syntaxHandler) newFile(ctx context.Context, ev textapi.Event) *file {
	f := new(file)
	f.Buffer.InitWithTabspaces(h.tabspaces)
	f.Scroll.Init(&f.Buffer)
	f.Buffer.WriteString(ev.Content)
	f.uri = ev.URI
	f.handler = ev.Resource

	filename := ev.URI.Path()
	parser, language, ok := syntaxExtension.NewParser(filename)
	if ok {
		f.language = language
		f.parser = parser
		var err error
		f.tree, err = f.parser.ParseCtx(ctx, nil, []byte(ev.Content))
		if err != nil {
			log.Errorf("parser parse %q: %v", filename, err)
		}
	}
	log.Debugf("found parser for file '%s': %v", filename, ok)

	return f
}

func (h *syntaxHandler) Close() error {
	return nil
}
