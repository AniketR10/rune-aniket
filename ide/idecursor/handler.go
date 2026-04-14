// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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

package idecursor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/retry"
	"unstable.build/go-tui/handler/locationpicker"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/workspace"
)

var retryStrategy = retry.DefaultStrategy

// WithHistory installs persisted cursor history on the given editor.
func WithHistory(
	ed text.Editor,
	storage storageapi.Service,
	opener browserapi.ResourceOpener,
	wm browserapi.WindowManager,
	fs workspaceapi.FileSystem,
	parser syntaxapi.Parser,
	workspaceManager workspace.WorkspaceManager,
	workspaceURI workspaceapi.URI,
	scheduleNextTick func(func()) bool,
) (io.Closer, error) {
	h := &handler{
		editor:           ed,
		store:            storage,
		opener:           opener,
		wm:               wm,
		fs:               fs,
		parser:           parser,
		workspaceManager: workspaceManager,
		workspaceURI:     workspaceURI,
		scheduleNextTick: scheduleNextTick,
		docID:            documentID(workspaceURI),
		doc:              newHistoryDocument(workspaceURI),
	}
	if err := h.load(context.Background()); err != nil {
		return nil, err
	}
	if err := ed.SubscribeEvents([]textapi.EventType{textapi.EventTypeOpen, textapi.EventTypeCursor}, h); err != nil {
		return nil, err
	}
	if err := ed.SubscribeCommand(manual(), h); err != nil {
		_, _ = ed.UnsubscribeEvents(h)
		return nil, err
	}
	return historyCloser{}, nil
}

type historyCloser struct{}

func (h historyCloser) Close() error {
	return nil
}

type handler struct {
	editor           text.Editor
	store            storageapi.Service
	opener           browserapi.ResourceOpener
	wm               browserapi.WindowManager
	fs               workspaceapi.FileSystem
	parser           syntaxapi.Parser
	workspaceManager workspace.WorkspaceManager
	workspaceURI     workspaceapi.URI
	scheduleNextTick func(func()) bool

	docID       string
	doc         historyDocument
	suppress    bool
	lastCursor  location
	hasLast     bool
	lastWasOpen bool
}

func manual() textapi.CommandManual {
	return textapi.CommandManual{
		Name:     commandName,
		Summary:  "Navigate persisted cursor history across resources",
		Synopsis: "(prev|next|jump) [location]",
		Commands: []textapi.CommandManual{
			{Name: "prev", Summary: "Jump to the previous cursor history entry", Synopsis: "[location]"},
			{Name: "next", Summary: "Jump to the next cursor history entry", Synopsis: "[location]"},
			{Name: "jump", Summary: "Open a picker for cursor history entries", Synopsis: "[location]"},
		},
	}
}

func (h *handler) Handle(ctx context.Context, ev textapi.Event) bool {
	if h.suppress {
		return false
	}
	switch ev.Type {
	case textapi.EventTypeOpen, textapi.EventTypeCursor:
	default:
		return false
	}
	next := location{
		URI:          ev.URI.String(),
		Cursor:       ev.From,
		WindowCursor: ev.Start,
		Timestamp:    time.Now(),
	}
	if !h.hasLast {
		h.lastCursor = next
		h.hasLast = true
		h.lastWasOpen = ev.Type == textapi.EventTypeOpen
		return false
	}
	if ev.Type == textapi.EventTypeOpen && h.lastCursor.URI == next.URI {
		if sameLocation(h.lastCursor, next) {
			h.lastWasOpen = true
		}
		return false
	}
	if ev.Type == textapi.EventTypeCursor && h.lastWasOpen && h.lastCursor.URI == next.URI {
		h.amendLastOpen(ctx, next)
		return false
	}
	if !shouldRecord(h.lastCursor, next) {
		h.lastCursor = next
		h.lastWasOpen = ev.Type == textapi.EventTypeOpen
		return false
	}
	from := h.lastCursor
	h.lastCursor = next
	h.lastWasOpen = ev.Type == textapi.EventTypeOpen
	if !h.doc.recordJump(from, next) {
		return false
	}
	err := h.persistState(ctx)
	if err != nil {
		logrus.Errorf("could not persist state: %v", err)
	}
	return false
}

