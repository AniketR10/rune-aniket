package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	multierr "github.com/ernestrc/go-multierror"
	"github.com/ernestrc/golang-internal-tools/fakenet"
	"github.com/ernestrc/golang-internal-tools/jsonrpc2"
	"github.com/ernestrc/golang-internal-tools/lsp"
	"github.com/ernestrc/golang-internal-tools/lsp/lsprpc"
	"github.com/ernestrc/golang-internal-tools/lsp/protocol"
	"github.com/ernestrc/golang-internal-tools/lsp/source"
	"github.com/ernestrc/golang-internal-tools/span"
	log "github.com/sirupsen/logrus"
	browserapi "unstable.build/go-tui/api/browser"
	browserplugin "unstable.build/go-tui/api/browser/plugin"
	textapi "unstable.build/go-tui/api/text"
	workspaceapi "unstable.build/go-tui/api/workspace"
	workspaceplugin "unstable.build/go-tui/api/workspace/plugin"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/config"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/handler/search"
	"unstable.build/go-tui/plugin"
	plugutil "unstable.build/go-tui/plugin/util"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text/vi"
)

const (
	maxHoverColumns             = 90
	defaultRpcTimeout           = 10 * time.Second
	defaultConnectTimeout       = 10 * time.Second
	defaultDisconnectTimeout    = 300 * time.Millisecond
	firstFileVersion            = 1
	commandNextDiagnostic       = "lspNextDiagnostic"
	commandPrevDiagnostic       = "lspPrevDiagnostic"
	commandHover                = "lspHover"
	commandGoToDef              = "lspGoToDefinition"
	commandFormat               = "lspFormat"
	commandOrganizeImports      = "lspOrganizeImports"
	commandReferences           = "lspReferences"
	commandAddWorkspace         = "lspAddWorkspaceFolder"
	commandRemoveWorkspace      = "lspRemoveWorkspaceFolder"
	handleBackpressureEvs       = 64
	referencesWindowWidth       = 50
	referencesWindowHeight      = 15
	defaultSemanticTokensListID = "lsp_syntax_highlighting"
	defaultDiagnosticListID     = "lsp_diagnostic"
)

var (
	lspHandlerCommands = []string{
		commandNextDiagnostic, commandPrevDiagnostic,
		commandHover, commandGoToDef, commandFormat, commandReferences,
		commandAddWorkspace, commandRemoveWorkspace, commandOrganizeImports,
	}
	lspHandlerEvents = []textapi.EventType{
		textapi.EventTypeClose,
		textapi.EventTypeFlush,
		textapi.EventTypeOpen,
		textapi.EventTypeEdit,
	}
	lspHandlerPermissions = []plugin.Permission{
		plugin.Permission(plugin.PermissionBrowserWindowManager),
		plugin.Permission(plugin.PermissionBrowserResourceOpener),
		plugin.Permission(plugin.PermissionBrowserEventPublisher),
		plugin.Permission(plugin.PermissionBrowserMessenger),
		plugin.Permission(plugin.PermissionFileSystem),
		plugin.Permission(plugin.PermissionExecute),
		plugin.PermissionConfig,
	}
	defaultDiagnosticAttr = map[protocol.DiagnosticSeverity]term.Attributes{
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
		"method":        {},
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
	errNoServer                 = errors.New("no LSP server found for language file language")
	defaultReferencesWindowAttr = term.Attributes{Bg: term.ColorBlack}
)

type file struct {
	uri        workspaceapi.URI
	languageID string
	handler    textapi.Handler
	docID      protocol.TextDocumentIdentifier

	// handler use getters
	_version     int32
	_cells       [][]term.Cell
	_diagnostics []protocol.Diagnostic
}

type execServer struct {
	langID  string
	pid     workspaceapi.Pid
	srv     protocol.Server
	caps    protocol.ServerCapabilities
	closers []io.Closer
}

type lspEditorHandler struct {
	mu     sync.Mutex
	evChan chan textapi.Event

	ed   textapi.Editor
	wm   browserapi.WindowManager
	m    browserapi.Messenger
	o    browserapi.ResourceOpener
	exec workspaceapi.Executor
	fs   workspaceapi.FileSystem
	p    browserapi.EventPublisher

	tabspaces                  int
	frame                      bool
	semanticTypesAttr          map[string]term.Attributes
	diagnosticAttr             map[protocol.DiagnosticSeverity]term.Attributes
	enableSemanticTokens       map[string]bool
	semanticTokensListID       string
	diagnosticListID           string
	rpcTimeout                 time.Duration
	connectTimeout             time.Duration
	disconnectTimeout          time.Duration
	referencesWindowAttributes term.Attributes

	cwd               string
	exit              bool
	files             map[string]*file
	pendingDiagnostic map[string][]protocol.Diagnostic
	pendingGoTo       map[string]protocol.Range
	serversCfg        map[string]interface{}
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
		// TODO validate compat
		// params.Path = params.RootPath // backwards compat with tsserver
	}

	params.Capabilities.Workspace.Configuration = false
	params.Capabilities.Workspace.Symbol = new(protocol.WorkspaceSymbolClientCapabilities)
	params.Capabilities.Workspace.Symbol.SymbolKind.ValueSet = []protocol.SymbolKind{}

	// Make sure to respect configured options when sending initialize request.
	opts := source.DefaultOptions().Clone()

	params.Capabilities.TextDocument.Hover = protocol.HoverClientCapabilities{
		ContentFormat: []protocol.MarkupKind{opts.PreferredContentFormat},
	}
	params.Capabilities.Workspace.ApplyEdit = true
	params.Capabilities.Workspace.WorkspaceEdit = new(protocol.WorkspaceEditClientCapabilities)
	params.Capabilities.Workspace.WorkspaceEdit.DocumentChanges = true
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

	h.mu.Lock()
	exit := h.exit
	h.mu.Unlock()
	if err != nil && !exit {
		log.Errorf("jsonrpc2 processing goroutine error: %v", err)
	}
}

func initializeConnection(
	h *lspEditorHandler, reader io.ReadCloser, writer io.WriteCloser,
) protocol.Server {
	conn := fakenet.NewConn("stdio", reader, writer)
	stream := jsonrpc2.NewHeaderStream(conn)
	cc := jsonrpc2.NewConn(stream)
	server := protocol.ServerDispatcher(cc)
	go streamRPC(cc, h)

	return server
}

