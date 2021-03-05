package main

//go:generate mockgen -destination=./lsp_server_gomock.go -package main -self_package main github.com/ernestrc/golang-internal-tools/lsp/protocol Server

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/editor"
	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/plugin"
	plugutil "github.com/ernestrc/go-tui/plugin/util"
	"github.com/ernestrc/go-tui/proto"
	"github.com/ernestrc/go-tui/term"
	"github.com/ernestrc/golang-internal-tools/fakenet"
	"github.com/ernestrc/golang-internal-tools/jsonrpc2"
	"github.com/ernestrc/golang-internal-tools/lsp"
	"github.com/ernestrc/golang-internal-tools/lsp/lsprpc"
	"github.com/ernestrc/golang-internal-tools/lsp/protocol"
	"github.com/ernestrc/golang-internal-tools/lsp/source"
	"github.com/ernestrc/golang-internal-tools/span"
	log "github.com/sirupsen/logrus"
)

const (
	maxHoverColumns          = 90
	defaultRpcTimeout        = 10 * time.Second
	defaultConnectTimeout    = 10 * time.Second
	defaultDisconnectTimeout = 1 * time.Second
	firstFileVersion         = 1
	commandNextDiagnostic    = "lspNextDiagnostic"
	commandPrevDiagnostic    = "lspPrevDiagnostic"
	commandHover             = "lspHover"
	commandGoToDef           = "lspGoToDefinition"
	commandAddWorkspace      = "lspAddWorkspaceFolder"
	commandRemoveWorkspace   = "lspRemoveWorkspaceFolder"
	handleBackpressureEvs    = 64
)

var (
	lspHandlerCommands = []string{
		commandNextDiagnostic, commandPrevDiagnostic,
		commandHover, commandGoToDef, commandAddWorkspace,
		commandRemoveWorkspace,
	}
	lspHandlerPermissions = []plugin.Permission{
		plugin.PermissionBrowserWindowManager,
		plugin.PermissionBrowserResourceOpener,
		plugin.PermissionBrowserMessenger,
	}
	defaultSemanticTokensListID = "lsp_syntax_highlighting"
	defaultDiagnosticListID     = "lsp_diagnostic"
	defaultDiagnosticAttr       = map[protocol.DiagnosticSeverity]term.Attributes{
		protocol.SeverityError:       {Bg: term.ColorRed, Fg: term.ColorWhite},
		protocol.SeverityWarning:     {Bg: term.ColorYellow, Fg: term.ColorBlack},
		protocol.SeverityInformation: {Bg: term.ColorBlue, Fg: term.ColorWhite},
		protocol.SeverityHint:        {Bg: term.ColorGreen, Fg: term.ColorWhite},
	}
	defaultSemanticTypeAttr = map[string]term.Attributes{
		"namespace":     {},
		"type":          {},
		"class":         {},
		"enum":          {},
		"interface":     {Fg: term.ColorYellow},
		"struct":        {},
		"typeParameter": {},
		"parameter":     {},
		"variable":      {},
		"property":      {},
		"enumMember":    {},
		"event":         {},
		"function":      {},
		"member":        {},
		"macro":         {},
		"keyword":       {Fg: term.ColorYellow},
		"modifier":      {Fg: term.ColorYellow},
		"comment":       {Fg: term.ColorBlue},
		"string":        {Fg: term.ColorMagenta},
		"number":        {Fg: term.ColorRed},
		"regexp":        {},
		"operator":      {},
	}

	matcherString = map[source.SymbolMatcher]string{
		source.SymbolFuzzy:           "fuzzy",
		source.SymbolCaseSensitive:   "caseSensitive",
		source.SymbolCaseInsensitive: "caseInsensitive",
	}
)

type file struct {
	name       string
	languageID string
	handler    editor.Handler
	docID      protocol.TextDocumentIdentifier
	uri        span.URI

	// handler use getters
	_version     int32
	_cells       [][]term.Cell
	_diagnostics []protocol.Diagnostic
}

type execServer struct {
	langID string
	cmd    *exec.Cmd
	srv    protocol.Server
	caps   protocol.ServerCapabilities
}

type lspEditorHandler struct {
	mu     sync.Mutex
	evChan chan editor.Event

	ed editor.Editor
	wm browser.WindowManager
	m  browser.Messenger
	o  browser.ResourceOpener

	semanticTypesAttr    map[string]term.Attributes
	diagnosticAttr       map[protocol.DiagnosticSeverity]term.Attributes
	semanticTokensListID string
	diagnosticListID     string
	rpcTimeout           time.Duration
	connectTimeout       time.Duration
	disconnectTimeout    time.Duration

	exit              bool
	files             map[span.URI]*file
	pendingDiagnostic map[span.URI][]protocol.Diagnostic
	pendingGoTo       map[span.URI]protocol.Range
	servers           map[string]execServer
	cancelTokensReq   func()
}

