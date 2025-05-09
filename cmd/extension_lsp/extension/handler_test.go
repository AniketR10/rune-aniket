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
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/golang-internal-tools/lsp/protocol"
	"github.com/unstablebuild/golang-internal-tools/span"
	"go.uber.org/mock/gomock"
	"unstable.build/go-tui/api/config"
	"unstable.build/go-tui/api/textapi"
	textapitest "unstable.build/go-tui/api/textapi/texttest"
	"unstable.build/go-tui/api/workspaceapi"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/text/texttest"
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

func makeExtensionConfig() config.Config {
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
		cfg := makeExtensionConfig()
		server := NewMockServer(ctrl)

		h := newTestLspHandler(ctrl, ed, cfg, server)
		dispatchOpen(t, h, server, ed, filename1, filecontent1,
			tokenData1, h.semanticTokensListID, expectedLocations1)
	})
	t.Run("file with missing configuration does not panic", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ed := textapitest.NewMockEditor(ctrl)
		cfg := makeExtensionConfig()
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
	cfg := makeExtensionConfig()
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
	cfg := makeExtensionConfig()
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

	newLocations := []textapi.Location{
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
			To:   term.Coordinates{Y: 5, X: 12},
		},
		{
			From: term.Coordinates{Y: 5, X: 13},
			To:   term.Coordinates{Y: 5, X: 23},
		},
	}
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
