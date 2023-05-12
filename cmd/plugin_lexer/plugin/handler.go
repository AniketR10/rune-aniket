package plugin

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/alecthomas/chroma"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
	multierr "github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui/api/config"
	configplugin "unstable.build/go-tui/api/config/plugin"
	textapi "unstable.build/go-tui/api/text"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/plugin"
	plugutil "unstable.build/go-tui/plugin/util"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/term/color"
)

const (
	defaultSemanticTokensListID = "chroma_syntax_highlighting"
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
	SyntaxHandlerCommands = []string{}

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
		/* Editor perms already included */
		plugin.PermissionConfig,
	}
)

type file struct {
	cell.Buffer
	component.Scroll
	uri     workspaceapi.URI
	handler textapi.Handler
}

type syntaxHandler struct {
	ed                   textapi.Editor
	semanticTokensListID string
	setBackgroundAttr    bool
	tabspaces            int

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
	ret.styles[".hoprc"] = styles.Get("doom-one")
	ret.styles[".hopdevrc"] = styles.Get("doom-one")

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
	return
}

func (h *syntaxHandler) handleOpen(ev textapi.Event) error {
	h.files[ev.URI.String()] = h.newFile(ev)
	ext := filepath.Ext(ev.URI.Path())
	if _, ok := h.lexers[ext]; !ok {
		lexer := lexers.Match(ev.URI.Path())
		if lexer == nil {
			lexer = lexers.Fallback
		}
		h.lexers[ext] = lexer
		log.Debugf("Loaded lexer %v for extension %s", lexer, ext)
	}
	if h.setBackgroundAttr {
		h.setBackground(ev.URI, ev.Resource)
	}
	return h.checkSyntax(ev)
}

func (h *syntaxHandler) handleEdit(ev textapi.Event) error {
	err := h.editFile(ev)
	if err != nil {
		return err
	}
	return h.checkSyntax(ev)
}

func (h *syntaxHandler) checkSyntax(ev textapi.Event) error {
	f, ok := h.files[ev.URI.String()]
	if !ok {
		return fmt.Errorf("could not find buffer for file %s", ev.URI.String())
	}
	ext := filepath.Ext(ev.URI.Path())
	lexer, ok := h.lexers[ext]
	if !ok {
		log.Warningf("lexer for %s not found. Using fallback..", ext)
		lexer = lexers.Fallback
	}
	iterator, err := lexer.Tokenise(nil, f.String())
	if err != nil {
		log.Errorf("lexer.Tokenise(%s): %v", ev.URI.Path(), err)
		return err
	}
	return h.setTokenPositions(f, iterator)
}

func (h *syntaxHandler) setTokenPositions(f *file, it chroma.Iterator) error {
	style, ok := h.getStyle(f.uri)
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
		return fmt.Errorf("SetLocationList(%s): %v", f.uri, err)
	}
	return nil
}

func (h *syntaxHandler) getStyle(file workspaceapi.URI) (*chroma.Style, bool) {
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
	style, ok := h.getStyle(file)
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
	return nil
}

func (h *syntaxHandler) newFile(ev textapi.Event) *file {
	f := new(file)
	f.Buffer.InitWithTabspaces(h.tabspaces)
	f.Scroll.Init(&f.Buffer)
	f.Buffer.WriteString(ev.Content)
	f.uri = ev.URI
	f.handler = ev.Resource

	return f
}

func (h *syntaxHandler) Close() error {
	return nil
}
