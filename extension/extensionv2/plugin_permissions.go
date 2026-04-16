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
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	blueauth "github.com/unstablebuild/blue/auth"
	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
)

const (
	pluginPermissionDecisionAllow    = "allow"
	pluginPermissionDecisionDeny     = "deny"
	pluginPermissionOnceTTL          = 10 * time.Minute
	pluginPermissionStoragePrefix    = "extensionv2:plugin-permissions:"
	extensionPermissionStoragePrefix = "extensionv2:extension-permissions:"
)

type storedPermissionDecision struct {
	Key        string
	Decision   string
	Path       string
	Args       []string
	Permission extensionapi.Permission
}

type pluginPermissionOnceDecision struct {
	Decision   string
	Path       string
	Args       []string
	Permission extensionapi.Permission
	Expires    time.Time
}

type pluginPermissionIdentity struct {
	PID          int
	UID          uint32
	Path         string
	Args         []string
	LauncherPath string
	LauncherArgs []string
}

func (a *authorizer) authorizePlugin(
	ctx context.Context, ext Extension, perm extensionapi.Permission, resource string,
) error {
	identity := pluginPermissionIdentityFromContext(ctx, ext)
	key := pluginPermissionStorageKey(identity.Path, identity.Args, perm)
	onceKey := pluginPermissionOnceKey(identity, perm)
	return a.authorizePermission(ctx, ext, identity, key, onceKey, perm, resource)
}

func (a *authorizer) authorizeExtension(
	ctx context.Context, ext Extension, perm extensionapi.Permission, resource string,
) error {
	identity := pluginPermissionIdentity{
		Path: ext.ExtensionID,
		Args: []string{ext.DeveloperID, ext.DeveloperKey, ext.ExtensionName},
	}
	key := extensionPermissionStorageKey(ext, perm)
	onceKey := stablePermissionOnceKey(identity, perm)
	return a.authorizePermission(ctx, ext, identity, key, onceKey, perm, resource)
}

