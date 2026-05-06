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
	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/extension"
	"unstable.build/go-tui/ide/ideauthorizer"
	"unstable.build/go-tui/text"
)

// ExtensionsRunner abstracts the ability to construct extension.Runner.
type ExtensionsRunner interface {
	WorkspaceExtensionsRunner(
		uri workspaceapi.URI, res map[extensionapi.Permission]extension.ResourceRegistrar,
		authorizer *ideauthorizer.Authorizer,
		dataDir string, notifications browser.Notifications,
		executor, extExecutor schemeapi.Executor,
		grantor extension.Grantor,
		editor text.Editor,
		promptOpener ideauthorizer.PromptOpener, storage storageapi.Service,
		scheduleNextTick func(func()) bool,
	) (extension.Runner, error)
}

// FuncExtensionsRunner wraps fn to satisfy Extensions by invoking in calls to Runner.
func FuncExtensionsRunner(
	fn func(workspaceapi.URI,
		map[extensionapi.Permission]extension.ResourceRegistrar, string,
		browser.Notifications, schemeapi.Executor, schemeapi.Executor,
		extension.Grantor,
		text.Editor,
		ideauthorizer.PromptOpener, storageapi.Service,
		func(func()) bool) (extension.Runner, error),
) ExtensionsRunner {
	return fnExtensions{fn: fn}
}

type fnExtensions struct {
	fn func(workspaceapi.URI,
		map[extensionapi.Permission]extension.ResourceRegistrar, string,
		browser.Notifications, schemeapi.Executor, schemeapi.Executor,
		extension.Grantor,
		text.Editor,
		ideauthorizer.PromptOpener, storageapi.Service,
		func(func()) bool) (extension.Runner, error)
}

func (f fnExtensions) WorkspaceExtensionsRunner(
	uri workspaceapi.URI,
	res map[extensionapi.Permission]extension.ResourceRegistrar,
	authorizer *ideauthorizer.Authorizer,
	dataDir string, n browser.Notifications, exec, extExec schemeapi.Executor,
	grantor extension.Grantor,
	editor text.Editor,
	promptOpener ideauthorizer.PromptOpener, storage storageapi.Service,
	scheduleNextTick func(func()) bool,
) (extension.Runner, error) {
	return f.fn(uri, res, dataDir, n, exec, extExec, grantor, editor,
		promptOpener, storage, scheduleNextTick)
}
