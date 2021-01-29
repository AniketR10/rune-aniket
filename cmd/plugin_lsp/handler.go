package main

//go:generate mockgen -destination=./lsp_server_gomock.go -package main -self_package main github.com/ernestrc/golang-internal-tools/lsp/protocol Server

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/editor"
	"github.com/ernestrc/go-tui/plugin"
	"github.com/ernestrc/go-tui/term"
	"github.com/ernestrc/golang-internal-tools/jsonrpc2"
	"github.com/ernestrc/golang-internal-tools/lsp"
	"github.com/ernestrc/golang-internal-tools/lsp/lsprpc"
	"github.com/ernestrc/golang-internal-tools/lsp/protocol"
	"github.com/ernestrc/golang-internal-tools/lsp/source"
	"github.com/ernestrc/golang-internal-tools/span"
	log "github.com/sirupsen/logrus"
)

const (
	connectTimeout = 5 * time.Second
)

var (
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
	name    string
	version float64
	handler editor.Handler
	docID   protocol.TextDocumentIdentifier
	uri     span.URI
	cells   [][]term.Cell
}

type lspEditorHandler struct {
	mu                   sync.Mutex
	ed                   editor.Editor
	p                    browser.EventPublisher
	server               protocol.Server
	protocol             *protocol.InitializeResult
	files                map[span.URI]*file
	semanticTypesAttr    map[string]term.Attributes
	diagnosticAttr       map[protocol.DiagnosticSeverity]term.Attributes
	semanticTokensListID string
	diagnosticListID     string
}

func initializeParams(
	ctx context.Context, cwd string, server protocol.Server,
) (*protocol.InitializeResult, error) {
	params := &protocol.ParamInitialize{}
	params.RootURI = protocol.URIFromPath(cwd)
	params.Capabilities.Workspace.Configuration = true

	// Make sure to respect configured options when sending initialize request.
	opts := source.DefaultOptions().Clone()

	params.Capabilities.TextDocument.Hover = protocol.HoverClientCapabilities{
		ContentFormat: []protocol.MarkupKind{opts.PreferredContentFormat},
	}
	params.Capabilities.TextDocument.DocumentSymbol.HierarchicalDocumentSymbolSupport = opts.HierarchicalDocumentSymbolSupport
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

	log.Debugf("initializing server with params %#v", params)

	res, err := server.Initialize(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("lsp.Server.Initialize: %v", err)
	}
	err = server.Initialized(ctx, &protocol.InitializedParams{})
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
		log.Debugf("jsonrpc2 processing goroutine error: %v", err)
	}

}

func connectRemote(ctx context.Context, ret *lspEditorHandler, remote string) (
	protocol.Server, *protocol.InitializeResult, error,
) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, nil, fmt.Errorf("Getwd: %v", err)
	}

	network, addr := parseAddr(remote)

	log.Debugf("connecting to LSP remote server at network %s and addr %s", network, addr)
	conn, err := lsprpc.ConnectToRemote(ctx, network, addr)
	if err != nil {
		return nil, nil, err
	}

	log.Debugf("connected to LSP remote server at network %s and addr %s", network, addr)
	stream := jsonrpc2.NewHeaderStream(conn)
	cc := jsonrpc2.NewConn(stream)
	server := protocol.ServerDispatcher(cc)
	go streamRPC(cc, ret)
	res, err := initializeParams(ctx, cwd, server)
	if err != nil {
		return nil, nil, err
	}
	log.Debugf("initialized LSP client parameters")
	return server, res, nil
}

