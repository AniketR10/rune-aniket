package main

import (
	"context"
	"sync"
	"testing"

	"github.com/ernestrc/golang-internal-tools/lsp/protocol"
	"github.com/ernestrc/golang-internal-tools/span"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	textapi "unstable.build/go-tui/api/text"
	textapitest "unstable.build/go-tui/api/text/test"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/config"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
	texttest "unstable.build/go-tui/text/test"
)

var (
	filename1         workspaceapi.URI
	nonConfiguredFile workspaceapi.URI
	filecontent1      = `package me.drton.jmavsim;
public class	Rotor {
     sta  mtyp;


  myClass;

`
	tokenData1         = []uint32{2, 5, 3, 0, 3, 0, 5, 4, 1, 0, 3, 2, 7, 2, 0}
	expectedLocations1 = []textapi.Location{
		{
			From: term.Coordinates{Y: 2, X: 5},
			To:   term.Coordinates{Y: 2, X: 8},
		},
		{
			From: term.Coordinates{Y: 2, X: 10},
			To:   term.Coordinates{Y: 2, X: 14},
		},
		{
			From: term.Coordinates{Y: 5, X: 2},
			To:   term.Coordinates{Y: 5, X: 9},
		},
	}
)

func init() {
	var err error
	filename1, err = workspaceapi.ParseURI("file:///wa_tup.go")
	if err != nil {
		panic(err)
	}
	nonConfiguredFile, err = workspaceapi.ParseURI("file:///server.MISSING")
	if err != nil {
		panic(err)
	}
}

func makePluginConfig() config.Config {
	m := make(map[string]interface{})
	return config.MapConfig(m)
}

func newTestLspHandler(
	ctrl *gomock.Controller, ed textapi.Editor,
	cfg config.Config, server protocol.Server,
) *lspEditorHandler {
	ret := new(lspEditorHandler)
	ret.ed = ed
	ret.files = make(map[string]*file)
	ret.servers = map[string]execServer{".go": {langID: ".go", srv: server}}
	ret.semanticTokensListID = defaultSemanticTokensListID
	ret.enableSemanticTokens = map[string]bool{
		".go": true,
	}
	ret.diagnosticListID = defaultDiagnosticListID
	ret.semanticTypesAttr = defaultSemanticTypeAttr
	ret.diagnosticAttr = defaultDiagnosticAttr
	ret.evChan = make(chan textapi.Event)
	ret.tabspaces = 4
	go ret.handleEvents(ret.evChan)
	return ret
}

func expectDidOpen(t *testing.T, mock *MockServer, file workspaceapi.URI, content string) {
	expected := &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        protocol.URIFromSpanURI(span.URIFromURI(file.String())),
			LanguageID: ".go",
			Version:    1,
			Text:       content,
		},
	}
	mock.EXPECT().DidOpen(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, p *protocol.DidOpenTextDocumentParams) error {
			assert.Equal(t, expected, p)
			return nil
		}).Times(1)
}

func expectDidChange(
	t *testing.T, mock *MockServer, file workspaceapi.URI, version int32,
	expectedEvents []protocol.TextDocumentContentChangeEvent,
) {
	expected := &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			Version: version,
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{
				URI: protocol.URIFromSpanURI(span.URIFromURI(file.String())),
			},
		},
		ContentChanges: expectedEvents,
	}
	mock.EXPECT().DidChange(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, p *protocol.DidChangeTextDocumentParams) error {
			assert.Equal(t, expected, p)
			return nil
		}).Times(1)
}

func expectSemanticTokens(t *testing.T, server *MockServer, returnData []uint32) {
	server.EXPECT().SemanticTokensFull(gomock.Any(), gomock.Any()).
		Return(&protocol.SemanticTokens{Data: returnData}, nil).
		Times(1)
}

