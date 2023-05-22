package plugin

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
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
	browserapi "unstable.build/go-tui/api/browser"
	browserplugin "unstable.build/go-tui/api/browser/plugin"
	"unstable.build/go-tui/api/config"
	configplugin "unstable.build/go-tui/api/config/plugin"
	textapi "unstable.build/go-tui/api/text"
	workspaceapi "unstable.build/go-tui/api/workspace"
	workspaceplugin "unstable.build/go-tui/api/workspace/plugin"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/handler/search"
	"unstable.build/go-tui/plugin"
	plugutil "unstable.build/go-tui/plugin/util"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/term/color"
	"unstable.build/go-tui/text/vi"
)

const (
	defaultSemanticTokensListID = "chroma_syntax_highlighting"
	cmdSyntaxQuery              = "syntaxQuery"
)

var (
	errNotAvailable = errors.New("syntax tree parser not yet available for this language")
)

// Grantee returns this plugin's Grantee and the permissions required to run it.
func Grantee() (plugin.Grantee, []plugin.Permission) {
	return plugutil.NewEditorEventHandler(SyntaxHandlerCommands,
		newSyntaxHandler, SyntaxHandlerEvents,
		SyntaxHandlerPermissions...)
}

var (
	// SyntaxHandlerCommands returns the commands that this plugin is
	// interested in registering.
	SyntaxHandlerCommands = []string{cmdSyntaxQuery}

	// SyntaxHandlerEvents returns the events that this plugin is
	// interested in subscribing to.
	SyntaxHandlerEvents = []textapi.EventType{
		textapi.EventTypeOpen,
		textapi.EventTypeClose,
		textapi.EventTypeEdit,
	}

	// SyntaxHandlerPermissions are the required permissions for this
	// plugin to run.
	SyntaxHandlerPermissions = []plugin.Permission{
		plugin.PermissionBrowserWindowManager,
		plugin.PermissionBrowserResourceOpener,
		plugin.PermissionBrowserEventPublisher,
		plugin.PermissionBrowserMessenger,
		plugin.PermissionConfig,
		plugin.PermissionFileSystem,
		plugin.PermissionEditor,
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
	m                    browserapi.Messenger
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
	ed textapi.Editor, grants []plugin.Grant,
	broker proto.MuxBroker, pconfig config.Config,

) (plugutil.CommandEventHandler, error) {
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
		case plugin.PermissionBrowserEventPublisher:
			ret.p, err = browserplugin.EventPublisher(g, broker)
			if err != nil {
				return nil, err
			}
		case plugin.PermissionBrowserResourceOpener:
			ret.o, err = browserplugin.ResourceOpener(g, broker)
			if err != nil {
				return nil, err
			}
		case plugin.PermissionBrowserWindowManager:
			ret.wm, err = browserplugin.WindowManager(g, broker)
			if err != nil {
				return nil, err
			}
		case plugin.PermissionBrowserMessenger:
			ret.m, err = browserplugin.Messenger(g, broker)
			if err != nil {
				return nil, err
			}
		case plugin.PermissionFileSystem:
			ret.fs, err = workspaceplugin.FileSystem(g, broker)
			if err != nil {
				return nil, err
			}
			cwdURI, err := ret.fs.URI(".")
			if err != nil {
				return nil, err
			}
			ret.cwd = cwdURI.Path()
		case plugin.PermissionConfig:
			config, err := configplugin.FetchConfig(g, broker)
			if err != nil {
				return nil, err
			}
			ret.tabspaces, err = plugutil.Tabspaces(config)
			if err != nil {
				ret.tabspaces = cell.DefaultTabspaces
				log.Warnf("Could not get tabspaces from config: %s.. Using default of %d",
					err, ret.tabspaces)
			}
			ret.frame, err = plugutil.WindowManagerFrame(config)
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
		err = h.handleOpen(ev)
	case textapi.EventTypeEdit:
		err = h.handleEdit(ev)
	}
	if err != nil {
		log.Error(err)
	}
	if log.IsLevelEnabled(log.TraceLevel) {
		log.Tracef("Handle(%#v) in %s", ev.Type, time.Since(start))
	}
	return
}

func (h *syntaxHandler) HandleCommand(ctx context.Context, cmd textapi.Command) (
	exit bool, err error,
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
	edh, err := ed.Edit(workspaceapi.URI{}, viewBuffer)
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
		cells, ok := buf.Select(from, to)

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
		data, err := ioutil.ReadAll(f)
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
			log.Error(err)
			return
		}

		attrs := term.Attributes{Bg: term.AttrReverse, Fg: term.AttrReverse}
		loc := textapi.Location{From: from, To: to, Attr: attrs}
		ed.SetLocationList(edh, textapi.LocationPriorityInfo,
			locID, textapi.LocationSlice([]textapi.Location{loc}))
		ed.MoveToPrevLocation(edh, locID)
	}

	sh := search.Handler(list, func(text string) {
		done = true

		// liberate all tabs
		closeWin(top)()
		closeWin(bottom)()

		err := h.goToLocation(win, uri, textToLocation[text])
		if err != nil {
			h.m.SetMessage("search.Handler: %s", err)
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
	exit bool, err error,
) {
	query := strings.Join(cmd.Args, " ")
	if query == "" {
		return false, errors.New("empty query")
	}
	uri := cmd.URI.String()

	f, ok := h.files[uri]
	if !ok {
		return false, fmt.Errorf("could not find buffer for file %s", uri)
	}

	if f.tree == nil {
		return false, errNotAvailable
	}

	q, err := sitter.NewQuery([]byte(query), f.language)
	if err != nil {
		return false, fmt.Errorf("new query: %v", err)
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
	return false, h.browseNodes(cmd.URI, cmd.Window, f.language, nodes, &f.Buffer)
}

func (h *syntaxHandler) handleOpen(ev textapi.Event) error {
	filename := ev.URI.String()
	ext := filepath.Ext(filename)
	h.files[filename] = h.newFile(ev)
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

func (h *syntaxHandler) handleEdit(ev textapi.Event) error {
	err := h.editFile(ev)
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

	attr := color.RGBToAttribute(bg.Background.Red(), bg.Background.Green(), bg.Background.Blue())
	err := h.ed.SetDefaultAttributes(ed, term.Attributes{Fg: term.ColorDefault, Bg: attr})
	if err != nil {
		log.Errorf("SetDefaultAttributes: %v", err)
	}
}

func (h *syntaxHandler) editFile(ev textapi.Event) error {
	f, ok := h.files[ev.URI.String()]
	if !ok {
		return fmt.Errorf("could not find buffer for file %s", ev.URI.String())
	}
	f.Edit(ev.Start, ev.End, ev.Content)
	if f.parser != nil {
		// TODO edit tree rather than re-parsing everything every time
		f.tree = f.parser.Parse(nil /*f.tree*/, []byte(f.Buffer.String()))
	}
	return nil
}

func (h *syntaxHandler) newFile(ev textapi.Event) *file {
	f := new(file)
	f.Buffer.InitWithTabspaces(h.tabspaces)
	f.Scroll.Init(&f.Buffer)
	f.Buffer.WriteString(ev.Content)
	f.uri = ev.URI
	f.handler = ev.Resource

	filename := ev.URI.Path()
	parser, language, ok := newParser(filename)
	if ok {
		f.language = language
		f.parser = parser
		f.tree = f.parser.Parse(nil, []byte(ev.Content))
	}
	log.Debugf("found parser for file '%s': %v", filename, ok)

	return f
}

func (h *syntaxHandler) Close() error {
	return nil
}