// TODO automatically spin lsp server or attach to it
// based on the filetype and provided pconfig type to lsp server executable
func getRemoteAddr(pconfig plugin.Config) (string, error) {
	addr, err := pconfig.GetString("remote")
	if err != nil {
		err = fmt.Errorf("Failed to get remote lsp server address: %v", err)
		return "", err
	}
	return addr, nil
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
		return "information"
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

func newLspHandler(
	ed editor.Editor, p browser.EventPublisher, pconfig plugin.Config,
) (*lspEditorHandler, error) {
	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()

	remoteAddr, err := getRemoteAddr(pconfig)
	if err != nil {
		return nil, err
	}
	ret := new(lspEditorHandler)
	ret.ed = ed
	ret.p = p
	ret.files = make(map[span.URI]*file)
	ret.server, ret.protocol, err = connectRemote(ctx, ret, remoteAddr)
	if err != nil {
		return nil, err
	}

	ret.server = ioUnlockServer{mu: &ret.mu, server: ret.server}

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

	log.Infof("connected to remote server named %s with version %s at %s: ",
		ret.protocol.ServerInfo.Name, ret.protocol.ServerInfo.Version, remoteAddr)

	return ret, nil
}

func (h *lspEditorHandler) newFile(handler editor.Handler, name, content string) *file {
	uri := span.URIFromPath(name)

	f := &file{
		version: 1,
		handler: handler,
		name:    name,
		docID: protocol.TextDocumentIdentifier{
			URI: protocol.URIFromSpanURI(uri),
		},
		uri:   uri,
		cells: cell.StringToCells(content),
	}

	h.files[uri] = f
	return f
}

func (h *lspEditorHandler) getFileWithName(name string) (*file, bool) {
	uri := span.URIFromPath(name)
	return h.getFile(uri)
}

func (h *lspEditorHandler) getFile(uri span.URI) (*file, bool) {
	f, ok := h.files[uri]
	return f, ok
}

func parseLocationData(
	uri span.URI, cells [][]term.Cell, content []byte, d []float64,
	semanticTypes map[string]term.Attributes,
) (ret []editor.Location) {
	tc := span.NewContentConverter(uri.Filename(), content)
	colmap := protocol.ColumnMapper{
		URI:       uri,
		Content:   content,
		Converter: tc,
	}

	lspLine := make([]float64, len(d)/5)
	lspChar := make([]float64, len(d)/5)
	line, char := 0.0, 0.0
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
				Line:      lspLine[i],
				Character: lspChar[i],
			},
			End: protocol.Position{
				Line:      lspLine[i],
				Character: lspChar[i] + d[5*i+2],
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

// https://microsoft.github.io/language-server-protocol/specifications/specification-current/#textDocument_semanticTokens
func (h *lspEditorHandler) semanticTokens(
	ctx context.Context, f *file, cells [][]term.Cell, content string,
) {
	// WorkDoneProgressParams
	// PartialResultParams
	p2 := protocol.SemanticTokensParams{TextDocument: f.docID}
	resp, err := h.server.SemanticTokensFull(ctx, &p2)
	if err != nil {
		log.Errorf("lspEditorHandler.Server.SemanticTokensFull(%s): %v", f.name, err)
		return
	}

	locations := parseLocationData(f.uri, cells, []byte(content), resp.Data, h.semanticTypesAttr)
	err = h.ed.SetLocationList(f.handler, h.semanticTokensListID, editor.LocationSlice(locations))
	if err != nil {
		log.Errorf("lspEditorHandler.SetLocationList(%s): %v", f.name, err)
		return
	}
}

func (h *lspEditorHandler) publishInterrupt(filename string) {
	if h.p != nil {
		err := h.p.PublishInterrupt()
		if err != nil {
			log.Errorf("lspEditorHandler.PublishInterrupt(%s): %v", filename, err)
		}
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
			Line:      float64(starty),
			Character: float64(startx),
		},
		End: protocol.Position{
			Line:      float64(starty),
			Character: float64(startx),
		},
	}
}

func makeDeleteProtocolRange(
	oldCells [][]term.Cell, from, to term.Coordinates,
) protocol.Range {
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
			Line:      float64(starty),
			Character: float64(startx),
		},
		End: protocol.Position{
			Line:      float64(endy),
			Character: float64(endx),
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
	ctx context.Context, f *file, evts []protocol.TextDocumentContentChangeEvent,
) error {
	params := protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			Version:                f.version,
			TextDocumentIdentifier: f.docID,
		},
		ContentChanges: evts,
	}
	err := h.server.DidChange(ctx, &params)
	if err != nil {
		log.Errorf("lspEditorHandler.Server.DidChange(%s): %v", f.name, err)
	}
	return err
}

// TODO client is expected to support both incremental and full synchronization
// based on h.protocol server capabilities. Right now we are assuming incremental.
func (h *lspEditorHandler) pushFullUpdate(ctx context.Context, f *file, content string) {
	f.version++

	evts := []protocol.TextDocumentContentChangeEvent{{Text: content}}
	err := h.callServerDidChange(ctx, f, evts)
	if err == nil {
		log.Tracef("sent full file update: file=%v, length=%v, version=%v",
			f.name, len(content), f.version)
	}
}

func (h *lspEditorHandler) sendIncrementalUpdate(
	ctx context.Context, f *file, newCells, oldCells [][]term.Cell,
	content string, from, to term.Coordinates,
) {
	f.version++

	// https://microsoft.github.io/language-server-protocol/specification#textDocument_didChange
	rng := makeProtocolRange(content, newCells, oldCells, from, to)
	evts := []protocol.TextDocumentContentChangeEvent{{Text: content, Range: &rng}}

	log.Tracef("handling file update with range: from=%#v, to=%#v: content='%s'",
		rng.Start, rng.End, content)

	err := h.callServerDidChange(ctx, f, evts)
	if err == nil {
		log.Tracef("sent incremental file update: file=%v, length=%v, version=%v,"+
			" rangeStart: %#v, rangeEnd: %#v, from=%#v, to=%#v: content='%s'",
			f.name, len(content), f.version, rng.Start, rng.End, from, to, content)
	}
}

func (h *lspEditorHandler) handleFileFlush(ev editor.Event) {
	ctx := context.Background()

	f, ok := h.getFileWithName(ev.ResourceName)
	if !ok {
		f = h.newFile(ev.Resource, ev.ResourceName, ev.Content)
	}

	h.pushFullUpdate(ctx, f, ev.Content)
	f.cells = cell.StringToCells(ev.Content)
	h.semanticTokens(ctx, f, f.cells, ev.Content)
	h.publishInterrupt(f.name)
}