func (h *lspEditorHandler) parseCmd(arg interface{}) (workspaceapi.Cmd, error) {
	str, ok := arg.(string)
	cmd := strings.Split(str, " ")
	if !ok || len(cmd) == 0 {
		return workspaceapi.Cmd{}, fmt.Errorf("invalid command: %v", arg)
	}

	return workspaceapi.Cmd{
		Path: cmd[0],
		Args: cmd[1:],
	}, nil
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

type nop struct {
	io.Writer
	io.Reader
}

func (w nop) Close() error {
	/* enable for debugging
	if log.IsLevelEnabled(log.TraceLevel) {
		buf := make([]byte, 1<<16)
		n := runtime.Stack(buf, true)
		var pipe string
		if w.Writer != nil {
			pipe = "stdin"
		} else {
			pipe = "stdout"
		}
		log.Warningf("Close called on lsp server's %s pipe: \n%s", pipe, buf[:n])
	}*/
	return nil
}

func (h *lspEditorHandler) setPipes(
	cmd *workspaceapi.Cmd, srv *execServer,
) (stdout, stderr io.ReadCloser, stdin io.WriteCloser, err error) {
	var stdoutWrite, stderrWrite io.WriteCloser
	var stdinRead io.ReadCloser
	stdout, stdoutWrite, err = os.Pipe()
	if err != nil {
		return
	}
	stderr, stderrWrite, err = os.Pipe()
	if err != nil {
		return
	}
	stdinRead, stdin, err = os.Pipe()
	if err != nil {
		return
	}
	cmd.Stdout = stdoutWrite
	cmd.Stderr = stderrWrite
	cmd.Stdin = stdinRead
	srv.closers = []io.Closer{
		stdoutWrite, stderrWrite, stdinRead, stdout, stderr, stdin,
	}
	return
}

func (h *lspEditorHandler) startLanguageServer(
	langID string, cmdAndArgs interface{},
) (execServer, error) {
	ctx := context.Background()
	ctx, cancelFn := context.WithTimeout(ctx, h.connectTimeout)
	defer cancelFn()

	cmd, err := h.parseCmd(cmdAndArgs)
	if err != nil {
		return execServer{}, err
	}

	srv := execServer{langID: langID}

	stdout, stderr, stdin, err := h.setPipes(&cmd, &srv)
	if err != nil {
		return execServer{}, err
	}

	log.Debugf("starting %q LSP server with cmd %#v", langID, cmd)
	pid, err := h.exec.Start(cmd)
	log.Tracef("started %q LSP server with pid=%d: err=%v", langID, pid, err)
	if err != nil {
		err = fmt.Errorf("workspace.Start: %v", err)
		return execServer{}, err
	}

	// helps debug
	if log.IsLevelEnabled(log.DebugLevel) {
		go logStderr(langID, stderr)
	}

	stdout = nop{Reader: stdout}
	stdin = nop{Writer: stdin}
	server := initializeConnection(h, stdout, stdin)

	srv.srv = server
	srv.pid = pid
	initRes, err := sendInitializeRequest(ctx, h.cwd, srv)
	if err != nil {
		return execServer{}, err
	}

	srv.caps = initRes.Capabilities

	log.Infof("connected to '%s' LSP server '%s' with version %s: ",
		langID, initRes.ServerInfo.Name, initRes.ServerInfo.Version)

	return srv, nil
}

func (h *lspEditorHandler) initLanguageServers(pconfig config.Config) error {
	cfg, err := pconfig.GetMap("exec")
	if err != nil {
		err = fmt.Errorf("Failed to get lsp servers 'exec' config: %v", err)
		return err
	}

	h.servers = make(map[string]execServer)
	h.serversCfg = cfg

	return nil
}

func getSemanticTypesAttr(pconfig config.Config) (map[string]term.Attributes, error) {
	ret := make(map[string]term.Attributes, len(defaultSemanticTypeAttr))
	for k, v := range defaultSemanticTypeAttr {
		ret[k] = v
	}

	colors, err := pconfig.GetConfig("syntax_highlighting")
	if err != nil {
		if err != config.ErrNotFound {
			err = fmt.Errorf("Error getting 'syntax_highlighting' from plugin config: %v", err)
			return nil, err
		}
		return ret, nil
	}

	for semanticType := range defaultSemanticTypeAttr {
		attr, err := config.GetAttributes(colors, semanticType)
		if err != nil {
			if err != config.ErrNotFound {
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

func getDiagnosticAttr(pconfig config.Config) (
	map[protocol.DiagnosticSeverity]term.Attributes, error,
) {
	ret := make(map[protocol.DiagnosticSeverity]term.Attributes, len(defaultDiagnosticAttr))
	for k, v := range defaultDiagnosticAttr {
		ret[k] = v
	}

	colors, err := pconfig.GetConfig("diagnostics")
	if err != nil {
		if err != config.ErrNotFound {
			err = fmt.Errorf("Error getting 'diagnostics' from plugin config: %v", err)
			return nil, err
		}
		return ret, nil
	}

	for s := range defaultDiagnosticAttr {
		name := severityToString(s)
		attr, err := config.GetAttributes(colors, name)
		if err != nil {
			if err != config.ErrNotFound {
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
		log.Errorf("lspEditorHandler.convertRange: failed to create rangespan for range: %#v->%#v: %v",
			rng.Start, rng.End, err)
		return
	}

	startLine := spn.Start().Line() - 1
	startChar := spn.Start().Column() - 1
	from, ok = cell.ConvertRuneCoordinates(cells, startLine, startChar)
	if !ok {
		log.Errorf("lspEditorHandler.convertRange: failed to convert lsp Start coordinates"+
			" to term From coordinates: rng: (y=%d,x=%d) -> spn: %#v -> from:%#v",
			rng.Start.Line, rng.Start.Character, spn.Start(), from)
		return
	}

	log.Tracef("lspEditorHandler.convertRange: convert lsp Start coordinates"+
		" to term From coordinates: rng: (y=%d,x=%d) -> spn: %#v -> from:%#v",
		rng.Start.Line, rng.Start.Character, spn.Start(), from)

	endLine := spn.End().Line() - 1
	endChar := spn.End().Column() - 1
	to, ok = cell.ConvertRuneCoordinates(cells, endLine, endChar)
	if !ok {
		log.Errorf("lspEditorHandler.convertRange: failed to convert lsp End coordinates to "+
			"term From coordinates: rng: (y=%d,x=%d) -> spn: %#v -> to:%#v",
			rng.End.Line, rng.End.Character, spn.End(), to)
		return
	}

	log.Tracef("lspEditorHandler.convertRange: convert lsp End coordinates to "+
		" to term To coordinates: rng: (y=%d,x=%d) -> spn: %#v -> to:%#v",
		rng.End.Line, rng.End.Character, spn.End(), to)

	return
}

func newLspHandler(
	ed textapi.Editor, grants []plugin.Grant,
	broker proto.MuxBroker, pconfig config.Config,
) (plugutil.CommandEventHandler, error) {
	ret := new(lspEditorHandler)
	ret.ed = ed
	ret.files = make(map[string]*file)
	ret.pendingDiagnostic = make(map[string][]protocol.Diagnostic)
	ret.pendingGoTo = make(map[string]protocol.Range)
	ret.evChan = make(chan textapi.Event, handleBackpressureEvs)

	var err error
	ret.semanticTypesAttr, err = getSemanticTypesAttr(pconfig)
	if err != nil {
		return nil, err
	}

	ret.diagnosticAttr, err = getDiagnosticAttr(pconfig)
	if err != nil {
		return nil, err
	}

	var configErr error
	ret.semanticTokensListID, err = pconfig.GetString("semantic_tokens_list_id")
	if err != nil {
		if err != config.ErrNotFound {
			configErr = multierr.Append(configErr,
				fmt.Errorf("failed to get 'semantic_tokens_list_id' from config: %v", err))
		}
		ret.semanticTokensListID = defaultSemanticTokensListID
	}

	// default is disabled for all
	ret.enableSemanticTokens = make(map[string]bool)

	var enableSemanticTokensIfc map[string]interface{}
	enableSemanticTokensIfc, err = pconfig.GetMap("enable_semantic_tokens")
	if err != nil {
		if err != config.ErrNotFound {
			configErr = multierr.Append(configErr,
				fmt.Errorf("failed to get 'enable_semantic_tokens' from config: %v", err))
		}
	} else {
		for k, v := range enableSemanticTokensIfc {
			b, ok := v.(bool)
			if !ok {
				configErr = multierr.Append(configErr,
					fmt.Errorf("failed to get 'enable_semantic_tokens' from config: "+
						"expected map of string to bool, found %q to be %v", k, v))
				continue
			}
			ret.enableSemanticTokens[k] = b
		}
	}

	ret.diagnosticListID, err = pconfig.GetString("diagnostic_list_id")
	if err != nil {
		if err != config.ErrNotFound {
			configErr = multierr.Append(configErr,
				fmt.Errorf("failed to get 'diagnostic_list_id' from config: %v", err))
		}
		ret.diagnosticListID = defaultDiagnosticListID
	}

	ret.connectTimeout, err = config.GetDuration(pconfig,
		"connect_timeout", defaultConnectTimeout)
	if err != nil {
		if err != config.ErrNotFound {
			configErr = multierr.Append(configErr, err)
		}
	}

	ret.disconnectTimeout, err = config.GetDuration(pconfig,
		"disconnect_timeout", defaultDisconnectTimeout)
	if err != nil {
		if err != config.ErrNotFound {
			configErr = multierr.Append(configErr, err)
		}
	}
	ret.referencesWindowAttributes, err = config.GetAttributes(pconfig, "references_window_attr")
	if err != nil {
		if err != config.ErrNotFound {
			configErr = multierr.Append(configErr, err)
		}
		ret.referencesWindowAttributes = defaultReferencesWindowAttr
	}
	ret.rpcTimeout, err = config.GetDuration(pconfig,
		"rpc_timeout", defaultRpcTimeout)
	if err != nil {
		if err != config.ErrNotFound {
			configErr = multierr.Append(configErr, err)
		}
	}
	if configErr != nil {
		log.Warnf("Errors loading config: %v", configErr)
	}

	err = ret.initLanguageServers(pconfig)
	if err != nil {
		return nil, err
	}

	for _, g := range grants {
		switch g.Permission {
		case plugin.Permission(plugin.PermissionFileSystem):
			ret.fs, err = workspaceplugin.FileSystem(g, broker)
			if err != nil {
				return nil, err
			}
			cwdURI, err := ret.fs.URI(".")
			if err != nil {
				return nil, err
			}
			ret.cwd = cwdURI.Path()
		case plugin.Permission(plugin.PermissionExecute):
			ret.exec, err = workspaceplugin.Executor(g, broker)
			if err != nil {
				return nil, err
			}
		case plugin.Permission(plugin.PermissionBrowserEventPublisher):
			ret.p, err = browserplugin.EventPublisher(g, broker)
			if err != nil {
				return nil, err
			}
		case plugin.Permission(plugin.PermissionBrowserResourceOpener):
			ret.o, err = browserplugin.ResourceOpener(g, broker)
			if err != nil {
				return nil, err
			}
		case plugin.Permission(plugin.PermissionBrowserWindowManager):
			ret.wm, err = browserplugin.WindowManager(g, broker)
			if err != nil {
				return nil, err
			}
		case plugin.Permission(plugin.PermissionBrowserMessenger):
			ret.m, err = browserplugin.Messenger(g, broker)
			if err != nil {
				return nil, err
			}
		case plugin.PermissionConfig:
			config, err := plugin.FetchConfig(g, broker)
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

	log.Infof("Initialized LSP handler with cwd %q", ret.cwd)

	go ret.handleEvents(ret.evChan)

	return ret, nil
}

func (h *lspEditorHandler) getServer(languageID string) (
	execServer, bool,
) {
	h.mu.Lock()
	proc, ok := h.servers[languageID]
	h.mu.Unlock()
	if ok {
		return proc, true
	}

	h.mu.Lock()
	cmd, ok := h.serversCfg[languageID]
	h.mu.Unlock()
	if !ok {
		// language server not configured for language
		return execServer{}, false
	}

	srv, err := h.startLanguageServer(languageID, cmd)
	if err != nil {
		log.Errorf("failed to start language server for %q: %s", languageID, err)
		return execServer{}, false
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	h.servers[languageID] = srv

	return srv, true
}

func (h *lspEditorHandler) newFile(
	handler textapi.Handler, uri workspaceapi.URI, content string,
) *file {
	h.mu.Lock()
	defer h.mu.Unlock()

	spanURI := workspaceURIToSpan(uri)
	languageID := filepath.Ext(uri.Path())

	f := &file{
		_version: firstFileVersion,
		handler:  handler,
		docID: protocol.TextDocumentIdentifier{
			URI: protocol.URIFromSpanURI(spanURI),
		},
		uri:        uri,
		_cells:     cell.StringToCells(content, h.tabspaces),
		languageID: languageID,
	}

	h.files[uri.String()] = f
	return f
}

func (h *lspEditorHandler) removePendingGoTo(uri workspaceapi.URI) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.pendingGoTo, uri.String())
}

func (h *lspEditorHandler) addPendingGoTo(
	uri workspaceapi.URI, rs protocol.Range,
) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.pendingGoTo[uri.String()] = rs
}

func (h *lspEditorHandler) addPendingDiagnostics(
	uri workspaceapi.URI, ds []protocol.Diagnostic,
) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.pendingDiagnostic[uri.String()] = ds
}

func (h *lspEditorHandler) dispatchPendingDiagnostics(
	ctx context.Context, uri workspaceapi.URI,
) error {
	h.mu.Lock()
	ds, ok := h.pendingDiagnostic[uri.String()]
	delete(h.pendingDiagnostic, uri.String())
	h.mu.Unlock()
	if !ok {
		return nil
	}

	return h.handleDiagnostics(ctx, uri, ds, firstFileVersion)
}

func getColumnMapper(uri span.URI, buf *cell.Buffer) protocol.ColumnMapper {
	content := []byte(buf.String())
	tc := span.NewContentConverter(uri.Filename(), content)
	return protocol.ColumnMapper{
		URI:       uri,
		Content:   content,
		Converter: tc,
	}
}

func (h *lspEditorHandler) handleGoTo(f *file, rs protocol.Range) error {
	cells := h.getCells(f)
	buf := cell.CellsToBuffer(cells, h.tabspaces)
	spanURI := workspaceURIToSpan(f.uri)
	colmap := getColumnMapper(spanURI, buf)
	pos, _, ok := convertRange(rs, cells, colmap)
	if !ok {
		return fmt.Errorf("could not convert lsp range to coordinates")
	}

	err := h.ed.SetCursor(f.handler, pos)
	if err != nil {
		err = fmt.Errorf("ed.SetCursor(%s): %v", f.uri, err)
		return err
	}
	return nil
}

func (h *lspEditorHandler) dispatchPendingGoTo(
	ctx context.Context, srv execServer, f *file,
) error {
	uri := f.uri

	h.mu.Lock()
	rs, ok := h.pendingGoTo[uri.String()]
	delete(h.pendingGoTo, uri.String())
	h.mu.Unlock()
	if !ok {
		return nil
	}

	return h.handleGoTo(f, rs)
}

func (h *lspEditorHandler) getFile(uri workspaceapi.URI) (*file, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	f, ok := h.files[uri.String()]
	return f, ok
}

func parseLocationData(
	uri span.URI, cells [][]term.Cell, content []byte, d []uint32,
	semanticTypes map[string]term.Attributes,
) (ret []textapi.Location) {
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

		loc := textapi.Location{From: from, To: to, Attr: attr}
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
) error {
	ext := filepath.Ext(f.uri.Path())
	if enabled, ok := h.enableSemanticTokens[ext]; !ok || !enabled {
		log.Debugf("lspEditorHandler.Server.SemanticTokensFull(%s): disabled for file with extension %s",
			f.uri, ext)
		return nil
	}
	// NOTE: gopls does not pass semanticTokensProvider
	if srv.caps.SemanticTokensProvider == nil && srv.langID != ".go" {
		log.Debugf("lspEditorHandler.Server.SemanticTokensFull(%s): server does not support semantic tokens", f.uri)
		return nil
	}
	version := h.getVersion(f)
	p2 := protocol.SemanticTokensParams{TextDocument: f.docID}
	resp, err := srv.srv.SemanticTokensFull(ctx, &p2)
	if err != nil || resp == nil {
		err = fmt.Errorf("Server.SemanticTokensFull(%s): %v", f.uri, err)
		return err
	}
	if resp == nil {
		return errors.New("empty Server.SemanticTokensFull response")
	}
	if h.getVersion(f) != version {
		log.Warnf("lspEditorHandler.Server.SemanticTokensFull(%s): stale result", f.uri)
		return nil
	}

	spanURI := workspaceURIToSpan(f.uri)
	locations := parseLocationData(spanURI, cells, []byte(content), resp.Data,
		h.semanticTypesAttr)
	if log.IsLevelEnabled(log.TraceLevel) {
		log.Tracef("lspEditorHandler.Server.SemanticTokensFull(%s): OK: %v: locations: %v",
			f.uri, resp.Data, locations)
	}
	err = h.ed.SetLocationList(f.handler, textapi.LocationPriorityInfo,
		h.semanticTokensListID, textapi.LocationSlice(locations))
	if err != nil {
		err = fmt.Errorf("SetLocationList(%s): %v", f.uri, err)
		return err
	}
	return nil
}

// https://microsoft.github.io/language-server-protocol/specifications/specification-current/#range
func makeProtocolRange(
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
		err = fmt.Errorf("Server.DidChange(%s): %v", f.uri, err)
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
func (h *lspEditorHandler) pushFullEdit(
	ctx context.Context, srv execServer, f *file, content string,
) error {
	version := h.incrementVersion(f)

	evts := []protocol.TextDocumentContentChangeEvent{{Text: content}}
	err := h.callServerDidChange(ctx, srv, f, evts)
	if err != nil {
		err = fmt.Errorf("failed to send file update: file=%v, length=%v, version=%v, err=%s",
			f.uri, len(content), version, err)
		return err
	}
	log.Tracef("sent full file update: file=%v, length=%v, version=%v",
		f.uri, len(content), version)
	return nil
}

func (h *lspEditorHandler) sendIncrementalEdit(
	ctx context.Context, srv execServer, f *file, newCells,
	oldCells [][]term.Cell, content string, from, to term.Coordinates,
) (protocol.Range, error) {
	version := h.incrementVersion(f)

	// https://microsoft.github.io/language-server-protocol/specification#textDocument_didChange
	rng := makeProtocolRange(oldCells, from, to)
	evts := []protocol.TextDocumentContentChangeEvent{{Text: content, Range: &rng}}

	log.Tracef("sending incremental file update: file=%v, length=%v, version=%v,"+
		" rangeStart: %#v, rangeEnd: %#v, from=%#v, to=%#v: content='%s'",
		f.uri, len(content), version, rng.Start, rng.End, from, to, content)

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

func (h *lspEditorHandler) handleFileFlush(ev textapi.Event) error {
	ctx := context.Background()
	ctx, cancelFn := context.WithTimeout(ctx, h.rpcTimeout)
	defer cancelFn()

	// content last EOL is trimmed by the buffer's unix file reader.
	// lsp expects the last EOL
	ev.Content += "\n"

	f, ok := h.getFile(ev.URI)
	if !ok {
		f = h.newFile(ev.Resource, ev.URI, ev.Content)
	}

	srv, ok := h.getServer(f.languageID)
	if !ok {
		return errNoServer
	}

	err := h.pushFullEdit(ctx, srv, f, ev.Content)
	if err != nil {
		return err
	}
	h.setCells(f, cell.StringToCells(ev.Content, h.tabspaces))
	ctx = h.newSemanticTokensCtx()
	return h.semanticTokensFull(ctx, srv, f, h.getCells(f), ev.Content)
}

func (h *lspEditorHandler) handleFileEdit(ev textapi.Event) error {
	ctx := context.Background()
	ctx, cancelFn := context.WithTimeout(ctx, h.rpcTimeout)
	defer cancelFn()
	f, ok := h.getFile(ev.URI)
	if !ok {
		err := fmt.Errorf("Received insert/delete event for an unknown file: %#v", ev)
		return err
	}

	srv, ok := h.getServer(f.languageID)
	if !ok {
		return errNoServer
	}

	var ret error
	// text.Editor requires clients to re-send locations on every update.
	// unfortunately it seems that the LSP spec is a bit confusing regarding
	// what to do when there are updates to the buffer but changes do not affect diagnostics.
	// Certain LSP servers (rls, clangd, tsserver) re-send the diagnostics
	// after every update. Some other LSP servers do not follow this behavior so
	// we need to send the last known diagnostics and hope that if the locations are incorrect,
	// the LSP server will overwrite them:
	// See discussion https://github.com/microsoft/language-server-protocol/issues/1217
	// Fix I submitted to gopls and was rejected https://go-review.googlesource.com/c/tools/+/298853
	ds := h.getDiagnostics(f)
	if len(ds) != 0 {
		if err := h.setDiagnosticsLocationList(ctx, f, ds); err != nil {
			ret = multierr.Append(ret, err)
		}
	}

	oldCells := h.getCells(f)
	buf := cell.CellsToBuffer(oldCells, h.tabspaces)
	buf.Edit(ev.Start, ev.End, ev.Content)
	newCells := buf.RawCells()
	if _, err := h.sendIncrementalEdit(ctx, srv, f, newCells, oldCells,
		ev.Content, ev.Start, ev.End); err != nil {
		ret = multierr.Append(ret, err)
	}
	h.setCells(f, newCells)

	ctx = h.newSemanticTokensCtx()
	if err := h.semanticTokensFull(ctx, srv, f, newCells, buf.String()); err != nil {
		ret = multierr.Append(ret, err)
	}
	return ret
}

func (h *lspEditorHandler) handleFileOpen(ev textapi.Event) error {
	ctx := context.Background()
	ctx, cancelFn := context.WithTimeout(ctx, h.rpcTimeout)
	defer cancelFn()

	// content last EOL is trimmed by the buffer's unix file reader.
	// lsp expects the last EOL
	ev.Content += "\n"

	f := h.newFile(ev.Resource, ev.URI, ev.Content)
	srv, ok := h.getServer(f.languageID)
	if !ok {
		return errNoServer
	}

	spanURI := workspaceURIToSpan(f.uri)
	p := protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        protocol.URIFromSpanURI(spanURI),
			LanguageID: f.languageID,
			Version:    h.getVersion(f),
			Text:       ev.Content,
		},
	}

	if err := srv.srv.DidOpen(ctx, &p); err != nil {
		err = fmt.Errorf("Server.DidOpen(%s, %s): %v",
			f.uri, f.languageID, err)
		return err
	}
	log.Tracef("lspEditorHandler.Server.DidOpen(%s, %s)", f.uri, f.languageID)

	var ret error
	if err := h.dispatchPendingDiagnostics(ctx, f.uri); err != nil {
		ret = multierr.Append(ret, err)
	}
	if err := h.dispatchPendingGoTo(ctx, srv, f); err != nil {
		ret = multierr.Append(ret, err)
	}
	ctx = h.newSemanticTokensCtx()
	if err := h.semanticTokensFull(ctx, srv, f, h.getCells(f), ev.Content); err != nil {
		ret = multierr.Append(ret, err)
	}
	return ret
}

func (h *lspEditorHandler) removeFile(resource workspaceapi.URI) (*file, bool) {
	f, ok := h.getFile(resource)
	if !ok {
		return nil, false
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.files, f.uri.String())

	return f, true
}

func (h *lspEditorHandler) sendDidClose(
	ctx context.Context, srv execServer, uri workspaceapi.URI,
) error {
	spanURI := workspaceURIToSpan(uri)
	p := protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: protocol.URIFromSpanURI(spanURI),
		},
	}

	if err := srv.srv.DidClose(ctx, &p); err != nil {
		err = fmt.Errorf("Server.DidClose(%s): %v", uri, err)
		return err
	}
	log.Tracef("lspEditorHandler.Server.DidClose(%s)", uri)
	return nil
}

func (h *lspEditorHandler) handleFileClose(ev textapi.Event) error {
	ctx := context.Background()
	ctx, cancelFn := context.WithTimeout(ctx, h.rpcTimeout)
	defer cancelFn()
	f, ok := h.removeFile(ev.URI)
	if !ok {
		err := fmt.Errorf("Received close event for an unknown file: %#v", ev)
		return err
	}
	srv, ok := h.getServer(f.languageID)
	if !ok {
		return errNoServer
	}

	var ret error
	if err := h.sendDidClose(ctx, srv, f.uri); err != nil {
		ret = multierr.Append(ret, err)
	}
	h.addPendingDiagnostics(f.uri, h.getDiagnostics(f))
	return ret
}

func (h *lspEditorHandler) parseDiagnostics(
	f *file, d []protocol.Diagnostic,
) []textapi.Location {
	cells := h.getCells(f)
	buf := cell.CellsToBuffer(cells, h.tabspaces)
	spanURI := workspaceURIToSpan(f.uri)
	colmap := getColumnMapper(spanURI, buf)

	locs := make([]textapi.Location, 0, len(d))
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

		loc := textapi.Location{Message: msg, From: from, To: to, Attr: attr}
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
	ctx context.Context, file workspaceapi.URI,
	ds []protocol.Diagnostic, version int32,
) error {
	f, ok := h.getFile(file)
	if !ok {
		h.addPendingDiagnostics(file, ds)
		log.Tracef("lspEditorHandler: Received diagnostic for a unopened file: %#v", file)
		return nil
	}

	h.setDiagnostics(f, ds)

	if version != h.getVersion(f) {
		log.Debugf("lspEditorHandler: Received diagnostic for "+
			"outdated version of file '%s': %#v", f.uri, version)
		// it seems that tsserver does not send any version info
		// return
	}

	return h.setDiagnosticsLocationList(ctx, f, ds)
}

func (h *lspEditorHandler) setDiagnosticsLocationList(
	ctx context.Context, f *file, ds []protocol.Diagnostic,
) error {
	locs := h.parseDiagnostics(f, ds)
	// NOTE: should probably break down by location priority rather than
	// bundling all of them under Error.
	err := h.ed.SetLocationList(f.handler, textapi.LocationPriorityError,
		h.diagnosticListID, textapi.LocationSlice(locs))
	if err != nil {
		err = fmt.Errorf("SetLocationList(%s): %v", f.uri, err)
		return err
	}
	return err
}

func (h *lspEditorHandler) spanURIToWorkspace(u span.URI) (workspaceapi.URI, error) {
	localFile, err := workspaceapi.ParseURI(string(u))
	if err != nil {
		err = fmt.Errorf("ParseURI: convert LSP URI to workspaceapi.URI %s: %s", u, err)
		return workspaceapi.URI{}, err
	}
	// convert local LSP file URI to the current workspace's URI scheme
	// which could be remote or something else.
	workspaceFile, err := h.fs.URI(localFile.Path())
	if err != nil {
		return workspaceapi.URI{}, fmt.Errorf("workspaceapi.URI: %s", err)
	}
	return workspaceFile, err
}

func workspaceURIToSpan(u workspaceapi.URI) span.URI {
	// the workspace is always local for the language server
	// if URI is a remote uri, then the language server is executed
	// in the remote host as well.
	return span.URI(fmt.Sprintf("file://%s", u.Path()))
}

func (h *lspEditorHandler) HandleDiagnostics(
	ctx context.Context, p *protocol.PublishDiagnosticsParams,
) error {
	var start time.Time
	if log.IsLevelEnabled(log.TraceLevel) {
		start = time.Now()
		log.Tracef("lspEditorHandler.HandleDiagnostics(%#v)", p.URI)
	}
	file, err := h.spanURIToWorkspace(p.URI.SpanURI())
	if err != nil {
		return err
	}

	err = h.handleDiagnostics(ctx, file, p.Diagnostics, p.Version)

	if log.IsLevelEnabled(log.TraceLevel) {
		log.Tracef("lspEditorHandler.HandleDiagnostics(%#v) in %s", p.URI, time.Since(start))
	}

	return err
}

func (h *lspEditorHandler) goToLocation(win browserapi.Window, l protocol.Location) error {
	uri, err := h.spanURIToWorkspace(l.URI.SpanURI())
	if err != nil {
		return err
	}
	f, alreadyOpen := h.getFile(uri)
	if !alreadyOpen {
		h.addPendingGoTo(uri, l.Range)
	}

	buf, err := h.o.Open(uri)
	if err != nil {
		err = fmt.Errorf("browser.Open(%s): %v", uri, err)
		h.removePendingGoTo(uri)
		return err
	}

	if alreadyOpen {
		err := h.handleGoTo(f, l.Range)
		if err != nil {
			return err
		}
	}

	err = win.SetContent(buf)
	if err != nil && err != browserapi.ErrTabNotFree {
		err = fmt.Errorf("win.SetContent: %v", err)
		return err
	}
	return nil
}

func (h *lspEditorHandler) getFilePosition(cursor term.Coordinates, uri workspaceapi.URI) (
	f *file, pos protocol.Position, ok bool,
) {
	f, ok = h.getFile(uri)
	if !ok {
		return
	}

	line, column, ok := cell.ConvertTermCoordinates(h.getCells(f), cursor)
	if !ok {
		return
	}

	ok = true
	pos = protocol.Position{
		Line:      uint32(line),
		Character: uint32(column),
	}

	return
}

func (h *lspEditorHandler) handleGoToDefinition(
	cursor term.Coordinates, ed textapi.Handler, uri workspaceapi.URI,
	win browserapi.Window,
) error {
	f, pos, ok := h.getFilePosition(cursor, uri)
	if !ok {
		return fmt.Errorf("resource with URI %q not found", uri)
	}

	p := protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: f.docID,
			Position:     pos,
		},
	}

	srv, ok := h.getServer(f.languageID)
	if !ok {
		return errNoServer
	}

	ctx := context.Background()
	ctx, cancelFn := context.WithTimeout(ctx, h.rpcTimeout)
	defer cancelFn()

	locs, err := srv.srv.Definition(ctx, &p)
	if err != nil {
		err = fmt.Errorf("Server.Definition(%s, %s): %v", f.uri, f.languageID, err)
		return err
	}

	log.Tracef("lspEditorHandler.Server.Definition(%s, %s): %#v",
		f.uri, f.languageID, locs)

	if len(locs) == 0 {
		err = errors.New("no definitions found for symbol at position")
		return err
	}

	for _, l := range locs {
		if gerr := h.goToLocation(win, l); gerr != nil {
			err = multierr.Append(err, gerr)
		}
	}
	return err
}

func (h *lspEditorHandler) findBestFloatingWindowPosition(
	cursorAtWindow term.Coordinates, height int,
) term.Coordinates {
	at := cursorAtWindow
	// first try to set it above cursor, otherwise below if it's too large
	at.Y -= height
	if at.Y < 0 {
		at.Y = cursorAtWindow.Y + 1
	}

	return at
}

func (h *lspEditorHandler) handleHover(
	cursorAtScroll, cursorAtWindow term.Coordinates,
	ed textapi.Handler, uri workspaceapi.URI,
) error {
	f, pos, ok := h.getFilePosition(cursorAtScroll, uri)
	if !ok {
		return fmt.Errorf("resource with URI %q not found", uri)
	}

	p := protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: f.docID,
			Position:     pos,
		},
	}

	srv, ok := h.getServer(f.languageID)
	if !ok {
		return errNoServer
	}

	ctx := context.Background()
	ctx, cancelFn := context.WithTimeout(ctx, h.rpcTimeout)
	defer cancelFn()

	hover, err := srv.srv.Hover(ctx, &p)
	if err != nil || hover == nil {
		err = fmt.Errorf("Server.Hover(%s, %s): %v", f.uri, f.languageID, err)
		return err
	}

	log.Tracef("lspEditorHandler.Server.Hover(%s, %s): %#v", f.uri, f.languageID, hover)

	less := handler.NewLess(handler.DefaultLessConfig())
	less.Buffer().WriteString(hover.Contents.Value)
	padx, pady := 1, 1
	if h.frame {
		pady += 2
		padx += 2
	}
	less.Scroll().Attributes = h.referencesWindowAttributes
	bh := browserapi.NopFloatingHandler(handler.PaddedFloating(
		handler.FloatingBuffer(less, less.Buffer()), padx, pady))

	at := h.findBestFloatingWindowPosition(cursorAtWindow, less.Buffer().Rows())
	_, err = h.wm.Floating(bh, component.FloatingConfig{Offset: at})
	if err != nil {
		err = fmt.Errorf("wm.Floating: %v", err)
		return err
	}
	return nil
}

