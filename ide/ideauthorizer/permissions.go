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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
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
	Command    pluginPermissionCommandDetail
}

type pluginPermissionOnceDecision struct {
	Decision   string
	Path       string
	Args       []string
	Permission extensionapi.Permission
	Command    pluginPermissionCommandDetail
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

type pluginPermissionCommandDetail struct {
	Path string
	Args []string
	Dir  string
}

func pluginPermissionStorageKey(
	path string, args []string, perm extensionapi.Permission,
	command *pluginPermissionCommandDetail,
) string {
	parts := []string{
		"extensionv2",
		"plugin-permissions",
		pluginProgramHash(path, args),
		url.PathEscape(string(perm)),
	}
	if command != nil {
		parts = append(parts, pluginPermissionCommandHash(*command))
	}
	return strings.Join(parts, ":")
}

func extensionPermissionStorageKey(
	ext Extension, perm extensionapi.Permission, command *pluginPermissionCommandDetail,
) string {
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
	if command != nil {
		parts = append(parts, pluginPermissionCommandHash(*command))
	}
	return strings.Join(parts, ":")
}

func pluginPermissionOnceKey(
	identity pluginPermissionIdentity, perm extensionapi.Permission,
	command *pluginPermissionCommandDetail,
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
	if command != nil {
		parts = append(parts, pluginPermissionCommandHash(*command))
	}
	return strings.Join(parts, ":")
}

func stablePermissionOnceKey(
	identity pluginPermissionIdentity, perm extensionapi.Permission,
	command *pluginPermissionCommandDetail,
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
	if command != nil {
		parts = append(parts, pluginPermissionCommandHash(*command))
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

func pluginPermissionCommandHash(command pluginPermissionCommandDetail) string {
	h := sha256.New()
	writePluginPermissionCommandHash(h, command)
	return hex.EncodeToString(h.Sum(nil))
}

func writePluginPermissionCommandHash(
	h hashWriter, command pluginPermissionCommandDetail,
) {
	_, _ = h.Write([]byte(command.Path))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(command.Dir))
	_, _ = h.Write([]byte{0})
	for _, arg := range command.Args {
		_, _ = h.Write([]byte(arg))
		_, _ = h.Write([]byte{0})
	}
}

func copyPluginPermissionCommandDetail(
	command pluginPermissionCommandDetail,
) pluginPermissionCommandDetail {
	return pluginPermissionCommandDetail{
		Path: command.Path,
		Args: append([]string(nil), command.Args...),
		Dir:  command.Dir,
	}
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

// permissionActionText returns the phrase used to describe the action
// requested by an ad-hoc program.
func permissionActionText(permission extensionapi.Permission) string {
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

func extensionPermissionIdentity(ext Extension) pluginPermissionIdentity {
	return pluginPermissionIdentity{
		Path: ext.ExtensionID,
		Args: []string{ext.DeveloperID, ext.DeveloperKey, ext.ExtensionName},
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