func sendInitializeRequest(
	ctx context.Context, cwd string, server execServer,
) (*protocol.InitializeResult, error) {
	params := &protocol.ParamInitialize{}
	// NOTE: gopls doesn't respect workspaces if rootURI is set
	if server.langID == ".go" {
		params.Capabilities.Workspace.WorkspaceFolders = true
		params.WorkspaceFolders = []protocol.WorkspaceFolder{makeWorkspaceFolder(cwd)}
	} else {
		params.RootURI = protocol.URIFromPath(cwd)
		params.RootPath = cwd
		params.Path = params.RootPath // backwards compat with tsserver
	}

	params.Capabilities.Workspace.Configuration = false
	params.Capabilities.Workspace.Symbol.SymbolKind.ValueSet = []protocol.SymbolKind{}

	// Make sure to respect configured options when sending initialize request.
	opts := source.DefaultOptions().Clone()

	params.Capabilities.TextDocument.Hover = protocol.HoverClientCapabilities{
		ContentFormat: []protocol.MarkupKind{opts.PreferredContentFormat},
	}
	params.Capabilities.TextDocument.CodeAction.CodeActionLiteralSupport.CodeActionKind.ValueSet = []protocol.CodeActionKind{}
	params.Capabilities.TextDocument.CodeAction.ResolveSupport.Properties = []string{}
	params.Capabilities.TextDocument.Completion.CompletionItem.TagSupport.ValueSet = []protocol.CompletionItemTag{}
	params.Capabilities.TextDocument.Completion.CompletionItem.ResolveSupport.Properties = []string{}
	params.Capabilities.TextDocument.Completion.CompletionItem.InsertTextModeSupport.ValueSet = []protocol.InsertTextMode{}
	params.Capabilities.TextDocument.TypeDefinition.LinkSupport = false
	params.Capabilities.TextDocument.DocumentSymbol.HierarchicalDocumentSymbolSupport = opts.HierarchicalDocumentSymbolSupport
	params.Capabilities.TextDocument.DocumentSymbol.SymbolKind.ValueSet = []protocol.SymbolKind{}
	params.Capabilities.TextDocument.DocumentSymbol.TagSupport.ValueSet = []protocol.SymbolTag{}
	params.Capabilities.TextDocument.SemanticTokens.Formats = []string{"relative"}
	params.Capabilities.TextDocument.SemanticTokens.Requests.Range = true
	params.Capabilities.TextDocument.SemanticTokens.Requests.Full = true
	params.Capabilities.TextDocument.SemanticTokens.TokenTypes = lsp.SemanticTypes()
	params.Capabilities.TextDocument.SemanticTokens.TokenModifiers = lsp.SemanticModifiers()
	params.Capabilities.TextDocument.PublishDiagnostics.TagSupport.ValueSet = []protocol.DiagnosticTag{protocol.Unnecessary}
	params.Capabilities.TextDocument.PublishDiagnostics.VersionSupport = true
	params.InitializationOptions = map[string]interface{}{
		"symbolMatcher":  matcherString[opts.SymbolMatcher],
		"semanticTokens": true,
	}

	res, err := server.srv.Initialize(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("lsp.Server.Initialize: %v", err)
	}
	err = server.srv.Initialized(ctx, &protocol.InitializedParams{})
	if err != nil {
		return nil, fmt.Errorf("lsp.Server.Initialized: %v", err)
	}

	return res, nil
}

// parseAddr parses listen into a network, and address.
func parseAddr(listen string) (network string, address string) {
	if listen == lsprpc.AutoNetwork {
		return lsprpc.AutoNetwork, ""
	}
	if parts := strings.SplitN(listen, ";", 2); len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "tcp", listen
}

func streamRPC(cc jsonrpc2.Conn, h *lspEditorHandler) {
	ch := lspClientHandler{h: h}
	ctx := context.Background()

	cc.Go(ctx,
		protocol.Handlers(protocol.ClientHandler(&ch, jsonrpc2.MethodNotFound)))
	<-cc.Done()
	err := cc.Err()
	if err != nil {
		log.Errorf("jsonrpc2 processing goroutine error: %v", err)
	}
}

func initializeConnection(ctx context.Context, ret *lspEditorHandler, conn net.Conn) (
	protocol.Server, error,
) {
	stream := jsonrpc2.NewHeaderStream(conn)
	cc := jsonrpc2.NewConn(stream)
	server := protocol.ServerDispatcher(cc)
	go streamRPC(cc, ret)

	return server, nil
}

func getPipes(cmd *exec.Cmd) (
	io.WriteCloser, io.ReadCloser, io.ReadCloser, error,
) {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create stdout pipe: %v", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create stderr pipe: %v", err)
	}

	return stdin, stdout, stderr, nil
}

func parseCmd(arg interface{}) (*exec.Cmd, error) {
	str, ok := arg.(string)
	cmd := strings.Split(str, " ")
	if !ok || len(cmd) == 0 {
		return nil, fmt.Errorf("invalid command: %v", arg)
	}

	return exec.Command(cmd[0], cmd[1:]...), nil
}

func logStderr(langID string, stderr io.ReadCloser) {
	reader := bufio.NewReader(stderr)
	for {
		line, err := reader.ReadString('\n')
		log.Debugf("%s: %s", langID, line)
		if err != nil {
			if err != io.EOF {
				log.Errorf("failed to read from '%v' server stderr: %v", langID, err)
			}
			return
		}
	}
}

type nopWriter struct {
	io.Writer
}

func (w nopWriter) Close() error {
	return nil
}

func startLanguageServer(
	h *lspEditorHandler, ret map[string]execServer, langID string, arg interface{},
) {
	ctx := context.Background()
	ctx, cancelFn := context.WithTimeout(ctx, h.connectTimeout)
	defer cancelFn()

	c, err := parseCmd(arg)
	if err != nil {
		log.Errorf("failed to parse language %s command: %v", langID, err)
		return
	}

	stdin, stdout, stderr, err := getPipes(c)
	if err != nil {
		log.Errorf("failed to create net.Conn for '%s': %v", langID, err)
		return
	}

	log.Debugf("Starting lsp server '%s' with cmd: %#v", langID, c)
	err = c.Start()
	if err != nil {
		log.Errorf("failed to start exec for '%s': %v", langID, err)
		return
	}

	// helps debug
	if log.IsLevelEnabled(log.DebugLevel) {
		go logStderr(langID, stderr)
	}

	reader := ioutil.NopCloser(stdout)
	writer := nopWriter{Writer: stdin}
	conn := fakenet.NewConn("stdio", reader, writer)
	server, err := initializeConnection(ctx, h, conn)
	if err != nil {
		log.Errorf("failed to initialize LSP server for '%s': %v", langID, err)
		return
	}

	cwd, err := os.Getwd()
	if err != nil {
		log.Errorf("Getwd: %v", err)
		return
	}
	srv := execServer{langID: langID, cmd: c, srv: server}
	initRes, err := sendInitializeRequest(ctx, cwd, srv)
	if err != nil {
		return
	}

	srv.caps = initRes.Capabilities

	h.mu.Lock()
	ret[langID] = srv
	h.mu.Unlock()

	log.Infof("connected to '%s' LSP server '%s' with version %s: ",
		langID, initRes.ServerInfo.Name, initRes.ServerInfo.Version)
}