func makeWorkspaceFolder(in string) protocol.WorkspaceFolder {
	return protocol.WorkspaceFolder{
		URI:  string(protocol.URIFromPath(in)),
		Name: in,
	}
}

func (h *lspEditorHandler) handleChangedWorkspace(
	uri workspaceapi.URI, added []string, removed []string,
) error {
	languageID := filepath.Ext(uri.Path())
	srv, ok := h.getServer(languageID)
	if !ok {
		return errNoServer
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
		err = fmt.Errorf("Server.DidChangeWorkspaceFolders(%s): %v", uri, err)
		return err
	}
	return nil
}

func (h *lspEditorHandler) handleAddWorkspace(uri workspaceapi.URI, args []string) error {
	log.Tracef("lspEditorHandler.handleAddWorkspace(%v)", args)
	return h.handleChangedWorkspace(uri, args, nil)
}

func (h *lspEditorHandler) handleRemoveWorkspace(uri workspaceapi.URI, args []string) error {
	log.Tracef("lspEditorHandler.handleRemoveWorkspace(%v)", args)
	return h.handleChangedWorkspace(uri, nil, args)
}

func (h *lspEditorHandler) browseLocations(
	win browserapi.Window, locs []protocol.Location,
) error {
	const locID = "highlight_loc"
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
	textToLocation := make(map[string]protocol.Location)
	buf := cell.NewBuffer()
	ed := vi.Editor()
	edh, err := ed.Edit(workspaceapi.URI{}, buf)
	if err != nil {
		err = fmt.Errorf("ed.Edit: %s", err)
		return err
	}

	for _, l := range locs {
		uri := l.URI.SpanURI()
		filename := uri.Filename()

		relative, err := filepath.Rel(h.cwd, filename)
		if err == nil && len(relative) < len(filename) {
			filename = relative
		}
		text := fmt.Sprintf("%s:%#v", filename, l.Range)
		list.PushSync([]byte(text))
		textToLocation[text] = l
		if len(text) > longestLocation {
			longestLocation = len(text)
		}
	}

	closeWin := func(win browserapi.Window) func() error {
		return func() error {
			h.mu.Lock()
			shouldClose := done && win != nil
			closeWin := win
			h.mu.Unlock()
			if shouldClose {
				return closeWin.Close()
			}
			return nil
		}
	}

	renderFile := func(l protocol.Location) {
		uri := l.URI.SpanURI()
		// Open assumes path in current workspace
		f, oerr := h.fs.Open(uri.Filename(), os.O_RDONLY, 0)
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
		buf.Reset()
		buf.WriteString(content)

		cells := buf.RawCells()
		colmap := getColumnMapper(uri, buf)
		from, to, ok := convertRange(l.Range, cells, colmap)
		if !ok {
			log.Debugf("lspEditorHandler.convertRange: %v", ok)
			return
		}

		attrs := term.Attributes{Bg: term.AttrReverse, Fg: term.AttrReverse}
		loc := textapi.Location{From: from, To: to, Attr: attrs}
		ed.SetLocationList(edh, textapi.LocationPriorityInfo,
			locID, textapi.LocationSlice([]textapi.Location{loc}))
		ed.MoveToPrevLocation(edh, locID)
	}

	sh := search.Handler(list, func(text string) {
		h.mu.Lock()
		done = true
		h.mu.Unlock()

		// liberate all tabs
		closeWin(top)()
		closeWin(bottom)()

		err := h.goToLocation(win, textToLocation[text])
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
			renderFile(textToLocation[afterStr])
		}
		return exit, handle
	})

	eh := handler.Wrap(handler.Sync(&h.mu, edh), func(ev term.Event) (bool, bool) {
		h.mu.Lock()
		defer h.mu.Unlock()

		if ev.Type == term.EventResize {
			list.Wait()
			focus, _ := list.Focus()
			renderFile(textToLocation[string(focus.Data())])
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

func (h *lspEditorHandler) handleReferences(
	cursorAtScroll, cursorAtWindow term.Coordinates,
	ed textapi.Handler, uri workspaceapi.URI, win browserapi.Window,
) error {
	f, pos, ok := h.getFilePosition(cursorAtScroll, uri)
	if !ok {
		return fmt.Errorf("resource with URI %q not found", uri)
	}

	p := protocol.ReferenceParams{
		Context: protocol.ReferenceContext{IncludeDeclaration: true},
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: f.docID,
			Position:     pos,
		},
	}

	srv, ok := h.getServer(f.languageID)
	if !ok {
		return errNoServer
	}

	ctx := context.Background()
	ctx, cancelFn := context.WithTimeout(ctx, h.rpcTimeout)
	defer cancelFn()

	locs, err := srv.srv.References(ctx, &p)
	if err != nil {
		err = fmt.Errorf("Server.References(%s, %s): %v", f.uri, f.languageID, err)
		return err
	}

	log.Tracef("lspEditorHandler.Server.References(%s, %s): %#v", f.uri, f.languageID, locs)

	if len(locs) == 0 {
		err = errors.New("no references found for symbol at position")
		return err
	}

	return h.browseLocations(win, locs)
}

func toJSONEdits(edits []protocol.TextEdit) string {
	m, _ := json.Marshal(edits)
	return string(m)
}

func (h *lspEditorHandler) format(
	ctx context.Context, f *file, srv execServer, builder *editBuilder,
) error {
	p := protocol.DocumentFormattingParams{
		TextDocument: f.docID,
		Options: protocol.FormattingOptions{
			TabSize:                uint32(h.tabspaces),
			InsertSpaces:           false,
			TrimTrailingWhitespace: true,
			InsertFinalNewline:     false,
			TrimFinalNewlines:      false,
		},
	}

	edits, err := srv.srv.Formatting(ctx, &p)
	if err != nil {
		err = fmt.Errorf("Server.Formatting(%s): %v", f.uri, err)
		return err
	}

	// json marshaling is expensive
	if log.IsLevelEnabled(log.TraceLevel) {
		log.Tracef("lspEditorHandler.Server.Formatting(%s): %v", f.uri, toJSONEdits(edits))
	}

	return builder.applyEdits(edits)
}

func (h *lspEditorHandler) organizeImports(
	ctx context.Context, f *file, srv execServer, builder *editBuilder,
) error {
	p := protocol.CodeActionParams{
		TextDocument: f.docID,
		Context: protocol.CodeActionContext{
			Only: []protocol.CodeActionKind{protocol.Source, protocol.SourceOrganizeImports},
		},
	}
	codeActions, err := srv.srv.CodeAction(ctx, &p)
	if err != nil {
		err = fmt.Errorf("Server.CodeAction(%s): %v", f.uri, err)
		return err
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	var ret error
	for _, ca := range codeActions {
		switch ca.Kind {
		case protocol.Source, protocol.SourceOrganizeImports:
			log.Tracef("lspEditorHandler.Server.CodeAction(%s): %v", f.uri, ca.Edit)
			if err := builder.applyWorkspaceEdit(ca.Edit); err != nil {
				ret = multierr.Append(ret, err)
			}
		}
	}
	return ret
}

func (h *lspEditorHandler) handleFormat(ed textapi.Handler, uri workspaceapi.URI, imports bool) error {
	ctx := context.Background()
	ctx, cancelFn := context.WithTimeout(ctx, h.rpcTimeout)
	defer cancelFn()

	f, ok := h.getFile(uri)
	if !ok {
		err := fmt.Errorf("extraneous file %q", uri.String())
		return err
	}

	srv, ok := h.getServer(f.languageID)
	if !ok {
		return errNoServer
	}
	w := h.ed.CellEditor(ed)

	// this cells are used to map edit ranges to term.Coordinates
	// but discarded because only Insert/Delete events should
	// apply changes to the local file copy.
	cells := h.getCells(f)

	var b editBuilder
	b.init(h.tabspaces, f, w, cells)

	var err error
	if imports {
		err = h.organizeImports(ctx, f, srv, &b)
	} else {
		err = h.format(ctx, f, srv, &b)
	}
	return err
}

func (h *lspEditorHandler) HandleCommand(
	ctx context.Context, cmd textapi.Command,
) (exit bool, err error) {
	if cmd.Resource == nil {
		return
	}

	switch cmd.Name {
	case commandNextDiagnostic:
		err = h.ed.MoveToNextLocation(cmd.Resource, h.diagnosticListID)
		if err != nil {
			err = fmt.Errorf("MoveToNextLocation(%s): %v", cmd.Name, err)
		}
	case commandPrevDiagnostic:
		err = h.ed.MoveToPrevLocation(cmd.Resource, h.diagnosticListID)
		if err != nil {
			err = fmt.Errorf("MoveToPrevLocation(%s): %v", cmd.Name, err)
		}
	case commandHover:
		err = h.handleHover(cmd.Cursor.Content, cmd.Cursor.Window, cmd.Resource, cmd.URI)
	case commandGoToDef:
		err = h.handleGoToDefinition(cmd.Cursor.Content, cmd.Resource, cmd.URI, cmd.Window)
	case commandReferences:
		err = h.handleReferences(cmd.Cursor.Content, cmd.Cursor.Window, cmd.Resource, cmd.URI, cmd.Window)
	case commandAddWorkspace:
		err = h.handleAddWorkspace(cmd.URI, cmd.Args)
	case commandRemoveWorkspace:
		err = h.handleRemoveWorkspace(cmd.URI, cmd.Args)
	case commandFormat:
		err = h.handleFormat(cmd.Resource, cmd.URI, false)
	case commandOrganizeImports:
		err = h.handleFormat(cmd.Resource, cmd.URI, true)
	}

	return
}

func (h *lspEditorHandler) handleEvents(ch chan textapi.Event) {
	for ev := range ch {
		var start time.Time
		if log.IsLevelEnabled(log.TraceLevel) {
			start = time.Now()
			log.Tracef("lspEditorHandler.Handle(%#v)", ev)
		}

		var err error
		switch ev.Type {
		case textapi.EventTypeOpen:
			err = h.handleFileOpen(ev)
		case textapi.EventTypeClose:
			err = h.handleFileClose(ev)
		case textapi.EventTypeFlush:
			err = h.handleFileFlush(ev)
		case textapi.EventTypeEdit:
			err = h.handleFileEdit(ev)
		}

		if log.IsLevelEnabled(log.TraceLevel) {
			log.Tracef("lspEditorHandler.Handle(%#v) in %s: %s", ev, time.Since(start), err)
		}
		if err != nil && err != errNoServer {
			h.m.SetMessage("%s", err)
		}
	}
}

func (h *lspEditorHandler) Handle(
	ctx context.Context, ev textapi.Event,
) (exit bool) {
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
	h.exit = true
	close(h.evChan)
	h.mu.Unlock()

	log.Debugf("shutting down %d servers", len(h.servers))

	var wg sync.WaitGroup
	var i int
	errors := make([]error, len(h.servers))
	wg.Add(len(h.servers))
	for _, server := range h.servers {
		go func(ret *error, server execServer) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), h.disconnectTimeout)
			defer cancel()
			err := server.srv.Shutdown(ctx)
			if err != nil {
				*ret = multierr.Append(*ret, fmt.Errorf("shutdown: %v", err))
			}
			if err := h.exec.Signal(server.pid, syscall.SIGKILL); err != nil {
				*ret = multierr.Append(*ret, fmt.Errorf("signal: %v", err))
			}
			for _, closer := range server.closers {
				if err := closer.Close(); err != nil {
					*ret = multierr.Append(*ret, fmt.Errorf("pipe close: %v", err))
				}
			}
		}(&errors[i], server)
		i++
	}
	wg.Wait()

	var ret error
	for _, err := range errors {
		if err != nil {
			ret = multierr.Append(ret, err)
		}
	}
	level := log.InfoLevel
	if ret != nil {
		level = log.ErrorLevel
	}

	log.WithFields(log.Fields{}).
		Logf(level, "shut down servers: %d: %v", len(h.servers), ret)

	return ret
}
