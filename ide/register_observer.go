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

package ide

import (
	"context"
	"sync"

	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/text"
)

var _ text.Editor = (*commandRegisterObserver)(nil)

// commandRegisterObserver wraps a text.Editor and observes command
// registrations made through it.
type commandRegisterObserver struct {
	ed text.Editor

	mu         sync.Mutex
	registered map[string]struct{}
	waiters    map[string][]chan struct{}
}

func newCommandRegisterObserver(ed text.Editor) *commandRegisterObserver {
	return &commandRegisterObserver{
		ed:         ed,
		registered: map[string]struct{}{},
		waiters:    map[string][]chan struct{}{},
	}
}

func (o *commandRegisterObserver) Edit(
	ctx context.Context, file workspaceapi.URI, buf *cell.Buffer,
	readOnly, recovered bool,
) (text.Handler, error) {
	return o.ed.Edit(ctx, file, buf, readOnly, recovered)
}

func (o *commandRegisterObserver) Editor(
	uri workspaceapi.URI,
) (text.Handler, error) {
	return o.ed.Editor(uri)
}

func (o *commandRegisterObserver) UnsubscribeCommand(cmd string) error {
	return o.ed.UnsubscribeCommand(cmd)
}

func (o *commandRegisterObserver) UnregisterREPLCommand(cmd string) error {
	return o.ed.UnregisterREPLCommand(cmd)
}

func (o *commandRegisterObserver) IsExternal() bool {
	return o.ed.IsExternal()
}

func (o *commandRegisterObserver) SubscribeEvents(
	types []textapi.EventType, h text.EventHandler,
) error {
	return o.ed.SubscribeEvents(types, h)
}

func (o *commandRegisterObserver) UnsubscribeEvents(
	h text.EventHandler,
) (bool, error) {
	return o.ed.UnsubscribeEvents(h)
}

func (o *commandRegisterObserver) SubscribeCommand(
	cmd textapi.CommandManual, h text.CommandHandler,
) error {
	if err := o.ed.SubscribeCommand(cmd, h); err != nil {
		return err
	}
	o.markRegistered(cmd.Name)
	return nil
}

func (o *commandRegisterObserver) RegisterREPLCommand(
	cmd textapi.CommandManual, h textapi.REPLHandler,
) error {
	if err := o.ed.RegisterREPLCommand(cmd, h); err != nil {
		return err
	}
	o.markRegistered(cmd.Name)
	return nil
}

func (o *commandRegisterObserver) markRegistered(cmd string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.registered[cmd] = struct{}{}
	for _, ch := range o.waiters[cmd] {
		close(ch)
	}
	delete(o.waiters, cmd)
}

// Wait blocks until cmd has been registered through this editor or ctx
// is done. It returns immediately if cmd is already registered.
func (o *commandRegisterObserver) Wait(ctx context.Context, cmd string) error {
	o.mu.Lock()
	if _, ok := o.registered[cmd]; ok {
		o.mu.Unlock()
		return nil
	}
	ch := make(chan struct{})
	o.waiters[cmd] = append(o.waiters[cmd], ch)
	o.mu.Unlock()

	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		o.removeWaiter(cmd, ch)
		return ctx.Err()
	}
}

func (o *commandRegisterObserver) removeWaiter(cmd string, ch chan struct{}) {
	o.mu.Lock()
	defer o.mu.Unlock()
	waiters := o.waiters[cmd]
	for i, w := range waiters {
		if w == ch {
			o.waiters[cmd] = append(waiters[:i], waiters[i+1:]...)
			break
		}
	}
	if len(o.waiters[cmd]) == 0 {
		delete(o.waiters, cmd)
	}
}