func (h *handler) amendLastOpen(ctx context.Context, loc location) {
	amended := false
	if current, ok := h.doc.current(); ok && sameLocation(current, h.lastCursor) {
		h.doc.Entries[h.doc.Index] = loc
		amended = true
	}
	h.lastCursor = loc
	h.lastWasOpen = false
	if !amended {
		return
	}
	if err := h.persistState(ctx); err != nil {
		logrus.Errorf("could not persist state: %v", err)
	}
}

func (h *handler) HandleCommand(ctx context.Context, cmd textapi.Command) error {
	if cmd.Name != commandName {
		return fmt.Errorf("extraneous command")
	}
	if len(cmd.Args) == 0 {
		return fmt.Errorf("missing cursorhistory subcommand")
	}
	sub := cmd.Args[0]
	if len(cmd.Args) > 1 {
		idx := h.indexForDisplay(cmd.Args[1], sub)
		if idx < 0 {
			return fmt.Errorf("cursor history entry not found")
		}
		_, err := h.jumpToIndex(ctx, idx)
		return err
	}
	switch sub {
	case "prev":
		loc, err := h.doc.prev()
		if err != nil {
			return err
		}
		return h.navigate(ctx, loc)
	case "next":
		loc, err := h.doc.next()
		if err != nil {
			return err
		}
		return h.navigate(ctx, loc)
	case "jump":
		entries := h.pickerEntries(h.displayOrder(sub))
		picker := locationpicker.New(entries, h.wm, h.fs, h.scheduleNextTick, h.parser, locationpicker.DefaultConfig(), nil)
		picker.SetOnSelect(func(idx int) {
			_, _ = h.jumpToIndex(context.Background(), h.displayOrder(sub)[idx])
		})
		win, err := h.wm.Floating(picker, browserapi.FloatingConfig{Alignment: component.AlignmentCentered})
		if err != nil {
			return err
		}
		picker.SetWindow(win)
		return nil
	default:
		return fmt.Errorf("unknown cursorhistory subcommand %q", sub)
	}
}

func (h *handler) Complete(ctx context.Context, cmd textapi.Command) (
	iterator.Iterator[string], string, error,
) {
	if cmd.Name != commandName {
		return iterator.Empty[string](), "", nil
	}
	// Top-level: return empty so the framework auto-appends subcommands
	// from the CommandManual.Commands entries.
	if len(cmd.Args) == 0 {
		return iterator.Empty[string](), "", nil
	}
	sub := cmd.Args[0]
	switch sub {
	case "prev", "next", "jump":
		return iterator.FromSlice(h.completionDisplays(sub)), "", nil
	default:
		return iterator.Empty[string](), "", nil
	}
}

func (h *handler) load(ctx context.Context) error {
	err := h.store.Get(ctx, h.docID, &h.doc)
	if errors.Is(err, storageapi.ErrNotFound) {
		return h.store.Create(ctx, h.docID, &h.doc)
	}
	return err
}

func (h *handler) persistState(ctx context.Context) error {
	entries := append([]location(nil), h.doc.Entries...)
	index := h.doc.Index
	return storageapi.ConsistentUpdate(ctx, h.store, h.docID, &h.doc, retryStrategy,
		func() ([]storageapi.Update, []storageapi.Precondition) {
			return []storageapi.Update{
				{FieldPath: []string{"Entries"}, Value: entries},
				{FieldPath: []string{"Index"}, Value: index},
				{FieldPath: []string{"Version"}, Value: h.doc.Version + 1},
			}, []storageapi.Precondition{{FieldPath: []string{"Version"}, Value: h.doc.Version}}
		})
}

func (h *handler) persistIndex(ctx context.Context) error {
	index := h.doc.Index
	return storageapi.ConsistentUpdate(ctx, h.store, h.docID, &h.doc, retryStrategy,
		func() ([]storageapi.Update, []storageapi.Precondition) {
			return []storageapi.Update{
				{FieldPath: []string{"Index"}, Value: index},
				{FieldPath: []string{"Version"}, Value: h.doc.Version + 1},
			}, []storageapi.Precondition{{FieldPath: []string{"Version"}, Value: h.doc.Version}}
		})
}