func (h *lspEditorHandler) handleFileInsert(ev editor.Event) {
	ctx := context.Background()
	f, ok := h.getFileWithName(ev.ResourceName)
	if !ok {
		log.Warnf("lspEditorHandler: Received insert event for an unknown file: %#v", ev)
		return
	}

	buf := cell.CellsToBuffer(f.cells)
	buf.InsertString(ev.Start, ev.Content)
	oldCells := f.cells
	newCells := buf.RawCells()
	h.sendIncrementalUpdate(ctx, f, newCells, oldCells, ev.Content, ev.From, ev.To)
	f.cells = newCells

	h.semanticTokens(ctx, f, newCells, buf.String())
	h.publishInterrupt(f.name)
}

func (h *lspEditorHandler) handleFileDelete(ev editor.Event) {
	ctx := context.Background()
	f, ok := h.getFileWithName(ev.ResourceName)
	if !ok {
		log.Warnf("lspEditorHandler: Received delete event for an unknown file: %#v", ev)
		return
	}

	buf := cell.CellsToBuffer(f.cells)
	buf.Delete(ev.From, ev.To)
	oldCells := f.cells
	newCells := buf.RawCells()
	h.sendIncrementalUpdate(ctx, f, newCells, oldCells, "", ev.From, ev.To)
	f.cells = newCells

	h.semanticTokens(ctx, f, newCells, buf.String())
	h.publishInterrupt(f.name)
}

func (h *lspEditorHandler) handleFileOpen(ev editor.Event) {
	ctx := context.Background()
	f := h.newFile(ev.Resource, ev.ResourceName, ev.Content)
	p := protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        protocol.URIFromSpanURI(f.uri),
			LanguageID: source.DetectLanguage("", f.uri.Filename()).String(),
			Version:    f.version,
			Text:       ev.Content,
		},
	}

	if err := h.server.DidOpen(ctx, &p); err != nil {
		log.Errorf("lspEditorHandler.Server.DidOpen(%s): %v", f.name, err)
		return
	}

	h.semanticTokens(ctx, f, f.cells, ev.Content)
	h.publishInterrupt(f.name)
}

func (h *lspEditorHandler) removeFile(name string) (*file, bool) {
	f, ok := h.getFileWithName(name)
	if !ok {
		return nil, false
	}

	delete(h.files, f.uri)
	return f, true
}

func (h *lspEditorHandler) handleFileClose(ev editor.Event) {
	ctx := context.Background()
	f, ok := h.removeFile(ev.ResourceName)
	if !ok {
		log.Warnf("lspEditorHandler: Received close event for an unknown file: %#v", ev)
		return
	}
	p := protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: protocol.URIFromSpanURI(f.uri),
		},
	}

	if err := h.server.DidClose(ctx, &p); err != nil {
		log.Errorf("lspEditorHandler.Server.DidClose(%s): %v", f.name, err)
		return
	}
}

func parseDiagnostics(
	f *file, d []protocol.Diagnostic,
	diagnosticAttr map[protocol.DiagnosticSeverity]term.Attributes,
) []editor.Location {
	buf := cell.CellsToBuffer(f.cells)
	content := []byte(buf.String())
	tc := span.NewContentConverter(f.uri.Filename(), content)
	colmap := protocol.ColumnMapper{
		URI:       f.uri,
		Content:   content,
		Converter: tc,
	}

	locs := make([]editor.Location, 0, len(d))
	for _, d := range d {
		from, to, ok := convertRange(d.Range, f.cells, colmap)
		if !ok {
			continue
		}

		// TODO add Message to editor.Location
		// msg := d.Source + d.Message
		attr, ok := diagnosticAttr[d.Severity]
		if !ok {
			log.Warnf("could not diagnostic severity %v; skipping diagnostic", d.Severity)
			continue
		}

		loc := editor.Location{From: from, To: to, Attr: attr}
		locs = append(locs, loc)
	}

	return locs
}

func (h *lspEditorHandler) HandleDiagnostics(
	p *protocol.PublishDiagnosticsParams,
) {
	h.mu.Lock()
	defer h.mu.Unlock()

	f, ok := h.getFile(p.URI.SpanURI())
	if !ok {
		log.Warnf("lspEditorHandler: Received diagnostic for an unknown file: %#v", p.URI.SpanURI())
		return
	}

	if p.Version != f.version {
		log.Debugf("lspEditorHandler: Received diagnostic for"+
			"outdated version of file '%s': %#v", f.name, p.Version)
		return
	}

	locs := parseDiagnostics(f, p.Diagnostics, h.diagnosticAttr)
	err := h.ed.SetLocationList(f.handler, h.diagnosticListID, editor.LocationSlice(locs))
	if err != nil {
		log.Errorf("lspEditorHandler.SetLocationList(%s): %v", f.name, err)
		return
	}
}

func (h *lspEditorHandler) Handle(ev editor.Event) (exit bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	log.Tracef("lspEditorHandler.Handle(%#v)", ev)

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
	return
}

func (h *lspEditorHandler) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()

	return h.server.Shutdown(ctx)
}
