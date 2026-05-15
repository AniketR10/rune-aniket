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
)

type terminalSessionDocument struct {
	Kind     string
	Name     string
	Snapshot vte.Snapshot
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
	title := doc.Name
	if doc.Snapshot.Title != "" {
		title = doc.Snapshot.Title
	}
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
	}
	return tab, nil
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
		if doc.Name != "" {
			names = append(names, doc.Name)
		}
	}
	sort.Strings(names)
	return iterator.FromSlice(names), "", nil
}