func (h *handler) navigate(ctx context.Context, loc location) error {
	h.suppress = true
	defer func() { h.suppress = false }()
	uri, err := workspaceapi.ParseURI(loc.URI)
	if err != nil {
		return err
	}
	handler, err := h.opener.Open(uri)
	if err != nil {
		return err
	}
	win, err := h.wm.Focus()
	if err != nil {
		return err
	}
	if err := h.wm.SetWindowContent(win, handler); err != nil && !errors.Is(err, browserapi.ErrTabNotFree) {
		return err
	}
	eh, err := h.editor.Editor(uri)
	if err != nil {
		return err
	}
	if ok := eh.SetCursorAtScroll(loc.Cursor); !ok && eh.CursorAtScroll() != loc.Cursor {
		return fmt.Errorf("could not set cursor at (%d, %d)", loc.Cursor.X, loc.Cursor.Y)
	}
	_ = h.persistIndex(ctx)
	h.lastCursor = loc
	h.hasLast = true
	return nil
}

func (h *handler) jumpToIndex(ctx context.Context, idx int) (location, error) {
	loc, err := h.doc.jump(idx)
	if err != nil {
		return location{}, err
	}
	return loc, h.navigate(ctx, loc)
}

func (h *handler) displayOrder(sub string) []int {
	if len(h.doc.Entries) == 0 {
		return nil
	}
	ret := make([]int, 0, len(h.doc.Entries))
	switch sub {
	case "prev":
		for i := h.doc.Index - 1; i >= 0; i-- {
			ret = append(ret, i)
		}
		for i := len(h.doc.Entries) - 1; i > h.doc.Index; i-- {
			ret = append(ret, i)
		}
	case "next":
		for i := h.doc.Index + 1; i < len(h.doc.Entries); i++ {
			ret = append(ret, i)
		}
		for i := 0; i < h.doc.Index; i++ {
			ret = append(ret, i)
		}
	default:
		for i := len(h.doc.Entries) - 1; i >= 0; i-- {
			ret = append(ret, i)
		}
	}
	return ret
}

func (h *handler) displayFor(loc location) string {
	uri, err := workspaceapi.ParseURI(loc.URI)
	if err != nil {
		return fmt.Sprintf("%s:%d:%d", loc.URI, loc.Cursor.Y+1, loc.Cursor.X+1)
	}
	path := workspaceapi.RelPath(h.workspaceURI, uri)
	if path == "" || path == "." || path == uri.String() {
		path = uri.Path()
	}
	return fmt.Sprintf("%s:%d:%d", path, loc.Cursor.Y+1, loc.Cursor.X+1)
}

func (h *handler) indexForDisplay(display, sub string) int {
	for _, idx := range h.displayOrder(sub) {
		if h.displayFor(h.doc.Entries[idx]) == display {
			return idx
		}
	}
	return -1
}

func (h *handler) completionDisplays(sub string) []string {
	order := h.displayOrder(sub)
	ret := make([]string, 0, len(order))
	for _, idx := range order {
		ret = append(ret, h.displayFor(h.doc.Entries[idx]))
	}
	return ret
}

func (h *handler) pickerEntries(order []int) []locationpicker.Entry {
	ret := make([]locationpicker.Entry, 0, len(order))
	for _, idx := range order {
		loc := h.doc.Entries[idx]
		uri, err := workspaceapi.ParseURI(loc.URI)
		if err != nil {
			continue
		}
		ret = append(ret, locationpicker.Entry{
			URI: uri,
			Range: semanticapi.Range{
				Start: semanticapi.Position{Line: uint32(loc.Cursor.Y), Character: uint32(loc.Cursor.X)},
				End:   semanticapi.Position{Line: uint32(loc.Cursor.Y), Character: uint32(loc.Cursor.X + 1)},
			},
			Display: h.displayFor(loc),
		})
	}
	return ret
}
