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

package ideauthorizer

import (
	"context"
	"fmt"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/handler"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// Permission prompt option labels shown in permission prompts.
const (
	PromptOptionYes       = "   Yes   "
	PromptOptionYesAlways = "   Always   "
	PromptOptionNo        = "   No   "
	PromptOptionNoNever   = "   Never   "
)

type permissionPrompter struct {
	promptOpener     PromptOpener
	scheduleNextTick func(func()) bool
	notifications    browserapi.Notifications
}

func newPermissionPrompter(
	promptOpener PromptOpener,
	scheduleNextTick func(func()) bool,
	notifications browserapi.Notifications,
) *permissionPrompter {
	if promptOpener == nil || scheduleNextTick == nil {
		return nil
	}
	if notifications == nil {
		notifications = nopPromptNotifications{}
	}
	return &permissionPrompter{
		promptOpener:     promptOpener,
		scheduleNextTick: scheduleNextTick,
		notifications:    notifications,
	}
}

func (p *permissionPrompter) PromptPermission(
	ctx context.Context, req PermissionRequest,
) (PermissionDecision, error) {
	result := make(chan PermissionDecision, 1)
	message := pluginPermissionPromptMessage(req)
	warning := permissionPromptWarningMessage(req)
	options := []string{
		PromptOptionYes,
		PromptOptionYesAlways,
		PromptOptionNo,
		PromptOptionNoNever,
	}
	bindings := []term.KeyComb{{Ch: 'Y'}, {Ch: 'A'}, {Ch: 'N'}, {Ch: 'V'}}
	scheduled := p.scheduleNextTick(func() {
		_, _ = p.notifications.Notify(browserapi.LevelWarn, warning)
		p.promptOpener.Prompt(message, options, bindings, handler.FuncPromptHandler(
			func(i int, opt string) {
				select {
				case result <- pluginPromptDecision(opt):
				default:
				}
			},
			func() error {
				select {
				case result <- PermissionDenyOnce:
				default:
				}
				return nil
			}))
	})
	if !scheduled {
		return PermissionDenyOnce, nil
	}

	select {
	case decision := <-result:
		return decision, nil
	case <-ctx.Done():
		return PermissionDenyOnce, ctx.Err()
	}
}

func permissionPromptWarningMessage(req PermissionRequest) string {
	labels := displayScopeLabels(req)
	if len(labels) > 0 {
		return fmt.Sprintf("User authorization required for %s", strings.Join(labels, ", "))
	}
	if req.CommandPath != "" {
		return fmt.Sprintf("User authorization required for %s", req.CommandPath)
	}
	if req.ExtensionName != "" {
		return fmt.Sprintf("User authorization required for %s", req.ExtensionName)
	}
	if req.Path != "" {
		return fmt.Sprintf("User authorization required for %s", req.Path)
	}
	return "User authorization required"
}

func displayScopeLabels(req PermissionRequest) []string {
	labels := req.CommandScopeLabels
	if len(labels) == 0 && req.CommandScopeLabel != "" {
		labels = []string{req.CommandScopeLabel}
	}
	if len(labels) == 0 {
		return nil
	}
	ret := make([]string, len(labels))
	for i, l := range labels {
		ret[i] = strings.TrimSpace(strings.ReplaceAll(l, "*", ""))
	}
	return ret
}

func pluginPermissionPromptMessage(req PermissionRequest) string {
	action := fmt.Sprintf("**%s**", permissionActionText(req.Permission))
	commandScopeNote := commandScopeNote(req)
	if req.CommandPath != "" && req.ExtensionID != "" && req.ExtensionName != "" {
		command := fmt.Sprintf("run **%s** with args **%v**", req.CommandPath, req.CommandArgs)
		if req.CommandDir != "" {
			command = fmt.Sprintf("%s in **%s**", command, req.CommandDir)
		}
		return fmt.Sprintf("Extension %s by %s wants to %s.%s",
			req.ExtensionName, req.DeveloperID, command, commandScopeNote)
	}
	if req.ExtensionID != "" && req.ExtensionName != "" {
		return fmt.Sprintf("Extension %s by %s wants to %s.",
			req.ExtensionName, req.DeveloperID, action)
	}
	if req.CommandPath != "" {
		command := fmt.Sprintf("run **%s** with args **%v**", req.CommandPath, req.CommandArgs)
		if req.CommandDir != "" {
			command = fmt.Sprintf("%s in **%s**", command, req.CommandDir)
		}
		if req.LauncherPath != "" {
			return fmt.Sprintf("Program **%s** with args **%v** running inside **%s** **%v** wants to %s.%s",
				req.Path, req.Args, req.LauncherPath, req.LauncherArgs, command,
				commandScopeNote)
		}
		return fmt.Sprintf("Program **%s** with args **%v** wants to %s.%s",
			req.Path, req.Args, command, commandScopeNote)
	}
	message := fmt.Sprintf("Program %s with args %v wants to %s.",
		req.Path, req.Args, action)
	if req.LauncherPath != "" {
		message = fmt.Sprintf("Program %s with args %v running inside %s %v wants to %s.",
			req.Path, req.Args, req.LauncherPath, req.LauncherArgs, action)
	}
	return message
}

// commandScopeNote renders the scope-approval suffix shown after a
// command-scoped permission request.
// for a command-scoped permission prompt. When multiple scope labels are
// provided, each is rendered bold and separated by commas so the user
// sees which commands will be broadened.
func commandScopeNote(req PermissionRequest) string {
	labels := displayScopeLabels(req)
	if len(labels) == 0 {
		return ""
	}
	parts := make([]string, len(labels))
	for i, l := range labels {
		parts[i] = fmt.Sprintf("**%s**", l)
	}
	return fmt.Sprintf(
		"\n\nChoosing **Always** approves %s for all future authorization requests, for any argument combination.",
		strings.Join(parts, ", "),
	)
}

func pluginPromptDecision(opt string) PermissionDecision {
	switch opt {
	case PromptOptionYes:
		return PermissionAllowOnce
	case PromptOptionYesAlways:
		return PermissionAllowAlways
	case PromptOptionNoNever:
		return PermissionDenyAlways
	case PromptOptionNo:
		fallthrough
	default:
		return PermissionDenyOnce
	}
}

// nopPromptNotifications is used when no notifications implementation is
// provided to newPermissionPrompter; it silently drops all notifications.
type nopPromptNotifications struct{}

func (nopPromptNotifications) Notify(
	browserapi.NotificationLevel, string, ...any,
) (string, error) {
	return "", nil
}

func (nopPromptNotifications) NotifyOnce(
	browserapi.NotificationLevel, string, ...any,
) (string, error) {
	return "", nil
}

func (nopPromptNotifications) UpdateNotificationProgress(
	string, string, int64, int64,
) error {
	return nil
}