func assertEqualLocations(t *testing.T, loc, expected text.LocationList) {
	var locations, expectedLocations []textapi.Location
	for n, ok := loc.Current(); ok; n, ok = loc.Next() {
		locations = append(locations, n)
	}
	for n, ok := expected.Current(); ok; n, ok = expected.Next() {
		expectedLocations = append(expectedLocations, n)
	}
	assert.EqualValues(t, expectedLocations, locations)
}

func expectLocationList(
	t *testing.T, ed *textapitest.MockEditor, expectedID string, expectedLocations []textapi.Location,
	wg *sync.WaitGroup,
) {
	ed.EXPECT().SetLocationList(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(h text.Handler, pri textapi.LocationPriority, id string, loc text.LocationList) error {
			defer wg.Done()
			assertEqualLocations(t, loc, text.LocationSlice(expectedLocations))
			assert.Equal(t, expectedID, id)
			return nil
		}).Times(1)
}

func expectAnyLocationList(
	t *testing.T, ed *textapitest.MockEditor, expectedID string, wg *sync.WaitGroup,
) {
	ed.EXPECT().SetLocationList(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(h text.Handler, pri textapi.LocationPriority, id string, loc text.LocationList) error {
			defer wg.Done()
			assert.Equal(t, expectedID, id)
			return nil
		}).Times(1)
}

func dispatchOpen(
	t *testing.T, h *lspEditorHandler, server *MockServer, ed *textapitest.MockEditor,
	uri workspaceapi.URI, content string, tokenData []uint32, expectedListID string,
	expectedLocations []textapi.Location,
) {
	var wg sync.WaitGroup
	evOpen := textapi.Event{
		Type:     textapi.EventTypeOpen,
		URI:      uri,
		Content:  content,
		Resource: texttest.NewTestHandler(),
	}

	expectDidOpen(t, server, uri, content+"\n")
	expectSemanticTokens(t, server, tokenData)
	expectLocationList(t, ed, expectedListID, expectedLocations, &wg)
	wg.Add(1)
	assert.False(t, h.Handle(context.Background(), evOpen))
	wg.Wait()
}

func dispatchFlush(
	t *testing.T, h *lspEditorHandler, server *MockServer, ed *textapitest.MockEditor,
	uri workspaceapi.URI, content string, version int32, tokenData []uint32,
	expectedListID string, expectedLocations []textapi.Location,
) {
	var wg sync.WaitGroup
	ev := textapi.Event{
		Type:     textapi.EventTypeFlush,
		URI:      uri,
		Content:  content,
		Resource: texttest.NewTestHandler(),
	}
	changes := []protocol.TextDocumentContentChangeEvent{{Text: content + "\n"}}

	expectDidChange(t, server, uri, version, changes)
	expectSemanticTokens(t, server, tokenData)
	expectLocationList(t, ed, expectedListID, expectedLocations, &wg)
	wg.Add(1)
	assert.False(t, h.Handle(context.Background(), ev))
	wg.Wait()
}

func TestLspHandlerHandleOpen(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ed := textapitest.NewMockEditor(ctrl)
		cfg := makePluginConfig()
		server := NewMockServer(ctrl)

		h := newTestLspHandler(ctrl, ed, cfg, server)
		dispatchOpen(t, h, server, ed, filename1, filecontent1,
			tokenData1, h.semanticTokensListID, expectedLocations1)
	})
	t.Run("file with missing configuration does not panic", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ed := textapitest.NewMockEditor(ctrl)
		cfg := makePluginConfig()
		server := NewMockServer(ctrl)

		h := newTestLspHandler(ctrl, ed, cfg, server)
		evOpen := textapi.Event{
			Type:     textapi.EventTypeOpen,
			URI:      nonConfiguredFile,
			Content:  "",
			Resource: texttest.NewTestHandler(),
		}
		assert.False(t, h.Handle(context.Background(), evOpen))
	})
}