func startLanguageServers(
	h *lspEditorHandler, ret map[string]execServer, cfg map[string]interface{},
) {
	for langID, v := range cfg {
		go startLanguageServer(h, ret, langID, v)
	}
}

func initLanguageServers(h *lspEditorHandler, pconfig plugin.Config) (
	map[string]execServer, error,
) {
	cfg, err := pconfig.GetMap("exec")
	if err != nil {
		err = fmt.Errorf("Failed to get remote lsp server address: %v", err)
		return nil, err
	}

	ret := make(map[string]execServer)

	startLanguageServers(h, ret, cfg)

	return ret, nil
}

func getSemanticTypesAttr(pconfig plugin.Config) (map[string]term.Attributes, error) {
	ret := make(map[string]term.Attributes, len(defaultSemanticTypeAttr))
	for k, v := range defaultSemanticTypeAttr {
		ret[k] = v
	}

	colors, err := pconfig.GetConfig("syntax_highlighting")
	if err != nil {
		if err != plugin.ErrNotFound {
			err = fmt.Errorf("Error getting 'syntax_highlighting' from plugin config: %v", err)
			return nil, err
		}
		return ret, nil
	}

	for semanticType := range defaultSemanticTypeAttr {
		attr, err := colors.GetAttributes(semanticType)
		if err != nil {
			if err != plugin.ErrNotFound {
				err = fmt.Errorf("Error getting 'syntax_highlighting.%s' "+
					"from plugin config: %v", semanticType, err)
				return nil, err
			}
			continue
		}
		ret[semanticType] = attr
	}

	return ret, nil
}

func severityToString(s protocol.DiagnosticSeverity) string {
	switch s {
	case protocol.SeverityError:
		return "error"
	case protocol.SeverityWarning:
		return "warning"
	case protocol.SeverityInformation:
		return "info"
	case protocol.SeverityHint:
		return "hint"
	default:
		return ""
	}
}

func getDiagnosticAttr(pconfig plugin.Config) (
	map[protocol.DiagnosticSeverity]term.Attributes, error,
) {
	ret := make(map[protocol.DiagnosticSeverity]term.Attributes, len(defaultDiagnosticAttr))
	for k, v := range defaultDiagnosticAttr {
		ret[k] = v
	}

	colors, err := pconfig.GetConfig("diagnostics")
	if err != nil {
		if err != plugin.ErrNotFound {
			err = fmt.Errorf("Error getting 'diagnostics' from plugin config: %v", err)
			return nil, err
		}
		return ret, nil
	}

	for s := range defaultDiagnosticAttr {
		name := severityToString(s)
		attr, err := colors.GetAttributes(name)
		if err != nil {
			if err != plugin.ErrNotFound {
				err = fmt.Errorf("Error getting 'diagnostics.%s' "+
					"from plugin config: %v", name, err)
				return nil, err
			}
			continue
		}
		ret[s] = attr
	}

	return ret, nil
}

func convertRange(
	rng protocol.Range, cells [][]term.Cell, colmap protocol.ColumnMapper,
) (from, to term.Coordinates, ok bool) {
	spn, err := colmap.RangeSpan(rng)
	if err != nil {
		log.Errorf("lspEditorHandler: failed to create rangespan for range: %#v->%#v: %v",
			rng.Start, rng.End, err)
		return
	}

	startLine := spn.Start().Line() - 1
	startChar := spn.Start().Column() - 1
	from, ok = cell.ConvertRuneCoordinates(cells, startLine, startChar)
	if !ok {
		log.Errorf("lspEditorHandler: failed to convert lsp Start coordinates"+
			" to term From coordinates: %#v->%#v", startLine, startChar)
		return
	}

	// to is right exclusive, if result is negative then it's probably
	// not a token we're interested in
	endChar := int(math.Max(float64(spn.End().Column()-2), 0))
	endLine := spn.End().Line() - 1

	to, ok = cell.ConvertRuneCoordinates(cells, endLine, endChar)
	if !ok {
		log.Errorf("lspEditorHandler: failed to convert lsp End coordinates to "+
			"term To coordinates: %#v->%#v", endLine, endChar)
	}
	return
}

func getDuration(
	pconfig plugin.Config, key string, def time.Duration,
) (time.Duration, error) {
	durStr, err := pconfig.GetString(key)
	if err != nil {
		if err != plugin.ErrNotFound {
			err = fmt.Errorf("Error getting '%s' from plugin config: %v", key, err)
			return 0, err
		}
		return def, nil
	}

	duration, err := time.ParseDuration(durStr)
	if err != nil {
		err = fmt.Errorf("Error parsing duration '%s' from plugin config: %v", key, err)
		return 0, err
	}

	return duration, nil
}

