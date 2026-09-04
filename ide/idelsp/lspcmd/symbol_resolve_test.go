// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package lspcmd

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

type recordingNotifications struct {
	mu       sync.Mutex
	notifies []notifyCall
	updates  []updateCall
}

type notifyCall struct {
	level browserapi.NotificationLevel
	msg   string
}

type updateCall struct {
	id          string
	msg         string
	step, total int64
}

func (r *recordingNotifications) Notify(
	level browserapi.NotificationLevel, msg string, args ...any,
) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	formatted := fmt.Sprintf(msg, args...)
	r.notifies = append(r.notifies, notifyCall{level: level, msg: formatted})
	return formatted, nil
}

func (r *recordingNotifications) NotifyOnce(
	level browserapi.NotificationLevel, msg string, args ...any,
) (string, error) {
	return r.Notify(level, msg, args...)
}

func (r *recordingNotifications) UpdateNotificationProgress(
	id, msg string, step, total int64,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.updates = append(r.updates, updateCall{id: id, msg: msg, step: step, total: total})
	return nil
}

func (r *recordingNotifications) snapshot() ([]notifyCall, []updateCall) {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := append([]notifyCall(nil), r.notifies...)
	u := append([]updateCall(nil), r.updates...)
	return n, u
}

func TestResolveCommandSymbolNoMatchesShowsError(t *testing.T) {
	t.Parallel()

	notify := &recordingNotifications{}
	resolved := make(chan struct{})
	parser := &mockParser{
		searchFn: func(string, []string) (iterator.Iterator[syntaxapi.Result], error) {
			return iterator.Empty[syntaxapi.Result](), nil
		},
		searchNodeFn: func(syntaxapi.NodeCaptureName) (iterator.Iterator[syntaxapi.Result], error) {
			return iterator.Empty[syntaxapi.Result](), nil
		},
	}

	cmd := &textapi.Command{Args: []string{"iterator.Iterator"}}
	// resolveCommandSymbol schedules multiple callbacks: progress
	// hops from the resolver goroutine (post-RUNE-218) and the
	// resolution dispatch. Synchronise on the resolution callback
	// by signaling only after a tick that produces an error
	// notification rather than on the first tick.
	var resolvedOnce sync.Once
	tick := func(fn func()) bool {
		before, _ := notify.snapshot()
		fn()
		after, _ := notify.snapshot()
		if len(after) > len(before) {
			for _, n := range after[len(before):] {
				if n.level == browserapi.LevelError {
					resolvedOnce.Do(func() { close(resolved) })
				}
			}
		}
		return true
	}
	proceed, err := resolveCommandSymbol(
		t.Context(), cmd, workspaceapi.URI{}, nil, nil, notify, tick, parser, func(syntaxapi.Match, func()) {},
	)
	assert.False(t, proceed)
	assert.NoError(t, err)
	<-resolved

	notifies, _ := notify.snapshot()
	var errorMsgs []string
	for _, n := range notifies {
		if n.level == browserapi.LevelError {
			errorMsgs = append(errorMsgs, n.msg)
		}
	}
	assert.NotEmpty(t, errorMsgs,
		"expected an error notification after resolver returned no matches; got %#v", notifies)
	if len(errorMsgs) > 0 {
		assert.Contains(t, strings.ToLower(errorMsgs[0]), "no symbols found",
			"expected error notification to mention not-found; got %q", errorMsgs[0])
	}
}

func TestResolveCommandSymbolUnqualifiedShowsHint(t *testing.T) {
	t.Parallel()

	notify := &recordingNotifications{}
	resolved := make(chan struct{})
	parser := &mockParser{
		resolveFn: func(string, syntaxapi.Progress) ([]syntaxapi.Match, error) {
			return nil, syntaxapi.ErrNoDot
		},
	}

	cmd := &textapi.Command{Args: []string{"Println"}}
	var resolvedOnce sync.Once
	tick := func(fn func()) bool {
		before, _ := notify.snapshot()
		fn()
		after, _ := notify.snapshot()
		for _, n := range after[len(before):] {
			if n.level == browserapi.LevelError {
				resolvedOnce.Do(func() { close(resolved) })
			}
		}
		return true
	}
	proceed, err := resolveCommandSymbol(
		t.Context(), cmd, workspaceapi.URI{}, nil, nil, notify, tick, parser, func(syntaxapi.Match, func()) {},
	)
	assert.False(t, proceed)
	assert.NoError(t, err)
	<-resolved

	notifies, _ := notify.snapshot()
	var errorMsgs []string
	for _, n := range notifies {
		if n.level == browserapi.LevelError {
			errorMsgs = append(errorMsgs, n.msg)
		}
	}
	require.NotEmpty(t, errorMsgs)
	// The hint must guide the user to qualify the symbol rather than
	// the opaque "no symbols found".
	msg := errorMsgs[0]
	assert.Contains(t, msg, "Println")
	assert.Contains(t, strings.ToLower(msg), "qualif")
	assert.NotContains(t, strings.ToLower(msg), "no symbols found")
}
