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

package extensionv2

import (
	"context"
	"fmt"

	"github.com/unstablebuild/rune-go-sdk/handler"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/ide"
)

const (
	pluginPermissionPromptYes       = "   Yes   "
	pluginPermissionPromptYesAlways = "   Yes, Always   "
	pluginPermissionPromptNo        = "   No   "
	pluginPermissionPromptNoNever   = "   No, Never   "
)

type pluginPermissionPrompter struct {
	promptOpener     ide.ExtensionPromptOpener
	scheduleNextTick func(func()) bool
}

func newPluginPermissionPrompter(
	promptOpener ide.ExtensionPromptOpener,
	scheduleNextTick func(func()) bool,
) PluginPermissionPrompter {
	if promptOpener == nil || scheduleNextTick == nil {
		return nil
	}
	return &pluginPermissionPrompter{
		promptOpener:     promptOpener,
		scheduleNextTick: scheduleNextTick,
	}
}

func (p *pluginPermissionPrompter) PromptPluginPermission(
	ctx context.Context, req PluginPermissionRequest,
) (PluginPermissionDecision, error) {
	result := make(chan PluginPermissionDecision, 1)
	message := pluginPermissionPromptMessage(req)
	options := []string{
		pluginPermissionPromptYes,
		pluginPermissionPromptYesAlways,
		pluginPermissionPromptNo,
		pluginPermissionPromptNoNever,
	}
	bindings := []term.KeyComb{{Ch: 'Y'}, {Ch: 'A'}, {Ch: 'N'}, {Ch: 'V'}}
	scheduled := p.scheduleNextTick(func() {
		p.promptOpener.Prompt(message, options, bindings, handler.FuncPromptHandler(
			func(i int, opt string) {
				select {
				case result <- pluginPromptDecision(opt):
				default:
				}
			},
			func() error {
				select {
				case result <- PluginPermissionDenyOnce:
				default:
				}
				return nil
			}))
	})
	if !scheduled {
		return PluginPermissionDenyOnce, nil
	}

	select {
	case decision := <-result:
		return decision, nil
	case <-ctx.Done():
		return PluginPermissionDenyOnce, ctx.Err()
	}
}

func pluginPermissionPromptMessage(req PluginPermissionRequest) string {
	message := fmt.Sprintf("Program %s with args %v wants to %s.",
		req.Path, req.Args, PluginPermissionActionText(req.Permission))
	if req.LauncherPath != "" {
		message = fmt.Sprintf("Program %s with args %v running inside %s %v wants to %s.",
			req.Path, req.Args, req.LauncherPath, req.LauncherArgs,
			PluginPermissionActionText(req.Permission))
	}
	return message
}

func pluginPromptDecision(opt string) PluginPermissionDecision {
	switch opt {
	case pluginPermissionPromptYes:
		return PluginPermissionAllowOnce
	case pluginPermissionPromptYesAlways:
		return PluginPermissionAllowAlways
	case pluginPermissionPromptNoNever:
		return PluginPermissionDenyAlways
	case pluginPermissionPromptNo:
		fallthrough
	default:
		return PluginPermissionDenyOnce
	}
}

var _ PluginPermissionPrompter = (*pluginPermissionPrompter)(nil)