func newLspHandler(
	ed editor.Editor, grants []plugin.Grant,
	broker proto.MuxBroker, pconfig plugin.Config,

) (plugutil.CommandEventHandler, error) {
	ret := new(lspEditorHandler)
	ret.ed = ed
	ret.files = make(map[span.URI]*file)
	ret.pendingDiagnostic = make(map[span.URI][]protocol.Diagnostic)
	ret.pendingGoTo = make(map[span.URI]protocol.Range)
	ret.evChan = make(chan editor.Event, handleBackpressureEvs)

	var err error
	ret.semanticTypesAttr, err = getSemanticTypesAttr(pconfig)
	if err != nil {
		return nil, err
	}

	ret.diagnosticAttr, err = getDiagnosticAttr(pconfig)
	if err != nil {
		return nil, err
	}

	ret.semanticTokensListID, err = pconfig.GetString("semantic_tokens_list_id")
	if err != nil {
		if err != plugin.ErrNotFound {
			err = fmt.Errorf("failed to get 'semantic_tokens_list_id' from config: %v", err)
			return nil, err
		}
		ret.semanticTokensListID = defaultSemanticTokensListID
	}

	ret.diagnosticListID, err = pconfig.GetString("diagnostic_list_id")
	if err != nil {
		if err != plugin.ErrNotFound {
			err = fmt.Errorf("failed to get 'diagnostic_list_id' from config: %v", err)
			return nil, err
		}
		ret.diagnosticListID = defaultDiagnosticListID
	}

	ret.connectTimeout, err = getDuration(pconfig,
		"connect_timeout", defaultConnectTimeout)
	if err != nil {
		return nil, err
	}

	ret.disconnectTimeout, err = getDuration(pconfig,
		"disconnect_timeout", defaultDisconnectTimeout)
	if err != nil {
		return nil, err
	}

	ret.rpcTimeout, err = getDuration(pconfig,
		"rpc_timeout", defaultRpcTimeout)
	if err != nil {
		return nil, err
	}

	ret.servers, err = initLanguageServers(ret, pconfig)
	if err != nil {
		return nil, err
	}

	for _, g := range grants {
		switch g.Permission {
		case plugin.PermissionBrowserResourceOpener:
			ret.o, err = plugin.ResourceOpener(g.Token, broker)
			if err != nil {
				return nil, err
			}
		case plugin.PermissionBrowserWindowManager:
			ret.wm, err = plugin.WindowManager(g.Token, broker)
			if err != nil {
				return nil, err
			}
		case plugin.PermissionBrowserMessenger:
			ret.m, err = plugin.Messenger(g.Token, broker)
			if err != nil {
				return nil, err
			}
		}
	}

	go ret.handleEvents(ret.evChan)

	return ret, nil
}

func (h *lspEditorHandler) getServer(languageID string) (
	execServer, bool,
) {
	h.mu.Lock()
	defer h.mu.Unlock()

	proc, ok := h.servers[languageID]
	if !ok {
		return execServer{}, false
	}

	return proc, true
}

func (h *lspEditorHandler) newFile(handler editor.Handler, name, content string) *file {
	h.mu.Lock()
	defer h.mu.Unlock()

	uri := span.URIFromPath(name)
	languageID := filepath.Ext(uri.Filename())

	f := &file{
		_version: firstFileVersion,
		handler:  handler,
		name:     name,
		docID: protocol.TextDocumentIdentifier{
			URI: protocol.URIFromSpanURI(uri),
		},
		uri:        uri,
		_cells:     cell.StringToCells(content),
		languageID: languageID,
	}

	h.files[uri] = f
	return f
}

func (h *lspEditorHandler) removePendingGoTo(uri span.URI) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.pendingGoTo, uri)
}

func (h *lspEditorHandler) addPendingGoTo(
	uri span.URI, rs protocol.Range,
) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.pendingGoTo[uri] = rs
}

func (h *lspEditorHandler) addPendingDiagnostics(
	uri span.URI, ds []protocol.Diagnostic,
) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.pendingDiagnostic[uri] = ds
}

func (h *lspEditorHandler) dispatchPendingDiagnostics(
	ctx context.Context, uri span.URI,
) {
	h.mu.Lock()
	ds, ok := h.pendingDiagnostic[uri]
	delete(h.pendingDiagnostic, uri)
	h.mu.Unlock()
	if !ok {
		return
	}

	h.handleDiagnostics(ctx, uri, ds, firstFileVersion)
}
func (h *lspEditorHandler) getColumnMapper(f *file) protocol.ColumnMapper {
	buf := cell.CellsToBuffer(h.getCells(f))
	content := []byte(buf.String())
	tc := span.NewContentConverter(f.uri.Filename(), content)
	return protocol.ColumnMapper{
		URI:       f.uri,
		Content:   content,
		Converter: tc,
	}
}

func (h *lspEditorHandler) handleGoTo(f *file, rs protocol.Range) {
	colmap := h.getColumnMapper(f)
	pos, _, ok := convertRange(rs, h.getCells(f), colmap)
	if !ok {
		return
	}

	err := h.ed.SetCursor(f.handler, pos)
	if err != nil {
		log.Errorf("lspEditorHandler.SetCursor(%s): %v", f.name, err)
	}
}

func (h *lspEditorHandler) dispatchPendingGoTo(
	ctx context.Context, srv execServer, f *file,
) {
	uri := f.uri

	h.mu.Lock()
	rs, ok := h.pendingGoTo[uri]
	delete(h.pendingGoTo, uri)
	h.mu.Unlock()
	if !ok {
		return
	}

	h.handleGoTo(f, rs)
}

func (h *lspEditorHandler) getFileWithName(name string) (*file, bool) {
	uri := span.URIFromPath(name)
	return h.getFile(uri)
}

func (h *lspEditorHandler) getFile(uri span.URI) (*file, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	f, ok := h.files[uri]
	return f, ok
}

func parseLocationData(
	uri span.URI, cells [][]term.Cell, content []byte, d []uint32,
	semanticTypes map[string]term.Attributes,
) (ret []editor.Location) {
	tc := span.NewContentConverter(uri.Filename(), content)
	colmap := protocol.ColumnMapper{
		URI:       uri,
		Content:   content,
		Converter: tc,
	}

	lspLine := make([]uint32, len(d)/5)
	lspChar := make([]uint32, len(d)/5)
	var line, char uint32
	for i := 0; 5*i < len(d); i++ {
		lspLine[i] = line + d[5*i+0]
		if d[5*i+0] > 0 {
			char = 0
		}
		lspChar[i] = char + d[5*i+1]
		char = lspChar[i]
		line = lspLine[i]
	}

	// second, convert to gopls coordinates
	for i := 0; 5*i < len(d); i++ {
		pr := protocol.Range{
			Start: protocol.Position{
				Line:      uint32(lspLine[i]),
				Character: uint32(lspChar[i]),
			},
			End: protocol.Position{
				Line:      uint32(lspLine[i]),
				Character: uint32(lspChar[i] + d[5*i+2]),
			},
		}
		from, to, ok := convertRange(pr, cells, colmap)
		if !ok {
			continue
		}

		// mods:   lsp.SemMods(int(d[5*i+4])),
		semType := lsp.SemType(int(d[5*i+3]))
		attr, ok := semanticTypes[semType]
		if !ok {
			log.Warnf("could not map semantic type %s; skipping token", semType)
			continue
		}

		loc := editor.Location{From: from, To: to, Attr: attr}
		ret = append(ret, loc)
	}
	return ret
}

