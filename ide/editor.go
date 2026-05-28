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
	"errors"

	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"

	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/text"
)

type currentEditor struct {
	root *workspaceManagerHandler
}

var _ text.Editor = currentEditor{}

func (c currentEditor) editor() text.Editor {
	if c.root == nil {
		return nil
	}
	ex := c.root.focusEx()
	if ex == nil {
		return nil
	}
	return ex.ed
}

var errNoEditor = errors.New("no focused editor")

func (c currentEditor) Edit(
	ctx context.Context, file workspaceapi.URI, buf *cell.Buffer,
	readOnly, recovered bool,
) (text.Handler, error) {
	ed := c.editor()
	if ed == nil {
		return nil, errNoEditor
	}
	return ed.Edit(ctx, file, buf, readOnly, recovered)
}

func (c currentEditor) Editor(uri workspaceapi.URI) (text.Handler, error) {
	ed := c.editor()
	if ed == nil {
		return nil, errNoEditor
	}
	return ed.Editor(uri)
}

func (c currentEditor) SubscribeCommand(
	m textapi.CommandManual, h text.CommandHandler,
) error {
	ed := c.editor()
	if ed == nil {
		return errNoEditor
	}
	return ed.SubscribeCommand(m, h)
}

func (c currentEditor) RegisterREPLCommand(
	m textapi.CommandManual, h textapi.REPLHandler,
) error {
	ed := c.editor()
	if ed == nil {
		return errNoEditor
	}
	return ed.RegisterREPLCommand(m, h)
}

func (c currentEditor) UnsubscribeCommand(name string) error {
	ed := c.editor()
	if ed == nil {
		return errNoEditor
	}
	return ed.UnsubscribeCommand(name)
}

func (c currentEditor) UnregisterREPLCommand(name string) error {
	ed := c.editor()
	if ed == nil {
		return errNoEditor
	}
	return ed.UnregisterREPLCommand(name)
}

func (c currentEditor) IsExternal() bool {
	ed := c.editor()
	if ed == nil {
		return false
	}
	return ed.IsExternal()
}

func (c currentEditor) SubscribeEvents(
	types []textapi.EventType, h text.EventHandler,
) error {
	ed := c.editor()
	if ed == nil {
		return errNoEditor
	}
	return ed.SubscribeEvents(types, h)
}

func (c currentEditor) UnsubscribeEvents(h text.EventHandler) (bool, error) {
	ed := c.editor()
	if ed == nil {
		return false, errNoEditor
	}
	return ed.UnsubscribeEvents(h)
}
