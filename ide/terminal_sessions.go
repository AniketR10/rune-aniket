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

package ide

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/ernestrc/go-multierror"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/term/vte"
	"unstable.build/go-tui/term/vte/vtereservoir"
)

const (
	terminalSessionDocumentKind   = "terminal-session"
	terminalSessionDocumentPrefix = "terminal-sessions:"
	terminalSessionAutoNamePrefix = "__open-terminal-session-"
)

type terminalSessionDocument struct {
	Kind     string
	Name     string
	Snapshot vte.Snapshot
	Window   terminalSessionWindowSnapshot
}

type terminalSessionWindowSnapshot struct {
	Tab      bool
	Visible  bool
	Focus    bool
	WindowID uint64
}

func normalizeTerminalSessionName(args []string) (string, error) {
	name := strings.TrimSpace(strings.Join(args, " "))
	if name == "" {
		return "", errors.New("expected terminal session name")
	}
	return name, nil
}

func terminalSessionURI(name string) (workspaceapi.URI, error) {
	return workspaceapi.ParseURI("terminalsession:///" + url.PathEscape(name))
}

func terminalSessionDocumentID(name string) string {
	return terminalSessionDocumentPrefix + url.QueryEscape(name)
}

func terminalSessionAutoPrefix(workspaceURI workspaceapi.URI) string {
	return terminalSessionAutoNamePrefix + url.QueryEscape(workspaceURI.String()) + "-"
}

func terminalSessionAutoName(workspaceURI workspaceapi.URI, idx int) string {
	return fmt.Sprintf("%s%06d", terminalSessionAutoPrefix(workspaceURI), idx)
}

func isTerminalSessionAutoName(name string) bool {
	return strings.HasPrefix(name, terminalSessionAutoNamePrefix)
}

func (e *ex) terminalInFocus() (vtereservoir.VTE, bool) {
	content, err := e.invokeWindow().Content()
	if err != nil {
		return nil, false
	}

	switch h := content.(type) {
	case vtereservoir.VTE:
		return h, true
	case *browser.Tab:
		if s, ok := h.Handler().(vtereservoir.VTE); ok {
			return s, true
		}
	}

	return nil, false
}

func (e *ex) nextTerminalSessionName(ctx context.Context, h vtereservoir.VTE) (string, error) {
	base := "terminal"
	uri := h.URI()
	if uri.Name() != "" {
		base = uri.Name()
	} else if uri.Path() != "" {
		base = strings.Trim(strings.ReplaceAll(uri.Path(), "/", "-"), "-")
	}
	if base == "" {
		base = "terminal"
	}

	for i := 0; ; i++ {
		name := base + "-saved"
		if i != 0 {
			name = fmt.Sprintf("%s-saved-%d", base, i)
		}
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		var doc terminalSessionDocument
		err := e.storage.Get(ctx, terminalSessionDocumentID(name), &doc)
		cancel()
		if errors.Is(err, storageapi.ErrNotFound) {
			return name, nil
		}
		if err != nil {
			return "", fmt.Errorf("check terminal session name %q: %w", name, err)
		}
	}
}