func TestLspHandlerHandleFlush(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ed := textapitest.NewMockEditor(ctrl)
	cfg := makePluginConfig()
	server := NewMockServer(ctrl)

	h := newTestLspHandler(ctrl, ed, cfg, server)

	dispatchOpen(t, h, server, ed, filename1, filecontent1,
		tokenData1, h.semanticTokensListID, expectedLocations1)
	dispatchFlush(t, h, server, ed, filename1, filecontent1, 2,
		tokenData1, h.semanticTokensListID, expectedLocations1)
}

func TestLspHandlerHandleInsertDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var wg sync.WaitGroup
	ed := textapitest.NewMockEditor(ctrl)
	cfg := makePluginConfig()
	server := NewMockServer(ctrl)

	h := newTestLspHandler(ctrl, ed, cfg, server)

	dispatchOpen(t, h, server, ed, filename1, filecontent1,
		tokenData1, h.semanticTokensListID, expectedLocations1)

	buf := cell.NewBuffer()
	buf.WriteString(filecontent1)
	buf.Subscribe(text.CellSubscriber(filename1, texttest.NewTestHandler(),
		text.FuncEventHandler(func(ctx context.Context, ev textapi.Event) bool {
			assert.False(t, h.Handle(ctx, ev))
			return false
		})))

	// Insert
	insertStr := "\tmyClassVar\n"
	expectedEvents := []protocol.TextDocumentContentChangeEvent{{
		Range: &protocol.Range{
			Start: protocol.Position{Line: 5, Character: 9},
			End:   protocol.Position{Line: 5, Character: 9},
		},
		Text: insertStr,
	}}
	expectDidChange(t, server, filename1, 2, expectedEvents)
	returnData := []uint32{2, 5, 3, 0, 3, 0, 5, 4, 1, 0, 3, 2, 7, 2, 0, 0, 8, 10, 2, 0}
	expectSemanticTokens(t, server, returnData)

	newLocations := append(expectedLocations1, textapi.Location{
		From: term.Coordinates{Y: 5, X: 13},
		To:   term.Coordinates{Y: 5, X: 23},
	})
	expectLocationList(t, ed, defaultSemanticTokensListID, newLocations, &wg)

	wg.Add(1)
	from, until := buf.InsertString(term.Coordinates{X: 9, Y: 5}, insertStr)
	wg.Wait()

	// Delete
	expectedEvents = []protocol.TextDocumentContentChangeEvent{{
		Range: &protocol.Range{
			Start: protocol.Position{Line: 5, Character: 9},
			End:   protocol.Position{Line: 6, Character: 0},
		},
		Text: "",
	}}
	expectDidChange(t, server, filename1, 3, expectedEvents)
	expectSemanticTokens(t, server, tokenData1)
	expectLocationList(t, ed, defaultSemanticTokensListID, expectedLocations1, &wg)
	wg.Add(1)
	buf.Delete(from, until)
	wg.Wait()

	// Delete 2
	expectedEvents = []protocol.TextDocumentContentChangeEvent{{
		Range: &protocol.Range{
			Start: protocol.Position{Line: 6, Character: 0},
			End:   protocol.Position{Line: 7, Character: 0},
		},
		Text: "",
	}}
	expectDidChange(t, server, filename1, 4, expectedEvents)
	expectSemanticTokens(t, server, tokenData1)
	expectLocationList(t, ed, defaultSemanticTokensListID, expectedLocations1, &wg)
	wg.Add(1)
	buf.Delete(term.Coordinates{Y: 6}, term.Coordinates{Y: 7})
	wg.Wait()

	// Delete 3
	expectedEvents = []protocol.TextDocumentContentChangeEvent{{
		Range: &protocol.Range{
			Start: protocol.Position{Line: 0, Character: 25},
			End:   protocol.Position{Line: 1, Character: 20},
		},
		Text: "",
	}}
	expectDidChange(t, server, filename1, 5, expectedEvents)
	expectSemanticTokens(t, server, tokenData1)
	expectAnyLocationList(t, ed, defaultSemanticTokensListID, &wg)
	wg.Add(1)
	buf.DeleteRow(1)
	wg.Wait()
}
