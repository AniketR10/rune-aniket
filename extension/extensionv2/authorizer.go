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

package extensionv2

import (
	"context"
	"errors"
	"fmt"

	blueauth "github.com/unstablebuild/blue/auth"
	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"unstable.build/go-tui/text"
)

// Extension represents an authenticated extension which is
// associated with some access to some resources.
type Extension struct {
	extensionapi.Metadata
	Plugin bool
	Path   string
	Args   []string
}

// PluginPermissionRequest describes a single permission requested by an
// ad-hoc program.
type PluginPermissionRequest struct {
	Path         string
	Args         []string
	LauncherPath string
	LauncherArgs []string
	Permission   extensionapi.Permission
	Resource     string
}

// PluginPermissionDecision is the user's decision for an ad-hoc program
// permission request.
type PluginPermissionDecision string

const (
	// PluginPermissionAllowOnce allows the requested permission for the current
	// plugin process identity only.
	PluginPermissionAllowOnce PluginPermissionDecision = "allow-once"
	// PluginPermissionAllowAlways persists an allow decision for the plugin
	// program identity and requested permission.
	PluginPermissionAllowAlways PluginPermissionDecision = "allow-always"
	// PluginPermissionDenyOnce denies the requested permission for the current
	// plugin process identity only.
	PluginPermissionDenyOnce PluginPermissionDecision = "deny-once"
	// PluginPermissionDenyAlways persists a deny decision for the plugin program
	// identity and requested permission.
	PluginPermissionDenyAlways PluginPermissionDecision = "deny-always"
)

// PluginPermissionPrompter prompts for an ad-hoc program permission.
type PluginPermissionPrompter interface {
	PromptPluginPermission(
		context.Context, PluginPermissionRequest,
	) (PluginPermissionDecision, error)
}

// newAuthorizer returns an auth.Authorizer of Extension. It uses the permissions
// in the granted claims to authorize access to a resource.
func newAuthorizer(
	prompter PluginPermissionPrompter, storage storageapi.Service,
	editor text.Editor,
) (blueauth.Authorizer[Extension], error) {
	if editor == nil {
		return nil, errors.New("editor is required")
	}
	plugin := newPluginPermissionAuthorizer(prompter, storage)
	if err := registerAuthorizerREPLCommand(editor, plugin); err != nil {
		return nil, fmt.Errorf("register authorizer repl command: %w", err)
	}
	return authorizer{
		plugin: plugin,
	}, nil
}

type authorizer struct {
	plugin *pluginPermissionAuthorizer
}

func (a authorizer) Authorize(
	ctx context.Context, claims blueauth.UserClaims[Extension], resource string,
) (err error) {
	perm, ok := extensionapi.PermissionForResource(resource)
	if !ok {
		err = fmt.Errorf("extraneous rpc resource %s: %w", resource, blueauth.ErrForbidden)
		return
	}
	if claims.Extra.Plugin {
		return a.plugin.Authorize(ctx, claims.Extra, perm, resource)
	}
	if _, ok := claims.Extra.Permissions[perm]; !ok {
		err = blueauth.ErrForbidden
		return
	}
	return nil
}
