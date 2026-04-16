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
	"sync"

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

// PermissionRequest describes a single permission requested by an extension or
// ad-hoc plugin program.
type PermissionRequest struct {
	Path          string
	Args          []string
	LauncherPath  string
	LauncherArgs  []string
	ExtensionID   string
	ExtensionName string
	DeveloperID   string
	Permission    extensionapi.Permission
	Resource      string
}

// PermissionDecision is the user's decision for an extension permission request.
type PermissionDecision string

const (
	// PermissionAllowOnce allows the requested permission temporarily.
	PermissionAllowOnce PermissionDecision = "allow-once"
	// PermissionAllowAlways persists an allow decision for the extension identity
	// and requested permission.
	PermissionAllowAlways PermissionDecision = "allow-always"
	// PermissionDenyOnce denies the requested permission temporarily.
	PermissionDenyOnce PermissionDecision = "deny-once"
	// PermissionDenyAlways persists a deny decision for the extension identity and
	// requested permission.
	PermissionDenyAlways PermissionDecision = "deny-always"
)

// PermissionPrompter prompts for an extension permission.
type PermissionPrompter interface {
	PromptPermission(
		context.Context, PermissionRequest,
	) (PermissionDecision, error)
}

// newAuthorizer returns an auth.Authorizer of Extension. It uses the permissions
// in the granted claims to authorize access to a resource.
func newAuthorizer(
	prompter PermissionPrompter, storage storageapi.Service,
	editor text.Editor,
) (blueauth.Authorizer[Extension], error) {
	if editor == nil {
		return nil, errors.New("editor is required")
	}
	a := &authorizer{
		prompter: prompter,
		storage:  storage,
		once:     make(map[string]pluginPermissionOnceDecision),
	}
	if err := registerAuthorizerREPLCommand(editor, a); err != nil {
		return nil, fmt.Errorf("register authorizer repl command: %w", err)
	}
	return a, nil
}

type authorizer struct {
	prompter PermissionPrompter
	storage  storageapi.Service

	onceMu sync.Mutex
	once   map[string]pluginPermissionOnceDecision
}

func (a *authorizer) Authorize(
	ctx context.Context, claims blueauth.UserClaims[Extension], resource string,
) (err error) {
	perm, ok := extensionapi.PermissionForResource(resource)
	if !ok {
		err = fmt.Errorf("extraneous rpc resource %s: %w", resource, blueauth.ErrForbidden)
		return
	}
	if claims.Extra.Plugin {
		return a.authorizePlugin(ctx, claims.Extra, perm, resource)
	}
	if _, ok := claims.Extra.Permissions[perm]; !ok {
		err = blueauth.ErrForbidden
		return
	}
	return a.authorizeExtension(ctx, claims.Extra, perm, resource)
}
