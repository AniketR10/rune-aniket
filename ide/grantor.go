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
	"fmt"
	"net/url"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/handler"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/extension"
)

var _ extension.Grantor = (*permissionGrantor)(nil)

const (
	permissionDecisionAllow = "allow"
	permissionDecisionDeny  = "deny"
)

const (
	permissionPromptAllowOnce   = "   Once   "
	permissionPromptAllowAlways = "   Always   "
	permissionPromptDenyOnce    = "   No   "
	permissionPromptDenyNever   = "   Never   "
)

type permissionDecision struct {
	Decision string
}

type permissionGrantor struct {
	promptOpener     ExtensionPromptOpener
	storage          storageapi.Service
	scheduleNextTick func(func()) bool
}

func newExtensionPromptGrantor(
	promptOpener ExtensionPromptOpener,
	storage storageapi.Service,
	scheduleNextTick func(func()) bool,
) extension.Grantor {
	if promptOpener == nil || storage == nil || scheduleNextTick == nil {
		return extension.GrantAll()
	}
	return &permissionGrantor{
		promptOpener:     promptOpener,
		storage:          storage,
		scheduleNextTick: scheduleNextTick,
	}
}

func (g *permissionGrantor) Grant(meta extensionapi.Metadata) (bool, error) {
	ctx := context.Background()
	key := permissionStorageKey(meta)
	var stored permissionDecision
	err := g.storage.Get(ctx, key, &stored)
	if err == nil {
		switch stored.Decision {
		case permissionDecisionAllow:
			return true, nil
		case permissionDecisionDeny:
			return false, nil
		default:
			return false, fmt.Errorf("unknown stored extension "+
				"permission decision %q", stored.Decision)
		}
	}
	if !errors.Is(err, storageapi.ErrNotFound) {
		return false, fmt.Errorf("get extension permission decision: %w", err)
	}

	decision := g.prompt(meta)
	switch decision {
	case permissionPromptAllowAlways:
		err := g.storage.Set(ctx, key, permissionDecision{Decision: permissionDecisionAllow})
		if err != nil {
			return false, fmt.Errorf("set extension permission decision: %w", err)
		}
		return true, nil
	case permissionPromptAllowOnce:
		return true, nil
	case permissionPromptDenyNever:
		err := g.storage.Set(ctx, key, permissionDecision{Decision: permissionDecisionDeny})
		if err != nil {
			return false, fmt.Errorf("set extension permission decision: %w", err)
		}
		return false, nil
	case permissionPromptDenyOnce:
		return false, nil
	default:
		return false, nil
	}
}

func (g *permissionGrantor) prompt(meta extensionapi.Metadata) string {
	result := make(chan string, 1)
	message := fmt.Sprintf("Allow extension **%s** (%s) by **%s** to run?",
		meta.ExtensionName, meta.ExtensionVersion, meta.DeveloperID)
	options := []string{
		permissionPromptAllowOnce,
		permissionPromptAllowAlways,
		permissionPromptDenyOnce,
		permissionPromptDenyNever,
	}
	bindings := []term.KeyComb{{Ch: 'o'}, {Ch: 'a'}, {Ch: 'n'}, {Ch: 'v'}}
	scheduled := g.scheduleNextTick(func() {
		g.promptOpener.Prompt(message, options, bindings, handler.FuncPromptHandler(
			func(i int, opt string) {
				select {
				case result <- opt:
				default:
				}
			},
			func() error {
				select {
				case result <- permissionPromptDenyOnce:
				default:
				}
				return nil
			}))
	})
	if !scheduled {
		return permissionPromptDenyOnce
	}
	return <-result
}

func permissionStorageKey(meta extensionapi.Metadata) string {
	parts := []string{
		"extension-permission",
		meta.DeveloperID,
		meta.DeveloperKey,
		meta.ExtensionID,
		meta.ExtensionName,
	}
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}
