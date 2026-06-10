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
	"fmt"
	"os"

	"github.com/unstablebuild/rune-go-sdk/api/storageapi"

	"unstable.build/go-tui/handler/command"
	"unstable.build/go-tui/ide/idetutorial"
	"unstable.build/go-tui/ide/idetutorial/starlarktutorial"
)

func buildTutorials(i *IDE) map[string]idetutorial.Tutorial {
	files := i.ideConfig.tutorialFiles()
	embedded := i.options.starlarkTutorials
	if len(files) == 0 && len(embedded) == 0 {
		return nil
	}
	var partition storageapi.Service
	p, err := i.storage.Partition("idetutorial")
	if err == nil {
		partition = p
	} else {
		partition = i.storage
	}
	br := currentBrowser{root: i.workspaceHandler}
	ed := currentEditor{root: i.workspaceHandler}
	parser := currentParser{root: i.workspaceHandler}
	notifications := i.workspaceHandler.notifications.current()
	defaultAttr := i.ideConfig.defaultAttr()
	frameCharSet := i.ideConfig.windowFrameCharset()
	promptConfig := i.ideConfig.promptConfig()
	scheduleNextTick := i.options.scheduleFn
	commandKey := i.ideConfig.commandKey()
	editorMode := i.ideConfig.pkgEditorMode()
	rawKeyFor := i.ideConfig.commandKeyBindingLookup()
	keyForCommand := func(cmd string, args []string) string {
		return starlarktutorial.PrettyKeySpec(rawKeyFor(cmd, args))
	}
	manualLookup := buildTutorialCommandManualLookup(i.workspaceHandler)

	tutorials := make(map[string]idetutorial.Tutorial,
		len(files)+len(embedded))
	for name, src := range embedded {
		t, err := starlarktutorial.New(
			name, src,
			br, ed, notifications, parser,
			defaultAttr, frameCharSet, promptConfig,
			scheduleNextTick, partition, commandKey,
			editorMode, keyForCommand, manualLookup,
		)
		if err != nil {
			i.ideConfig.errors["tutorials."+name] = err
			continue
		}
		tutorials[name] = t
	}
	for name, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			i.ideConfig.errors["tutorials."+name] = fmt.Errorf(
				"read %q: %w", path, err)
			continue
		}
		t, err := starlarktutorial.New(
			name, string(src),
			br, ed, notifications, parser,
			defaultAttr, frameCharSet, promptConfig,
			scheduleNextTick, partition, commandKey,
			editorMode, keyForCommand, manualLookup,
		)
		if err != nil {
			i.ideConfig.errors["tutorials."+name] = err
			continue
		}
		tutorials[name] = t
	}
	return tutorials
}

// buildTutorialCommandManualLookup returns a closure that resolves
// a command name to its registered command.Manual. It consults the
// focused editor's subscribed commands first and then the workspace
// handler's alias expander so authors writing `wait_command("e")`
// (an alias for `edit`) see the alias entry in the hint window.
func buildTutorialCommandManualLookup(
	root *workspaceManagerHandler,
) starlarktutorial.CommandManualLookup {
	if root == nil {
		return nil
	}
	return func(name string) (command.Manual, bool) {
		ex := root.focusEx()
		if ex == nil {
			return command.Manual{}, false
		}
		for _, man := range ex.comp.Commands() {
			if man.Name == name {
				return man, true
			}
		}
		if ex.aliasExpander != nil {
			for _, man := range ex.aliasExpander.Aliases() {
				if man.Name == name {
					return man, true
				}
			}
		}
		return command.Manual{}, false
	}
}