func (h *lspEditorHandler) newSemanticTokensCtx() (ctx context.Context) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.cancelTokensReq != nil {
		h.cancelTokensReq()
		h.cancelTokensReq = nil
	}
	ctx, h.cancelTokensReq = context.WithCancel(context.Background())
	return ctx
}

// https://microsoft.github.io/language-server-protocol/specifications/specification-current/#textDocument_semanticTokens
func (h *lspEditorHandler) semanticTokensFull(
	ctx context.Context, srv execServer,
	f *file, cells [][]term.Cell, content string,
) {
	// NOTE: gopls does not pass semanticTokensProvider
	if srv.caps.SemanticTokensProvider == nil && srv.langID != ".go" {
		log.Debugf("lspEditorHandler.Server.SemanticTokensFull(%s): server does not support semantic tokens", f.name)
		return
	}
	version := h.getVersion(f)
	p2 := protocol.SemanticTokensParams{TextDocument: f.docID}
	resp, err := srv.srv.SemanticTokensFull(ctx, &p2)
	if err != nil {
		log.Errorf("lspEditorHandler.Server.SemanticTokensFull(%s): %v", f.name, err)
		return
	}
	if h.getVersion(f) != version {
		log.Debugf("lspEditorHandler.Server.SemanticTokensFull(%s): stale result", f.name)
		return
	}
	log.Tracef("lspEditorHandler.Server.SemanticTokensFull(%s): OK", f.name)

	locations := parseLocationData(f.uri, cells, []byte(content), resp.Data, h.semanticTypesAttr)
	err = h.ed.SetLocationList(f.handler, h.semanticTokensListID, editor.LocationSlice(locations))
	if err != nil {
		log.Errorf("lspEditorHandler.SetLocationList(%s): %v", f.name, err)
		return
	}
}

func makeInsertProtocolRange(
	newCells [][]term.Cell, from, to term.Coordinates,
) protocol.Range {
	starty, startx, ok := cell.ConvertTermCoordinates(newCells, from)
	if !ok {
		panic("coordinates out of sync")
	}
	return protocol.Range{
		Start: protocol.Position{
			Line:      uint32(starty),
			Character: uint32(startx),
		},
		End: protocol.Position{
			Line:      uint32(starty),
			Character: uint32(startx),
		},
	}
}

func makeDeleteProtocolRange(
	oldCells [][]term.Cell, from, to term.Coordinates,
) protocol.Range {
	from, to = cell.SortFromTo(from, to)
	starty, startx, ok := cell.ConvertTermCoordinates(oldCells, from)
	if !ok {
		panic("coordinates out of sync")
	}
	endy, endx, ok := cell.ConvertTermCoordinates(oldCells, to)
	if !ok {
		panic("coordinates out of sync")
	}

	// range end signals delete newline by setting it to x:0 y:next line
	// whereas in term.Coordinates, To signals the same by setting x==len
	if to.X == len(oldCells[to.Y]) {
		endy++
		endx = 0
	} else {
		// end is right exclusive
		endx++
	}

	return protocol.Range{
		Start: protocol.Position{
			Line:      uint32(starty),
			Character: uint32(startx),
		},
		End: protocol.Position{
			Line:      uint32(endy),
			Character: uint32(endx),
		},
	}
}

// https://microsoft.github.io/language-server-protocol/specifications/specification-current/#range
func makeProtocolRange(
	content string, newCells, oldCells [][]term.Cell,
	from, to term.Coordinates,
) protocol.Range {
	// the problem we face is that for delete, from, to are the coordinates that refer
	// to the original cells, but for insert, they refer to the new cells, thus
	// translation needs to be performed with a different set of cells
	if content == "" {
		return makeDeleteProtocolRange(oldCells, from, to)
	}
	return makeInsertProtocolRange(newCells, from, to)
}

func (h *lspEditorHandler) callServerDidChange(
	ctx context.Context, srv execServer,
	f *file, evts []protocol.TextDocumentContentChangeEvent,
) error {
	params := protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			Version:                h.getVersion(f),
			TextDocumentIdentifier: f.docID,
		},
		ContentChanges: evts,
	}
	err := srv.srv.DidChange(ctx, &params)
	if err != nil {
		log.Errorf("lspEditorHandler.Server.DidChange(%s): %v", f.name, err)
	}
	return err
}

func (h *lspEditorHandler) getVersion(f *file) int32 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return f._version
}

func (h *lspEditorHandler) incrementVersion(f *file) int32 {
	h.mu.Lock()
	defer h.mu.Unlock()
	f._version++
	return f._version
}

// TODO client is expected to support both incremental and full synchronization
// based on h.protocol server capabilities. Right now we are assuming incremental.
func (h *lspEditorHandler) pushFullUpdate(
	ctx context.Context, srv execServer, f *file, content string,
) {
	version := h.incrementVersion(f)

	evts := []protocol.TextDocumentContentChangeEvent{{Text: content}}
	err := h.callServerDidChange(ctx, srv, f, evts)
	if err == nil {
		log.Tracef("sent full file update: file=%v, length=%v, version=%v",
			f.name, len(content), version)
	}
}

func (h *lspEditorHandler) sendIncrementalUpdate(
	ctx context.Context, srv execServer, f *file, newCells,
	oldCells [][]term.Cell, content string, from, to term.Coordinates,
) (protocol.Range, error) {
	version := h.incrementVersion(f)

	// https://microsoft.github.io/language-server-protocol/specification#textDocument_didChange
	rng := makeProtocolRange(content, newCells, oldCells, from, to)
	evts := []protocol.TextDocumentContentChangeEvent{{Text: content, Range: &rng}}

	log.Tracef("sending incremental file update: file=%v, length=%v, version=%v,"+
		" rangeStart: %#v, rangeEnd: %#v, from=%#v, to=%#v: content='%s'",
		f.name, len(content), version, rng.Start, rng.End, from, to, content)

	err := h.callServerDidChange(ctx, srv, f, evts)
	if err != nil {
		log.Errorf("lspEditorHandler.Server.DidChange: %v", err)
		return rng, err
	}

	return rng, nil
}

