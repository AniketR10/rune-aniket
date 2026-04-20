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

// Package ideauthorizer provides the authorization logic for the IDE
// workspace extension host. An Authorizer satisfies both the workspace
// CommandAuthorizer interface (for per-command authorization of
// Executor.StartCommand payloads) and blueauth.Authorizer[Extension] (for
// gRPC middleware that authenticates and authorizes extension RPCs).
//
// The package lives under ide/ so that both ide and extension/extensionv2
// can depend on it without creating an import cycle.
package ideauthorizer

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	blueauth "github.com/unstablebuild/blue/auth"
	"github.com/unstablebuild/blue/auth/grpcauth"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi/workspacerpc"
	"github.com/unstablebuild/rune-go-sdk/handler"
	"github.com/unstablebuild/rune-go-sdk/term"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"unstable.build/go-tui/browser"
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
	Path               string
	Args               []string
	LauncherPath       string
	LauncherArgs       []string
	ExtensionID        string
	ExtensionName      string
	DeveloperID        string
	Permission         extensionapi.Permission
	Resource           string
	CommandPath        string
	CommandArgs        []string
	CommandDir         string
	CommandScopeLabel  string
	CommandScopeLabels []string
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

// PromptOpener opens a browser prompt for extension host decisions.
type PromptOpener interface {
	Prompt(
		message string, options []string,
		bindings []term.KeyComb,
		promptHandler handler.PromptHandler,
	) browser.Window
}

// NewAuthorizer returns an Authorizer that satisfies both the workspace
// CommandAuthorizer interface and the blue auth authorizer used by the
// extension runner for gRPC middleware.
func NewAuthorizer(
	editor text.Editor,
	promptOpener PromptOpener, storage storageapi.Service,
	scheduleNextTick func(func()) bool,
	notifications browserapi.Notifications,
) (*Authorizer, error) {
	if editor == nil {
		return nil, errors.New("editor is required")
	}
	a := &Authorizer{
		prompter: newPermissionPrompter(promptOpener, scheduleNextTick, notifications),
		storage:  storage,
		once:     make(map[string]pluginPermissionOnceDecision),
	}
	if err := registerAuthorizerREPLCommand(editor, a); err != nil {
		return nil, fmt.Errorf("register authorizer repl command: %w", err)
	}
	return a, nil
}

// Authorizer implements both workspacerpc.CommandAuthorizer and
// blueauth.Authorizer[Extension]. It is used by the extension runner to
// authorize gRPC requests and by the workspace server to authorize
// individual StartCommand payloads.
type Authorizer struct {
	prompter *permissionPrompter
	storage  storageapi.Service

	onceMu sync.Mutex
	once   map[string]pluginPermissionOnceDecision
}

// GRPCAuthServerOptions returns the gRPC server options that install
// this authorizer as middleware for extension authentication/authorization.
// When creds is nil, insecure OAuth2 is used (transport secured externally).
func (a *Authorizer) GRPCAuthServerOptions(
	keys blueauth.Keys, creds credentials.TransportCredentials,
) []grpc.ServerOption {
	if creds == nil {
		return grpcauth.GRPCServerWithInsecureOauth2(keys, a)
	}
	return grpcauth.GRPCServerWithOauth2(keys, a, creds)
}