func (a *authorizer) authorizePermission(
	ctx context.Context, ext Extension, identity pluginPermissionIdentity,
	key, onceKey string, perm extensionapi.Permission, resource string,
) error {
	if a.storage != nil {
		var stored storedPermissionDecision
		err := a.storage.Get(ctx, key, &stored)
		if err == nil {
			switch stored.Decision {
			case pluginPermissionDecisionAllow:
				return nil
			case pluginPermissionDecisionDeny:
				return blueauth.ErrForbidden
			default:
				return fmt.Errorf("unknown stored plugin permission decision %q",
					stored.Decision)
			}
		}
		if !errors.Is(err, storageapi.ErrNotFound) {
			return fmt.Errorf("get plugin permission decision: %w", err)
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
		a.setOnceDecision(onceKey, identity, perm, pluginPermissionDecisionAllow, time.Now())
		return nil
	case PermissionAllowAlways:
		return a.setStoredDecision(ctx, key, identity, perm, pluginPermissionDecisionAllow)
	case PermissionDenyOnce:
		a.setOnceDecision(onceKey, identity, perm, pluginPermissionDecisionDeny, time.Now())
		return blueauth.ErrForbidden
	case PermissionDenyAlways:
		err := a.setStoredDecision(ctx, key, identity, perm, pluginPermissionDecisionDeny)
		if err != nil {
			return err
		}
		return blueauth.ErrForbidden
	default:
		return blueauth.ErrForbidden
	}
}

func pluginPermissionIdentityFromContext(
	ctx context.Context, ext Extension,
) pluginPermissionIdentity {
	ret := pluginPermissionIdentity{
		Path: ext.Path,
		Args: append([]string(nil), ext.Args...),
	}
	process, ok := peerProcessFromContext(ctx)
	if !ok {
		return ret
	}
	path := process.ProgramPath()
	if path == "" {
		return ret
	}
	ret.Path = path
	ret.Args = process.ProgramArgs()
	ret.PID = process.PID
	ret.UID = process.UID
	if ret.Path != ext.Path || !sameStringSlice(ret.Args, ext.Args) {
		ret.LauncherPath = ext.Path
		ret.LauncherArgs = append([]string(nil), ext.Args...)
	}
	return ret
}

func (a *authorizer) getOnceDecision(
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

func (a *authorizer) setOnceDecision(
	key string, identity pluginPermissionIdentity,
	perm extensionapi.Permission, decision string, now time.Time,
) {
	if key == "" {
		return
	}
	a.onceMu.Lock()
	defer a.onceMu.Unlock()
	a.purgeExpiredOnceDecisionsLocked(now)
	a.once[key] = pluginPermissionOnceDecision{
		Decision:   decision,
		Path:       identity.Path,
		Args:       append([]string(nil), identity.Args...),
		Permission: perm,
		Expires:    now.Add(pluginPermissionOnceTTL),
	}
}

func (a *authorizer) purgeExpiredOnceDecisionsLocked(now time.Time) {
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

func (a *authorizer) setStoredDecision(
	ctx context.Context, key string, identity pluginPermissionIdentity,
	perm extensionapi.Permission, decision string,
) error {
	if a.storage == nil {
		return nil
	}
	err := a.storage.Set(ctx, key, storedPermissionDecision{
		Key:        key,
		Decision:   decision,
		Path:       identity.Path,
		Args:       append([]string(nil), identity.Args...),
		Permission: perm,
	})
	if err != nil {
		return fmt.Errorf("set plugin permission decision: %w", err)
	}
	return nil
}

func pluginPermissionStorageKey(
	path string, args []string, perm extensionapi.Permission,
) string {
	parts := []string{
		"extensionv2",
		"plugin-permissions",
		pluginProgramHash(path, args),
		url.PathEscape(string(perm)),
	}
	return strings.Join(parts, ":")
}

func extensionPermissionStorageKey(ext Extension, perm extensionapi.Permission) string {
	parts := []string{
		"extensionv2",
		"extension-permissions",
		pluginProgramHash(ext.ExtensionID, []string{
			ext.DeveloperID,
			ext.DeveloperKey,
			ext.ExtensionName,
		}),
		url.PathEscape(string(perm)),
	}
	return strings.Join(parts, ":")
}

func pluginPermissionOnceKey(
	identity pluginPermissionIdentity, perm extensionapi.Permission,
) string {
	if identity.PID == 0 {
		return ""
	}
	parts := []string{
		"extensionv2",
		"plugin-permission-once",
		pluginProgramHashWithProcess(identity.PID, identity.UID, identity.Path, identity.Args),
		url.PathEscape(string(perm)),
	}
	return strings.Join(parts, ":")
}

func stablePermissionOnceKey(
	identity pluginPermissionIdentity, perm extensionapi.Permission,
) string {
	if identity.Path == "" {
		return ""
	}
	parts := []string{
		"extensionv2",
		"extension-permission-once",
		pluginProgramHash(identity.Path, identity.Args),
		url.PathEscape(string(perm)),
	}
	return strings.Join(parts, ":")
}

func pluginProgramHash(path string, args []string) string {
	h := sha256.New()
	writePluginProgramHash(h, path, args)
	return hex.EncodeToString(h.Sum(nil))
}

func pluginProgramHashWithProcess(pid int, uid uint32, path string, args []string) string {
	h := sha256.New()
	_, _ = h.Write([]byte(strconv.Itoa(pid)))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(strconv.FormatUint(uint64(uid), 10)))
	_, _ = h.Write([]byte{0})
	writePluginProgramHash(h, path, args)
	return hex.EncodeToString(h.Sum(nil))
}

func writePluginProgramHash(h hashWriter, path string, args []string) {
	_, _ = h.Write([]byte(path))
	_, _ = h.Write([]byte{0})
	for _, arg := range args {
		_, _ = h.Write([]byte(arg))
		_, _ = h.Write([]byte{0})
	}
}

type hashWriter interface {
	Write([]byte) (int, error)
}

// PermissionActionText returns the phrase used to describe the action
// requested by an ad-hoc program.
func PermissionActionText(permission extensionapi.Permission) string {
	switch permission {
	case extensionapi.PermissionFileSystem:
		return "access workspace files"
	case extensionapi.PermissionExecute:
		return "execute commands"
	case extensionapi.PermissionTerminal:
		return "manage terminals"
	case extensionapi.PermissionBrowserWindowManager:
		return "manage the Window Manager"
	case extensionapi.PermissionBrowserResourceOpener:
		return "manage file tabs"
	case extensionapi.PermissionNotifications:
		return "show notifications"
	case extensionapi.PermissionInterrupt:
		return "interrupt the event loop"
	case extensionapi.PermissionEditor:
		return "access the editor"
	case extensionapi.PermissionCommands:
		return "register prompt commands"
	case extensionapi.PermissionStorage:
		return "use persistent storage"
	case extensionapi.PermissionSyntaxTree:
		return "inspect syntax trees"
	case extensionapi.PermissionConfig:
		return "read user configuration"
	case extensionapi.PermissionLSP:
		return "communicate with LSP servers"
	case extensionapi.PermissionDebugger:
		return "communicate with DAP servers"
	default:
		return fmt.Sprintf("access permission %s", permission)
	}
}
