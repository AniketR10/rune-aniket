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

package standard

import (
	"context"
	"errors"

	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/ide/vctrl/vctrlcmd"
	"unstable.build/go-tui/text"
)

// Editor allocates storage for a new Editor and initializes it.
func Editor(opts ...Option) text.Editor {
	ret := new(editor)
	ret.standardConfig = defaultConfig()
	for _, o := range opts {
		o(&ret.standardConfig)
	}
	ret.indents = text.IndentConfig{}
	ret.pub.Init()
	if ret.standardConfig.registry != nil {
		ret.fileRegistry = text.NewFileCommandRegistry(
			ret.standardConfig.workspace, ret.standardConfig.registry)
	}
	ret.opts = opts
	return ret
}

type editor struct {
	standardConfig
	indents      text.IndentConfig
	fileRegistry text.FileCommandRegistry
	pub          text.Publisher
	opts         []Option
}

type publisherEventsAdapter struct{ pub *text.Publisher }

func (a publisherEventsAdapter) SubscribeEvents(evs []textapi.EventType, sub text.EventHandler) error {
	a.pub.SubscribeEvents(evs, sub)
	return nil
}

func (a publisherEventsAdapter) UnsubscribeEvents(sub text.EventHandler) (bool, error) {
	return a.pub.UnsubscribeEvents(sub), nil
}

func (e *editor) Edit(
	ctx context.Context,
	file workspaceapi.URI, buf *cell.Buffer, readOnly, recovered bool,
) (ret text.Handler, err error) {
	indentRune := text.IndentRuneTab
	r, tabspaces, ok := text.IndentConfigForURI(
		file, buf, e.indents, e.standardConfig.tabspaces,
	)
	if ok {
		indentRune = r
	}
	root := NewHandler(buf, file, indentRune, tabspaces, e.opts...).(*standardHandler)
	ret = root
	cursor := &root.cursor
	if e.fileRegistry != nil {
		var err error
		ret, err = text.SubscribeLocationCommands(file, e.fileRegistry, ret)
		if err != nil {
			return nil, err
		}
		ret, err = text.SubscribeFoldCommands(file, e.fileRegistry, cursor, ret)
		if err != nil {
			return nil, err
		}
		ret, err = text.SubscribeIndentCommands(file, e.fileRegistry, cursor, e.indents, ret)
		if err != nil {
			return nil, err
		}
		ret, err = text.SubscribeCommentCommands(file, e.fileRegistry, cursor, ret)
		if err != nil {
			return nil, err
		}
		ret, err = vctrlcmd.SubscribeGitCommands(file, e.fileRegistry,
			ret, e.auxBarConfig.Service, e.clipboard, e.notifications)
		if err != nil {
			return nil, err
		}
	}
	scroll := root.less.Scroll()
	ret = e.pub.PublishEdit(file, buf, ret, cursor)
	if text.BarsFromContext(ctx) {
		auxBarConfig := e.auxBarConfig
		auxBarConfig.CommandRegistry = e.fileRegistry
		iconsBarConfig := e.iconsBarConfig
		iconsBarConfig.CommandRegistry = e.fileRegistry
		iconsBarConfig.Publisher = publisherEventsAdapter{pub: &e.pub}
		if e.enableAuxBar {
			ret = text.WithAuxBar(ret, buf, scroll, auxBarConfig)
		}
		if e.enableIconsBar {
			ret = text.WithIconsBar(e.auxBarConfig.Service, e.enableGitIcons, ret, buf,
				scroll, iconsBarConfig)
		}
		if e.statusBarEnabled {
			bar := text.WithStatusBar(ret, buf, scroll, readOnly, recovered, e.statusBarConfig)
			root.setStatusBar(bar)
			if !e.commandBar {
				bar.ShowCommandBar(false)
			}
			ret = bar
		}
	}
	if e.search.WindowManager != nil {
		ret = newSearchHandler(ret, root, e.search)
	}
	return ret, nil
}

func (e *editor) SubscribeCommand(cmd textapi.CommandManual, h text.CommandHandler) error {
	return errors.New("not supported")
}

func (e *editor) RegisterREPLCommand(
	cmd textapi.CommandManual, h textapi.REPLHandler,
) error {
	return errors.New("not supported")
}

func (e *editor) REPLCommands() []textapi.CommandManual {
	return nil
}

func (c *editor) UnsubscribeCommand(cmd string) error {
	return errors.New("not supported")
}

func (c *editor) UnregisterREPLCommand(cmd string) error {
	return errors.New("not supported")
}

func (e *editor) Editor(file workspaceapi.URI) (text.Handler, error) {
	return nil, errors.New("not supported")
}

func (e *editor) SubscribeEvents(evs []textapi.EventType, sub text.EventHandler) error {
	e.pub.SubscribeEvents(evs, sub)
	return nil
}

func (e *editor) UnsubscribeEvents(sub text.EventHandler) (bool, error) {
	ok := e.pub.UnsubscribeEvents(sub)
	return ok, nil
}

// IsExternal reports false: the standard editor manages its buffer
// in process.
func (*editor) IsExternal() bool { return false }