func (h *lspEditorHandler) setCells(f *file, cells [][]term.Cell) {
	h.mu.Lock()
	defer h.mu.Unlock()

	f._cells = cells
}

func (h *lspEditorHandler) getCells(f *file) (cells [][]term.Cell) {
	h.mu.Lock()
	defer h.mu.Unlock()

	return f._cells
}

func (h *lspEditorHandler) handleFileFlush(ev editor.Event) {
	ctx := context.Background()
	ctx, cancelFn := context.WithTimeout(ctx, h.rpcTimeout)
	defer cancelFn()

	f, ok := h.getFileWithName(ev.ResourceName)
	if !ok {
		f = h.newFile(ev.Resource, ev.ResourceName, ev.Content)
	}

	srv, ok := h.getServer(f.languageID)
	if !ok {
		return
	}

	h.pushFullUpdate(ctx, srv, f, ev.Content)
	h.setCells(f, cell.StringToCells(ev.Content))
	ctx = h.newSemanticTokensCtx()
	go h.semanticTokensFull(ctx, srv, f, h.getCells(f), ev.Content)
}

func (h *lspEditorHandler) handleFileInsert(ev editor.Event) {
	ctx := context.Background()
	ctx, cancelFn := context.WithTimeout(ctx, h.rpcTimeout)
	defer cancelFn()
	f, ok := h.getFileWithName(ev.ResourceName)
	if !ok {
		log.Warnf("lspEditorHandler: Received insert event for an unknown file: %#v", ev)
		return
	}

	srv, ok := h.getServer(f.languageID)
	if !ok {
		return
	}

	oldCells := h.getCells(f)
	buf := cell.CellsToBuffer(oldCells)
	buf.InsertString(ev.Start, ev.Content)
	newCells := buf.RawCells()
	_, err := h.sendIncrementalUpdate(ctx, srv, f, newCells, oldCells, ev.Content, ev.From, ev.To)
	h.setCells(f, newCells)
	if err != nil {
		return
	}

	ctx = h.newSemanticTokensCtx()
	go h.semanticTokensFull(ctx, srv, f, newCells, buf.String())
}

func (h *lspEditorHandler) handleFileDelete(ev editor.Event) {
	ctx := context.Background()
	ctx, cancelFn := context.WithTimeout(ctx, h.rpcTimeout)
	defer cancelFn()
	f, ok := h.getFileWithName(ev.ResourceName)
	if !ok {
		log.Warnf("lspEditorHandler: Received delete event for an unknown file: %#v", ev)
		return
	}

	srv, ok := h.getServer(f.languageID)
	if !ok {
		return
	}

	oldCells := h.getCells(f)
	buf := cell.CellsToBuffer(oldCells)
	buf.Delete(ev.From, ev.To)
	newCells := buf.RawCells()
	_, err := h.sendIncrementalUpdate(ctx, srv, f, newCells, oldCells, "", ev.From, ev.To)
	h.setCells(f, newCells)
	if err != nil {
		return
	}

	ctx = h.newSemanticTokensCtx()
	go h.semanticTokensFull(ctx, srv, f, newCells, buf.String())
}

func (h *lspEditorHandler) handleFileOpen(ev editor.Event) {
	ctx := context.Background()
	ctx, cancelFn := context.WithTimeout(ctx, h.rpcTimeout)
	defer cancelFn()

	f := h.newFile(ev.Resource, ev.ResourceName, ev.Content)
	srv, ok := h.getServer(f.languageID)
	if !ok {
		log.Warnf("could not connect to lsp server for %s: "+
			"configuration not found or process not running", f.languageID)
		return
	}

	p := protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        protocol.URIFromSpanURI(f.uri),
			LanguageID: f.languageID,
			Version:    h.getVersion(f),
			Text:       ev.Content,
		},
	}

	if err := srv.srv.DidOpen(ctx, &p); err != nil {
		log.Errorf("lspEditorHandler.Server.DidOpen(%s, %s): %v", f.name, f.languageID, err)
		return
	}
	log.Tracef("lspEditorHandler.Server.DidOpen(%s, %s)", f.name, f.languageID)

	h.dispatchPendingDiagnostics(ctx, f.uri)
	h.dispatchPendingGoTo(ctx, srv, f)
	ctx = h.newSemanticTokensCtx()
	go h.semanticTokensFull(ctx, srv, f, h.getCells(f), ev.Content)
}

func (h *lspEditorHandler) removeFile(name string) (*file, bool) {
	f, ok := h.getFileWithName(name)
	if !ok {
		return nil, false
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.files, f.uri)

	return f, true
}

func (h *lspEditorHandler) sendDidClose(
	ctx context.Context, srv execServer, uri span.URI,
) {
	p := protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: protocol.URIFromSpanURI(uri),
		},
	}

	if err := srv.srv.DidClose(ctx, &p); err != nil {
		log.Errorf("lspEditorHandler.Server.DidClose(%s): %v", uri, err)
		return
	}
	log.Tracef("lspEditorHandler.Server.DidClose(%s)", uri)
}

func (h *lspEditorHandler) handleFileClose(ev editor.Event) {
	ctx := context.Background()
	ctx, cancelFn := context.WithTimeout(ctx, h.rpcTimeout)
	defer cancelFn()
	f, ok := h.removeFile(ev.ResourceName)
	if !ok {
		log.Warnf("lspEditorHandler: Received close event for an unknown file: %#v", ev)
		return
	}
	srv, ok := h.getServer(f.languageID)
	if !ok {
		return
	}

	h.sendDidClose(ctx, srv, f.uri)
	h.addPendingDiagnostics(f.uri, h.getDiagnostics(f))
}

