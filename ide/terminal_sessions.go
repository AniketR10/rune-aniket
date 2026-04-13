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

const terminalSessionPartition = "terminal-sessions"

type terminalSnapshotter interface {
	TerminalSnapshot() (vte.Snapshot, error)
}

type terminalSnapshotRestorer interface {
	RestoreTerminalSnapshot(vte.Snapshot) error
}

type terminalURIer interface {
	URI() workspaceapi.URI
}

type terminalSessionDocument struct {
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

func (e *ex) terminalInFocus() (terminalSnapshotter, bool) {
	content, err := e.invokeWindow().Content()
	if err != nil {
		return nil, false
	}

	switch h := content.(type) {
	case terminalSnapshotter:
		return h, true
	case companionTerminalHandler:
		if s, ok := h.vth.(terminalSnapshotter); ok {
			return s, true
		}
	case *browser.Tab:
		if s, ok := h.Handler().(terminalSnapshotter); ok {
			return s, true
		}
	}

	return nil, false
}

func (e *ex) nextTerminalSessionName(ctx context.Context, h terminalSnapshotter) (string, error) {
	base := "terminal"
	if uriProvider, ok := h.(terminalURIer); ok {
		uri := uriProvider.URI()
		if uri.Name() != "" {
			base = uri.Name()
		} else if uri.Path() != "" {
			base = strings.Trim(strings.ReplaceAll(uri.Path(), "/", "-"), "-")
		}
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
		err := e.terminalSessionStore.Get(ctx, name, &doc)
		cancel()
		if errors.Is(err, storageapi.ErrNotFound) {
			return name, nil
		}
		if err != nil {
			return "", fmt.Errorf("check terminal session name %q: %w", name, err)
		}
	}
}

func (e *ex) saveTerminalSession(ctx context.Context, name string, h terminalSnapshotter) error {
	snapshot, err := h.TerminalSnapshot()
	if err != nil {
		return fmt.Errorf("snapshot terminal: %w", err)
	}
	if snapshot.Title == "" {
		snapshot.Title = name
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	err = e.terminalSessionStore.Set(ctx, name, terminalSessionDocument{
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
	if err := e.terminalSessionStore.Get(ctx, name, &doc); err != nil {
		return fmt.Errorf("resume terminal session %q: %w", name, err)
	}
	if doc.Name == "" {
		doc.Name = name
	}

	uri, err := terminalSessionURI(doc.Name)
	if err != nil {
		return err
	}
	if existing, ok := e.comp.Resource(uri); ok {
		return e.invokeWindow().SetContent(existing)
	}

	h, err := e.newTerminalSessionHandler(doc.Name, doc.Snapshot)
	if err != nil {
		return err
	}
	t, err := e.comp.Tab(uri, e.config.Icons.Terminal, doc.Name, h)
	if err != nil {
		_ = h.Close()
		return fmt.Errorf("wm.Tab: %w", err)
	}
	tab := t.(*browser.Tab)
	tab.Subscribe((*tabSubscriber)(e))
	win := e.invokeWindow()
	if err := win.SetContent(t); err != nil {
		_ = t.Close()
		return err
	}
	if restorer, ok := h.(terminalSnapshotRestorer); ok {
		return restorer.RestoreTerminalSnapshot(doc.Snapshot)
	}
	return nil
}

func (e *ex) newTerminalSessionHandler(
	name string, snapshot vte.Snapshot,
) (vtereservoir.VTE, error) {
	h, err := e.newEmulatorHandler(nil)
	if err != nil {
		return nil, err
	}
	restorer, ok := h.(terminalSnapshotRestorer)
	if !ok {
		_ = h.Close()
		return nil, fmt.Errorf("terminal %q does not support snapshot restore", name)
	}
	if err := restorer.RestoreTerminalSnapshot(snapshot); err != nil {
		_ = h.Close()
		return nil, fmt.Errorf("restore terminal session %q: %w", name, err)
	}
	return h, nil
}

func (e *ex) completeTerminalSessions(ctx context.Context, _ textapi.Command) (
	iterator.Iterator[string], string, error,
) {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	it, err := e.terminalSessionStore.List(ctx, nil)
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