func (e *ex) saveTerminalSession(ctx context.Context, name string, h vtereservoir.VTE) error {
	snapshot, err := h.Snapshot()
	if err != nil {
		return fmt.Errorf("snapshot terminal: %w", err)
	}
	if snapshot.Title == "" {
		snapshot.Title = name
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	err = e.storage.Set(ctx, terminalSessionDocumentID(name), terminalSessionDocument{
		Name:     name,
		Snapshot: snapshot,
	})
	if err != nil {
		return fmt.Errorf("save terminal session %q: %w", name, err)
	}

	_, _ = e.notifications.Notify(browserapi.LevelSuccess,
		"terminal session %q saved", name)
	return nil
}

func (e *ex) terminalsave(ctx context.Context, args ...string) error {
	name, err := normalizeTerminalSessionName(args)
	if err != nil {
		return err
	}
	h, ok := e.terminalInFocus()
	if !ok {
		return errors.New("not a terminal")
	}
	return e.saveTerminalSession(ctx, name, h)
}

func (e *ex) terminalresume(ctx context.Context, args ...string) error {
	name, err := normalizeTerminalSessionName(args)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	var doc terminalSessionDocument
	if err := e.storage.Get(ctx, terminalSessionDocumentID(name), &doc); err != nil {
		return fmt.Errorf("resume terminal session %q: %w", name, err)
	}
	if doc.Name == "" {
		doc.Name = name
	}

	_, err = e.restoreTerminalSessionTab(doc, e.invokeWindow())
	return err
}

func (e *ex) newTerminalSessionHandler(
	name string, snapshot vte.Snapshot,
) (vtereservoir.VTE, error) {
	h, err := e.newEmulatorHandler(nil)
	if err != nil {
		return nil, err
	}
	if err := h.RestoreFromSnapshot(snapshot); err != nil {
		_ = h.Close()
		return nil, fmt.Errorf("restore terminal session %q: %w", name, err)
	}
	return h, nil
}

func (e *ex) restoreTerminalSessionTab(
	doc terminalSessionDocument, win browser.Window,
) (*browser.Tab, error) {
	if doc.Name == "" {
		return nil, errors.New("terminal session document missing name")
	}

	uri, err := terminalSessionURI(doc.Name)
	if err != nil {
		return nil, err
	}
	if existing, ok := e.comp.Resource(uri); ok {
		if win != nil {
			if err := win.SetContent(existing); err != nil {
				return nil, err
			}
		}
		tab, ok := existing.(*browser.Tab)
		if !ok {
			return nil, fmt.Errorf("terminal session %q resource is not a tab", doc.Name)
		}
		return tab, nil
	}

	h, err := e.newTerminalSessionHandler(doc.Name, doc.Snapshot)
	if err != nil {
		return nil, err
	}
	title := terminalSessionTabTitle(doc)
	t, err := e.comp.Tab(uri, e.config.Icons.Terminal, title, h)
	if err != nil {
		_ = h.Close()
		return nil, fmt.Errorf("wm.Tab: %w", err)
	}
	tab := t.(*browser.Tab)
	tab.Subscribe((*tabSubscriber)(e))
	if win != nil {
		if err := win.SetContent(t); err != nil {
			_ = t.Close()
			return nil, err
		}
		if err := h.RestoreFromSnapshot(doc.Snapshot); err != nil {
			return nil, err
		}
	}
	return tab, nil
}

func (e *ex) newTerminalSessionTab(
	doc terminalSessionDocument, h vtereservoir.VTE,
) (*browser.Tab, error) {
	uri, err := terminalSessionURI(doc.Name)
	if err != nil {
		return nil, err
	}
	title := terminalSessionTabTitle(doc)
	t, err := e.comp.Tab(uri, e.config.Icons.Terminal, title, h)
	if err != nil {
		_ = h.Close()
		return nil, fmt.Errorf("wm.Tab: %w", err)
	}
	tab := t.(*browser.Tab)
	tab.Subscribe((*tabSubscriber)(e))
	return tab, nil
}

func terminalSessionTabTitle(doc terminalSessionDocument) string {
	if !isTerminalSessionAutoName(doc.Name) {
		return doc.Name
	}
	if doc.Snapshot.Title != "" {
		return doc.Snapshot.Title
	}
	return "terminal"
}

func (e *ex) saveOpenTerminalSessions(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	if err := e.clearOpenTerminalSessions(ctx); err != nil {
		return err
	}

	ret := new(multierror.Error)
	var idx int
	visibleTabs, visibleTerminals := e.openTerminalSessionWindows()
	for _, tab := range e.comp.Tabs() {
		terminal, ok := tab.Handler().(vtereservoir.VTE)
		if !ok {
			continue
		}
		name := terminalSessionAutoName(e.workspaceURI, idx)
		idx++
		window := visibleTabs[tab]
		if !window.Visible {
			window.Tab = true
		}
		err := e.saveOpenTerminalSession(ctx, name, terminal, tab, window)
		ret = multierror.Append(ret, err)
	}
	for _, win := range visibleTerminals {
		name := terminalSessionAutoName(e.workspaceURI, idx)
		idx++
		err := e.saveOpenTerminalSession(ctx, name, win.terminal, nil, win.window)
		ret = multierror.Append(ret, err)
	}
	return ret.ErrorOrNil()
}

type terminalSessionWindowTerminal struct {
	terminal vtereservoir.VTE
	window   terminalSessionWindowSnapshot
}

func (e *ex) openTerminalSessionWindows() (
	map[*browser.Tab]terminalSessionWindowSnapshot, []terminalSessionWindowTerminal,
) {
	tabs := make(map[*browser.Tab]terminalSessionWindowSnapshot)
	var terminals []terminalSessionWindowTerminal
	e.comp.Browser().IterateWindows(func(win browser.Window) {
		if win.IsFloating() {
			return
		}
		content, err := win.Content()
		if err != nil {
			return
		}
		focus, _ := win.Focus()
		window := terminalSessionWindowSnapshot{
			Visible:  true,
			Focus:    focus,
			WindowID: win.WindowID(),
		}
		if tab, ok := content.(*browser.Tab); ok {
			if _, ok := tab.Handler().(vtereservoir.VTE); ok {
				window.Tab = true
				tabs[tab] = window
			}
			return
		}
		terminal, ok := content.(vtereservoir.VTE)
		if !ok {
			return
		}
		terminals = append(terminals, terminalSessionWindowTerminal{
			terminal: terminal,
			window:   window,
		})
	})
	return tabs, terminals
}

func (e *ex) saveOpenTerminalSession(
	ctx context.Context,
	name string,
	terminal vtereservoir.VTE,
	tab *browser.Tab,
	window terminalSessionWindowSnapshot,
) error {
	snapshot, err := terminal.Snapshot()
	if err != nil {
		return fmt.Errorf("snapshot open terminal: %w", err)
	}
	if snapshot.Title == "" {
		snapshot.Title = e.openTerminalSessionTitle(tab, terminal, name)
	}
	err = e.storage.Set(ctx, terminalSessionDocumentID(name), terminalSessionDocument{
		Name:     name,
		Snapshot: snapshot,
		Window:   window,
	})
	if err != nil {
		return fmt.Errorf("save open terminal session %q: %w", name, err)
	}
	return nil
}

func (e *ex) openTerminalSessionTitle(
	tab *browser.Tab,
	terminal vtereservoir.VTE,
	fallback string,
) string {
	if tab != nil {
		name, _, ok := e.comp.Browser().TabName(tab.URI())
		if ok && name != "" {
			return name
		}
	}
	if title := terminal.Title(); title != "" {
		return title
	}
	return fallback
}

func (e *ex) hasOpenTerminalSessions(ctx context.Context) (bool, error) {
	docs, err := e.openTerminalSessionDocuments(ctx)
	return len(docs) != 0, err
}

func (e *ex) restoreOpenTerminalSessions(
	ctx context.Context,
	windows map[uint64]browser.Window,
) error {
	docs, err := e.openTerminalSessionDocuments(ctx)
	if err != nil {
		return err
	}

	ret := new(multierror.Error)
	if len(docs) == 0 {
		return nil
	}
	restored := make(map[string]bool)
	var focus browser.Window
	for _, doc := range docs {
		if len(windows) == 0 || !doc.Window.Visible || doc.Window.WindowID == 0 {
			continue
		}
		win, ok := windows[doc.Window.WindowID]
		if !ok {
			continue
		}
		content, err := e.newTerminalSessionContent(doc)
		if err != nil {
			ret = multierror.Append(ret,
				fmt.Errorf("restore open terminal session %q: %w", doc.Name, err))
			continue
		}
		if err := win.SetContent(content); err != nil {
			_ = content.Close()
			ret = multierror.Append(ret,
				fmt.Errorf("restore open terminal session %q: %w", doc.Name, err))
			continue
		}
		restored[doc.Name] = true
		if doc.Window.Focus {
			focus = win
		}
	}
	if focus != nil {
		e.comp.Browser().SetFocus(focus)
	}

	visibleDocs := make([]terminalSessionDocument, 0, len(docs))
	for _, doc := range docs {
		if restored[doc.Name] || !doc.Window.Visible {
			continue
		}
		visibleDocs = append(visibleDocs, doc)
	}
	ret = multierror.Append(ret, e.restoreOpenTerminalSessionsSequential(visibleDocs))
	ret = multierror.Append(ret, e.restoreHiddenOpenTerminalSessionTabs(docs))
	return ret.ErrorOrNil()
}

func (e *ex) restoreHiddenOpenTerminalSessionTabs(docs []terminalSessionDocument) error {
	ret := new(multierror.Error)
	for _, doc := range docs {
		if doc.Window.Visible || !doc.Window.Tab {
			continue
		}
		if _, err := e.newTerminalSessionContent(doc); err != nil {
			ret = multierror.Append(ret,
				fmt.Errorf("restore open terminal session %q: %w", doc.Name, err))
		}
	}
	return ret.ErrorOrNil()
}

func (e *ex) restoreOpenTerminalSessionsSequential(docs []terminalSessionDocument) error {
	ret := new(multierror.Error)
	var focus browser.Window
	for _, doc := range docs {
		win, err := e.restoreOpenTerminalSessionSequential(doc, focus)
		if err != nil {
			ret = multierror.Append(ret,
				fmt.Errorf("restore open terminal session %q: %w", doc.Name, err))
			continue
		}
		if win != nil {
			if doc.Window.Focus {
				focus = win
			}
		}
	}
	if focus != nil {
		e.comp.Browser().SetFocus(focus)
	}
	return ret.ErrorOrNil()
}

func (e *ex) restoreOpenTerminalSessionSequential(
	doc terminalSessionDocument,
	prev browser.Window,
) (browser.Window, error) {
	content, err := e.newTerminalSessionContent(doc)
	if err != nil {
		return nil, err
	}
	if !doc.Window.Visible {
		return nil, nil
	}
	if prev == nil {
		win := e.invokeWindow()
		if err := win.SetContent(content); err != nil {
			return nil, err
		}
		return win, nil
	}
	win, err := e.comp.Split(browserapi.OrientationDefault, prev, content)
	if err != nil {
		return nil, err
	}
	return win, nil
}

func (e *ex) newTerminalSessionContent(
	doc terminalSessionDocument,
) (browserapi.Handler, error) {
	h, err := e.newTerminalSessionHandler(doc.Name, doc.Snapshot)
	if err != nil {
		return nil, err
	}
	if !doc.Window.Tab {
		return h, nil
	}
	tab, err := e.newTerminalSessionTab(doc, h)
	if err != nil {
		return nil, err
	}
	return tab, nil
}

func (e *ex) clearOpenTerminalSessions(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	for i := 0; ; i++ {
		name := terminalSessionAutoName(e.workspaceURI, i)
		var doc terminalSessionDocument
		err := e.storage.Get(ctx, terminalSessionDocumentID(name), &doc)
		if errors.Is(err, storageapi.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := e.storage.Delete(ctx, terminalSessionDocumentID(name)); err != nil {
			return err
		}
	}
}

func (e *ex) openTerminalSessionDocuments(
	ctx context.Context,
) ([]terminalSessionDocument, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	docs := make([]terminalSessionDocument, 0)
	for i := 0; ; i++ {
		name := terminalSessionAutoName(e.workspaceURI, i)
		var doc terminalSessionDocument
		err := e.storage.Get(ctx, terminalSessionDocumentID(name), &doc)
		if errors.Is(err, storageapi.ErrNotFound) {
			return docs, nil
		}
		if err != nil {
			return nil, err
		}
		if doc.Name == "" {
			doc.Name = name
		}
		docs = append(docs, doc)
	}
}

func (e *ex) completeTerminalSessions(ctx context.Context, _ textapi.Command) (
	iterator.Iterator[string], string, error,
) {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	it, err := e.storage.List(ctx, []storageapi.Filter{{
		Field: storageapi.Field{FieldPath: []string{"Kind"}, Value: terminalSessionDocumentKind},
		Op:    storageapi.OpEqual,
	}})
	if err != nil {
		return nil, "", err
	}
	defer it.Close()

	var names []string
	for it.HasNext() {
		var doc terminalSessionDocument
		if err := it.NextTo(&doc); err != nil {
			return nil, "", err
		}
		if doc.Name != "" && !isTerminalSessionAutoName(doc.Name) {
			names = append(names, doc.Name)
		}
	}
	sort.Strings(names)
	return iterator.FromSlice(names), "", nil
}