func (h *lspEditorHandler) parseDiagnostics(
	f *file, d []protocol.Diagnostic,
) []editor.Location {
	cells := h.getCells(f)
	colmap := h.getColumnMapper(f)

	locs := make([]editor.Location, 0, len(d))
	for _, d := range d {
		from, to, ok := convertRange(d.Range, cells, colmap)
		if !ok {
			continue
		}

		msg := fmt.Sprintf("%s: %s", d.Source, d.Message)
		attr, ok := h.diagnosticAttr[d.Severity]
		if !ok {
			log.Warnf("unknown diagnostic severity %v; skipping diagnostic", d.Severity)
			continue
		}

		loc := editor.Location{Message: msg, From: from, To: to, Attr: attr}
		locs = append(locs, loc)
	}

	return locs
}

func (h *lspEditorHandler) setDiagnostics(f *file, ds []protocol.Diagnostic) {
	h.mu.Lock()
	defer h.mu.Unlock()

	f._diagnostics = ds
}

func (h *lspEditorHandler) getDiagnostics(f *file) (ds []protocol.Diagnostic) {
	h.mu.Lock()
	defer h.mu.Unlock()

	return f._diagnostics
}

func (h *lspEditorHandler) handleDiagnostics(
	ctx context.Context, uri span.URI,
	ds []protocol.Diagnostic, version int32,
) {
	f, ok := h.getFile(uri)
	if !ok {
		h.addPendingDiagnostics(uri, ds)
		log.Warnf("lspEditorHandler: Received diagnostic for an unknown file: %#v", uri)
		return
	}

	h.setDiagnostics(f, ds)

	if version != h.getVersion(f) {
		log.Debugf("lspEditorHandler: Received diagnostic for "+
			"outdated version of file '%s': %#v", f.name, version)
		// it seems that tsserver does not send any version info
		// return
	}

	locs := h.parseDiagnostics(f, ds)
	err := h.ed.SetLocationList(f.handler, h.diagnosticListID, editor.LocationSlice(locs))
	if err != nil {
		log.Errorf("lspEditorHandler.SetLocationList(%s): %v", f.name, err)
		return
	}
}

func (h *lspEditorHandler) HandleDiagnostics(
	ctx context.Context, p *protocol.PublishDiagnosticsParams,

) {
	var start time.Time
	if log.IsLevelEnabled(log.TraceLevel) {
		start = time.Now()
		log.Tracef("lspEditorHandler.HandleDiagnostics(%#v)", p.URI)
	}

	h.handleDiagnostics(ctx, p.URI.SpanURI(), p.Diagnostics, p.Version)

	if log.IsLevelEnabled(log.TraceLevel) {
		log.Tracef("lspEditorHandler.HandleDiagnostics(%#v) in %s", p.URI, time.Since(start))
	}
}

func (h *lspEditorHandler) goToLocation(win browser.Window, l protocol.Location) {
	uri := l.URI.SpanURI()
	filename := uri.Filename()

	f, alreadyOpen := h.getFileWithName(filename)
	if !alreadyOpen {
		h.addPendingGoTo(uri, l.Range)
	}

	buf, err := h.o.Open(filename)
	if err != nil {
		h.m.SetMessage("Open: %v", err)
		log.Errorf("lspEditorHandler.Open(%s): %v", filename, err)
		h.removePendingGoTo(uri)
		return
	}

	if alreadyOpen {
		h.handleGoTo(f, l.Range)
	}

	err = win.SetContent(buf)
	if err != nil && err != browser.ErrTabNotFree {
		log.Errorf("error SetContent: %v", err)
		return
	}
}

func (h *lspEditorHandler) getFilePosition(cursor term.Coordinates, filename string) (
	f *file, pos protocol.Position, ok bool,
) {
	uri := span.URIFromPath(filename)

	f, ok = h.getFile(uri)
	if !ok {
		log.Warnf("lspEditorHandler: Received hover request for an unknown file: %#v", uri)
		return
	}

	line, column, ok := cell.ConvertTermCoordinates(h.getCells(f), cursor)
	if !ok {
		log.Warnf("lspEditorHandler: Received hover request for an oob position: %#v", cursor)
		return
	}

	ok = true
	pos = protocol.Position{
		Line:      uint32(line),
		Character: uint32(column),
	}

	return
}

func (h *lspEditorHandler) handleGoToDefinition(cursor term.Coordinates, ed editor.Handler, filename string) {
	f, pos, ok := h.getFilePosition(cursor, filename)
	if !ok {
		return
	}

	p := protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: protocol.URIFromSpanURI(f.uri),
			},
			Position: pos,
		},
	}

	srv, ok := h.getServer(f.languageID)
	if !ok {
		return
	}

	ctx := context.Background()
	ctx, cancelFn := context.WithTimeout(ctx, h.rpcTimeout)
	defer cancelFn()

	locs, err := srv.srv.Definition(ctx, &p)
	if err != nil {
		log.Errorf("lspEditorHandler.Server.Definition(%s, %s): %v", f.name, f.languageID, err)
		return
	}

	log.Tracef("lspEditorHandler.Server.Definition(%s, %s): %#v", f.name, f.languageID, locs)

	if len(locs) == 0 {
		return
	}

	win, err := h.wm.Focus()
	if err != nil {
		log.Errorf("lspEditorHandler.Focus(): %v", err)
		return
	}

	for _, l := range locs {
		h.goToLocation(win, l)
	}
}

func findBestFloatingWindowPosition(cursorAtWindow term.Coordinates, buf *cell.Buffer) (
	term.Coordinates, int, int,
) {
	maxColumns := buf.MaxColumns()
	width := int(math.Min(float64(maxColumns)+3, maxHoverColumns))
	height := buf.Rows() + 3 // frame + less bar

	at := cursorAtWindow
	at.Y -= height - 1

	if at.Y < 0 {
		at.Y = cursorAtWindow.Y + 2
	}

	return at, width, height
}