// Authorize satisfies blueauth.Authorizer[Extension].
func (a *Authorizer) Authorize(
	ctx context.Context, claims blueauth.UserClaims[Extension], resource string,
) (err error) {
	perm, ok := extensionapi.PermissionForResource(resource)
	if !ok {
		err = fmt.Errorf("extraneous rpc resource %s: %w", resource, blueauth.ErrForbidden)
		return
	}
	// StartCommand is a client-streaming RPC, so the generic gRPC auth layer
	// only knows the method name here. Do not prompt/cache a blanket
	// PermissionExecute decision before seeing the command payload. Let the
	// stream open; workspacerpc.Server.StartCommand reads the first payload,
	// builds the workspaceapi.Cmd, and then calls AuthorizeCommand with
	// path/args/dir before any process is started.
	if resource == workspacerpc.Executor_StartCommand_FullMethodName {
		return nil
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

func (a *Authorizer) authorizePlugin(
	ctx context.Context, ext Extension, perm extensionapi.Permission, resource string,
) error {
	identity := pluginPermissionIdentityFromContext(ctx, ext)
	key := pluginPermissionStorageKey(identity.Path, identity.Args, perm)
	onceKey := pluginPermissionOnceKey(identity, perm, nil)
	return a.authorizePermission(ctx, ext, identity, []string{key}, onceKey, perm, resource, nil)
}

// AuthorizeCommand satisfies workspacerpc.CommandAuthorizer.
func (a *Authorizer) AuthorizeCommand(
	ctx context.Context, cmd workspaceapi.Cmd,
) error {
	claims, ok := blueauth.ClaimsFromContext[Extension](ctx)
	if !ok {
		return nil
	}
	command := pluginPermissionCommandDetail{
		Path: cmd.Path,
		Args: append([]string(nil), cmd.Args...),
		Dir:  cmd.Dir,
	}
	perm := extensionapi.PermissionExecute
	resource := workspacerpc.Executor_StartCommand_FullMethodName
	var identity pluginPermissionIdentity
	var keys []string
	var onceKey string
	if claims.Extra.Plugin {
		identity = pluginPermissionIdentityFromContext(ctx, claims.Extra)
		keys = pluginPermissionCommandStorageKeys(identity.Path, identity.Args, perm, command)
		onceKey = pluginPermissionOnceKey(identity, perm, &command)
	} else {
		if _, ok := claims.Extra.Permissions[perm]; !ok {
			return blueauth.ErrForbidden
		}
		identity = extensionPermissionIdentity(claims.Extra)
		keys = extensionPermissionCommandStorageKeys(claims.Extra, perm, command)
		onceKey = stablePermissionOnceKey(identity, perm, &command)
	}
	return a.authorizePermission(ctx, claims.Extra, identity, keys, onceKey, perm, resource, &command)
}

func (a *Authorizer) authorizeExtension(
	ctx context.Context, ext Extension, perm extensionapi.Permission, resource string,
) error {
	identity := extensionPermissionIdentity(ext)
	key := extensionPermissionStorageKey(ext, perm)
	onceKey := stablePermissionOnceKey(identity, perm, nil)
	return a.authorizePermission(ctx, ext, identity, []string{key}, onceKey, perm, resource, nil)
}

func (a *Authorizer) authorizePermission(
	ctx context.Context, ext Extension, identity pluginPermissionIdentity,
	keys []string, onceKey string, perm extensionapi.Permission, resource string,
	command *pluginPermissionCommandDetail,
) error {
	missingKeys := keys
	if a.storage != nil && len(keys) > 0 {
		var err error
		missingKeys, err = a.filterMissingStoredDecisions(ctx, keys)
		if err != nil {
			return err
		}
		if len(missingKeys) == 0 {
			return nil
		}
	}

	if decision, ok := a.getOnceDecision(onceKey, time.Now()); ok {
		switch decision {
		case pluginPermissionDecisionAllow:
			return nil
		case pluginPermissionDecisionDeny:
			return blueauth.ErrForbidden
		default:
			return fmt.Errorf("unknown cached plugin permission decision %q", decision)
		}
	}

	if a.prompter == nil {
		return blueauth.ErrForbidden
	}
	req := PermissionRequest{
		Path:         identity.Path,
		Args:         append([]string(nil), identity.Args...),
		LauncherPath: identity.LauncherPath,
		LauncherArgs: append([]string(nil), identity.LauncherArgs...),
		Permission:   perm,
		Resource:     resource,
	}
	if command != nil {
		req.CommandPath = command.Path
		req.CommandArgs = append([]string(nil), command.Args...)
		req.CommandDir = command.Dir
		labels := pluginPermissionApprovalScopeLabels(*command)
		req.CommandScopeLabels = labels
		req.CommandScopeLabel = strings.Join(labels, ", ")
	}
	if !ext.Plugin {
		req.ExtensionID = ext.ExtensionID
		req.ExtensionName = ext.ExtensionName
		req.DeveloperID = ext.DeveloperID
	}
	decision, err := a.prompter.PromptPermission(ctx, req)
	if err != nil {
		return err
	}

	switch decision {
	case PermissionAllowOnce:
		a.setOnceDecision(onceKey, identity, perm, command,
			pluginPermissionDecisionAllow, time.Now())
		return nil
	case PermissionAllowAlways:
		for _, k := range missingKeys {
			if err := a.setStoredDecision(ctx, k, identity, perm, command,
				pluginPermissionDecisionAllow); err != nil {
				return err
			}
		}
		return nil
	case PermissionDenyOnce:
		a.setOnceDecision(onceKey, identity, perm, command,
			pluginPermissionDecisionDeny, time.Now())
		return blueauth.ErrForbidden
	case PermissionDenyAlways:
		for _, k := range missingKeys {
			if err := a.setStoredDecision(ctx, k, identity, perm, command,
				pluginPermissionDecisionDeny); err != nil {
				return err
			}
		}
		return blueauth.ErrForbidden
	default:
		return blueauth.ErrForbidden
	}
}

// filterMissingStoredDecisions returns the subset of keys that have no
// persisted allow/deny decision. If any key has a stored deny, it returns
// blueauth.ErrForbidden immediately. Returns nil when all keys are stored
// as allow (meaning no prompt is required).
func (a *Authorizer) filterMissingStoredDecisions(
	ctx context.Context, keys []string,
) ([]string, error) {
	missing := make([]string, 0, len(keys))
	for _, k := range keys {
		var stored storedPermissionDecision
		err := a.storage.Get(ctx, k, &stored)
		if err == nil {
			switch stored.Decision {
			case pluginPermissionDecisionAllow:
				continue
			case pluginPermissionDecisionDeny:
				return nil, blueauth.ErrForbidden
			default:
				return nil, fmt.Errorf(
					"unknown stored plugin permission decision %q", stored.Decision)
			}
		}
		if !errors.Is(err, storageapi.ErrNotFound) {
			return nil, fmt.Errorf("get plugin permission decision: %w", err)
		}
		missing = append(missing, k)
	}
	return missing, nil
}

func (a *Authorizer) getOnceDecision(
	key string, now time.Time,
) (string, bool) {
	if key == "" {
		return "", false
	}
	a.onceMu.Lock()
	defer a.onceMu.Unlock()
	decision, ok := a.once[key]
	if !ok {
		return "", false
	}
	if !now.Before(decision.Expires) {
		delete(a.once, key)
		return "", false
	}
	return decision.Decision, true
}

func (a *Authorizer) setOnceDecision(
	key string, identity pluginPermissionIdentity,
	perm extensionapi.Permission, command *pluginPermissionCommandDetail,
	decision string, now time.Time,
) {
	if key == "" {
		return
	}
	a.onceMu.Lock()
	defer a.onceMu.Unlock()
	a.purgeExpiredOnceDecisionsLocked(now)
	onceDecision := pluginPermissionOnceDecision{
		Decision:   decision,
		Path:       identity.Path,
		Args:       append([]string(nil), identity.Args...),
		Permission: perm,
		Expires:    now.Add(pluginPermissionOnceTTL),
	}
	if command != nil {
		onceDecision.Command = copyPluginPermissionCommandDetail(*command)
	}
	a.once[key] = onceDecision
}

func (a *Authorizer) purgeExpiredOnceDecisionsLocked(now time.Time) {
	for key, decision := range a.once {
		if !now.Before(decision.Expires) {
			delete(a.once, key)
		}
	}
}

func sameStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (a *Authorizer) setStoredDecision(
	ctx context.Context, key string, identity pluginPermissionIdentity,
	perm extensionapi.Permission, command *pluginPermissionCommandDetail, decision string,
) error {
	if a.storage == nil {
		return nil
	}
	stored := storedPermissionDecision{
		Key:        key,
		Decision:   decision,
		Path:       identity.Path,
		Args:       append([]string(nil), identity.Args...),
		Permission: perm,
	}
	if command != nil {
		stored.Command = copyPluginPermissionCommandDetail(*command)
	}
	err := a.storage.Set(ctx, key, stored)
	if err != nil {
		return fmt.Errorf("set plugin permission decision: %w", err)
	}
	return nil
}
