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

package text

import (
	"errors"

	"github.com/unstablebuild/tcell/v3"
	"unstable.build/go-tui/api/textapi"
	"unstable.build/go-tui/api/workspaceapi"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/clipboard"
	"unstable.build/go-tui/term"
)

// DefaultSimpleEditor returns a simple to use Editor implementation.
func DefaultSimpleEditor(clipboard clipboard.Register) Editor {
	searchAttr := term.Attributes{Attrs: tcell.AttrReverse}
	return NewSimpleEditor(workspaceapi.URI{}, clipboard, false, true, false, false,
		term.Attributes{}, searchAttr, term.Attributes{}, AuxBarConfig{},
		GitBarConfig{}, nil, func(fn func()) bool { fn(); return true })
}

// NewSimpleEditor allocates storage for a new Editor and initializes it.
// The scheduleNextTick parameter can be nil if auxBar is false.
func NewSimpleEditor(
	cwd workspaceapi.URI,
	clipboard clipboard.Register,
	wrap, commandBar, auxBar, gitBar bool,
	attr, searchAttr, barAttr term.Attributes,
	auxBarConfig AuxBarConfig,
	gitBarConfig GitBarConfig,
	registry WorkspaceCommandRegistry,
	scheduleNextTick func(func()) bool,
) Editor {
	if auxBar && scheduleNextTick == nil {
		panic("nil schedule function")
	}
	ret := new(simpleEditor)
	ret.scheduleNextTick = scheduleNextTick
	ret.wrap = wrap
	ret.registry = registry
	ret.commandBar = commandBar
	ret.auxBar = auxBar
	ret.auxBarConfig = auxBarConfig
	ret.gitBarConfig = gitBarConfig
	ret.gitBar = gitBar
	ret.resAttr = searchAttr
	ret.barAttr = barAttr
	ret.attr = attr
	ret.clipboard = clipboard
	ret.pub.Init()
	if ret.registry != nil {
		ret.fileRegistry = NewFileCommandRegistry(cwd, ret.registry)
	}
	return ret
}

type simpleEditor struct {
	scheduleNextTick func(func()) bool
	registry         WorkspaceCommandRegistry
	fileRegistry     FileCommandRegistry
	pub              Publisher
	wrap             bool
	commandBar       bool
	auxBar           bool
	gitBar           bool
	auxBarConfig     AuxBarConfig
	gitBarConfig     GitBarConfig
	attr             term.Attributes
	resAttr          term.Attributes
	barAttr          term.Attributes
	clipboard        clipboard.Register
}

func (e *simpleEditor) Edit(file workspaceapi.URI, buf *cell.Buffer) (Handler, error) {
	rootIfc := NewSimpleHandler(e.clipboard, buf, file, e.wrap,
		e.commandBar, e.attr, e.resAttr, e.barAttr, e.scheduleNextTick)
	if e.fileRegistry != nil {
		var err error
		rootIfc, err = SubscribeLocationCommands(file, e.fileRegistry, rootIfc)
		if err != nil {
			return nil, err
		}
	}
	cursor := &rootIfc.(*simpleEditorHandler).cursor
	scroll := rootIfc.(*simpleEditorHandler).less.Scroll()
	ret := e.pub.PublishEdit(file, buf, rootIfc, cursor)
	auxBarConfig := e.auxBarConfig
	auxBarConfig.CommandRegistry = e.fileRegistry
	gitBarConfig := e.gitBarConfig
	gitBarConfig.CommandRegistry = e.fileRegistry
	if e.auxBar {
		ret = WithAuxBar(ret, buf, scroll, auxBarConfig)
		if e.gitBar {
			ret = WithGitBar(e.auxBarConfig.Service, ret, buf,
				scroll, gitBarConfig)
		}
	}
	return ret, nil
}

func (e *simpleEditor) SubscribeCommand(cmd textapi.CommandManual, h CommandHandler) error {
	return errors.New("not supported")
}

func (c *simpleEditor) UnsubscribeCommand(cmd string) error {
	return errors.New("not supported")
}

func (e *simpleEditor) Editor(file workspaceapi.URI) (Handler, error) {
	return nil, errors.New("not supported")
}

func (e *simpleEditor) SubscribeEvents(evs []textapi.EventType, sub EventHandler) error {
	e.pub.SubscribeEvents(evs, sub)
	return nil
}

func (e *simpleEditor) UnsubscribeEvents(sub EventHandler) (bool, error) {
	ok := e.pub.UnsubscribeEvents(sub)
	return ok, nil
}