func (h *lspEditorHandler) handleHover(
	cursorAtScroll, cursorAtWindow term.Coordinates,
	ed editor.Handler, filename string,
) {
	f, pos, ok := h.getFilePosition(cursorAtScroll, filename)
	if !ok {
		return
	}

	p := protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: protocol.URIFromSpanURI(f.uri),
			},
			Position: pos,
		},
	}

	srv, ok := h.getServer(f.languageID)
	if !ok {
		return
	}

	ctx := context.Background()
	ctx, cancelFn := context.WithTimeout(ctx, h.rpcTimeout)
	defer cancelFn()

	hover, err := srv.srv.Hover(ctx, &p)
	if err != nil || hover == nil {
		log.Errorf("lspEditorHandler.Server.Hover(%s, %s): %v", f.name, f.languageID, err)
		return
	}

	log.Tracef("lspEditorHandler.Server.Hover(%s, %s): %#v", f.name, f.languageID, hover)

	less := handler.NewLess()
	less.Scroll.Buffer().WriteString(hover.Contents.Value)
	bh := browser.NopHandler(less)

	at, width, height := findBestFloatingWindowPosition(cursorAtWindow, less.Scroll.Buffer())
	_, err = h.wm.FloatingWindow(bh, at, width, height)
	if err != nil {
		log.Errorf("lspEditorHandler.SplitHorizontalAbove(%s): %v", f.name, err)
	}
}

func makeWorkspaceFolder(in string) protocol.WorkspaceFolder {
	return protocol.WorkspaceFolder{
		URI:  string(protocol.URIFromPath(in)),
		Name: in,
	}
}

func (h *lspEditorHandler) handleChangedWorkspace(
	name string, added []string, removed []string,
) {
	uri := span.URIFromPath(name)
	languageID := filepath.Ext(uri.Filename())

	srv, ok := h.getServer(languageID)
	if !ok {
		return
	}

	/*cfg := caps.InnerServerCapabilities.Workspace.WorkspaceFolders
	if !cfg.Supported {
		h.m.SetMessage("LSP server does not support changing workspaces")
		log.Tracef("lspEditorHandler.Server.DidChangeWorkspaceFolders(%#v): not supported", caps)
		return
	}*/

	ctx := context.Background()
	ctx, cancelFn := context.WithTimeout(ctx, h.rpcTimeout)
	defer cancelFn()

	var remove []protocol.WorkspaceFolder
	for _, folder := range removed {
		remove = append(remove, makeWorkspaceFolder(folder))
	}

	var add []protocol.WorkspaceFolder
	for _, folder := range added {
		add = append(add, makeWorkspaceFolder(folder))
	}

	req := protocol.DidChangeWorkspaceFoldersParams{
		Event: protocol.WorkspaceFoldersChangeEvent{Added: add, Removed: remove},
	}

	log.Tracef("lspEditorHandler.Server.DidChangeWorkspaceFolders(%#v)", req)

	err := srv.srv.DidChangeWorkspaceFolders(ctx, &req)
	if err != nil {
		h.m.SetMessage("DidChangeWorkspaceFolders: %v", err)
		log.Errorf("lspEditorHandler.Server.DidChangeWorkspaceFolders(%s): %v", name, err)
	}
}

func (h *lspEditorHandler) handleAddWorkspace(name string, args []string) {
	log.Debugf("lspEditorHandler.handleAddWorkspace(%v)", args)
	h.handleChangedWorkspace(name, args, nil)
}

func (h *lspEditorHandler) handleRemoveWorkspace(name string, args []string) {
	log.Debugf("lspEditorHandler.handleRemoveWorkspace(%v)", args)
	h.handleChangedWorkspace(name, nil, args)
}

func (h *lspEditorHandler) HandleCommand(cmd editor.Command) (exit bool) {
	switch cmd.Name {
	case commandNextDiagnostic:
		err := h.ed.MoveToNextLocation(cmd.Resource, h.diagnosticListID)
		if err != nil {
			log.Errorf("lspEditorHandler.MoveToNextLocation(%s): %v", cmd.Name, err)
		}
	case commandPrevDiagnostic:
		err := h.ed.MoveToPrevLocation(cmd.Resource, h.diagnosticListID)
		if err != nil {
			log.Errorf("lspEditorHandler.MoveToNextLocation(%s): %v", cmd.Name, err)
		}
	case commandHover:
		h.handleHover(cmd.Cursor.Content, cmd.Cursor.Window, cmd.Resource, cmd.ResourceName)
	case commandGoToDef:
		h.handleGoToDefinition(cmd.Cursor.Content, cmd.Resource, cmd.ResourceName)
	case commandAddWorkspace:
		h.handleAddWorkspace(cmd.ResourceName, cmd.Args)
	case commandRemoveWorkspace:
		h.handleRemoveWorkspace(cmd.ResourceName, cmd.Args)
	}

	return false
}

func (h *lspEditorHandler) handleEvents(ch chan editor.Event) {
	for ev := range ch {
		var start time.Time
		if log.IsLevelEnabled(log.TraceLevel) {
			start = time.Now()
			log.Tracef("lspEditorHandler.Handle(%#v)", ev)
		}

		switch ev.Type {
		case editor.EventTypeOpen:
			h.handleFileOpen(ev)
		case editor.EventTypeClose:
			h.handleFileClose(ev)
		case editor.EventTypeFlush:
			h.handleFileFlush(ev)
		case editor.EventTypeInsert:
			h.handleFileInsert(ev)
		case editor.EventTypeDelete:
			h.handleFileDelete(ev)
		}

		if log.IsLevelEnabled(log.TraceLevel) {
			log.Tracef("lspEditorHandler.Handle(%#v) in %s", ev, time.Since(start))
		}
	}
}

func (h *lspEditorHandler) Handle(ev editor.Event) (exit bool) {
	h.mu.Lock()
	exit = h.exit
	h.mu.Unlock()

	if exit {
		return
	}

	h.evChan <- ev
	return
}

func (h *lspEditorHandler) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	defer close(h.evChan)
	h.exit = true

	var errs []string
	for _, server := range h.servers {
		if server.cmd.Process == nil {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), h.disconnectTimeout)

		h.mu.Unlock()
		err := server.srv.Shutdown(ctx)
		h.mu.Lock()

		cancel()
		if err != nil {
			errs = append(errs, err.Error())
		}
		err = server.cmd.Process.Kill()
		if err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) != 0 {
		return errors.New(strings.Join(errs, "; "))
	}

	return nil
}
