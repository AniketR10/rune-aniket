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

package lspcmd

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
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
		t.Context(), cmd, nil, nil, notify, tick, parser, func(symbolMatch, func()) {},
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
