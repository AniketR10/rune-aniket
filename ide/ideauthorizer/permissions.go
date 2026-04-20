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
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"mvdan.cc/sh/v3/syntax"
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
) string {
	return strings.Join([]string{
		"extensionv2",
		"plugin-permissions",
		pluginProgramHash(path, args),
		url.PathEscape(string(perm)),
	}, ":")
}

// pluginPermissionCommandStorageKeys returns the storage keys for a
// command-scoped permission decision. When the command can be decomposed
// into a known set of effective commands, returns one key per command
// identity so that approving "Yes, All" for a compound script broadens
// independently for each effective command. When the command is opaque
// (e.g. an unparseable shell script), returns a single exact-match key.
func pluginPermissionCommandStorageKeys(
	path string, args []string, perm extensionapi.Permission,
	command pluginPermissionCommandDetail,
) []string {
	prefix := pluginPermissionStorageKey(path, args, perm)
	identities := pluginPermissionPersistedCommandIdentities(command)
	keys := make([]string, len(identities))
	for i, id := range identities {
		keys[i] = prefix + ":" + identityHash(id)
	}
	return keys
}

func extensionPermissionStorageKey(
	ext Extension, perm extensionapi.Permission,
) string {
	return strings.Join([]string{
		"extensionv2",
		"extension-permissions",
		pluginProgramHash(ext.ExtensionID, []string{
			ext.DeveloperID,
			ext.DeveloperKey,
			ext.ExtensionName,
		}),
		url.PathEscape(string(perm)),
	}, ":")
}

// extensionPermissionCommandStorageKeys is the extension-keyed counterpart
// of pluginPermissionCommandStorageKeys.
func extensionPermissionCommandStorageKeys(
	ext Extension, perm extensionapi.Permission,
	command pluginPermissionCommandDetail,
) []string {
	prefix := extensionPermissionStorageKey(ext, perm)
	identities := pluginPermissionPersistedCommandIdentities(command)
	keys := make([]string, len(identities))
	for i, id := range identities {
		keys[i] = prefix + ":" + identityHash(id)
	}
	return keys
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

func identityHash(identity string) string {
	h := sha256.New()
	_, _ = h.Write([]byte(identity))
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

// pluginPermissionPersistedCommandIdentities returns the identity strings
// that key persisted decisions for command. When the command can be
// decomposed, returns one "<basename> *" identity per effective command.
// Otherwise, returns a single exact-match identity covering the full Cmd.
func pluginPermissionPersistedCommandIdentities(
	command pluginPermissionCommandDetail,
) []string {
	if cmds, ok := pluginPermissionEffectiveCommands(command); ok {
		out := make([]string, len(cmds))
		for i, c := range cmds {
			out[i] = c + " *"
		}
		return out
	}
	return []string{pluginPermissionExactCommandIdentity(command)}
}

// pluginPermissionApprovalScopeLabels returns the human-readable labels
// describing what "Yes, All" will approve for command. Mirrors the
// identities returned by pluginPermissionPersistedCommandIdentities.
func pluginPermissionApprovalScopeLabels(
	command pluginPermissionCommandDetail,
) []string {
	if cmds, ok := pluginPermissionEffectiveCommands(command); ok {
		out := make([]string, len(cmds))
		for i, c := range cmds {
			out[i] = c + " *"
		}
		return out
	}
	return []string{pluginPermissionExactCommandLabel(command)}
}

func pluginPermissionExactCommandIdentity(command pluginPermissionCommandDetail) string {
	return pluginPermissionCommandHash(command)
}

func pluginPermissionExactCommandLabel(command pluginPermissionCommandDetail) string {
	parts := []string{command.Path}
	if len(command.Args) > 0 {
		parts = append(parts, strings.Join(command.Args, " "))
	}
	if command.Dir != "" {
		parts = append(parts, "@ "+command.Dir)
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

// pluginPermissionEffectiveCommands extracts the sorted, deduplicated set
// of effective command basenames that command will execute. Returns
// (nil, false) when the command can't be safely decomposed (parse error,
// non-literal expansions, unsupported constructs, eval/source/.) — in
// which case callers should fall back to exact-match identity.
func pluginPermissionEffectiveCommands(
	command pluginPermissionCommandDetail,
) ([]string, bool) {
	if command.Path == "" {
		return nil, false
	}
	if !pluginPermissionIsShell(command.Path) {
		return []string{filepath.Base(command.Path)}, true
	}
	script, ok := shellWrappedScriptArgument(command.Args)
	if !ok {
		return nil, false
	}
	set := map[string]struct{}{}
	if !walkShellScriptCommands(script, set) {
		return nil, false
	}
	if len(set) == 0 {
		return nil, false
	}
	out := make([]string, 0, len(set))
	for name := range set {
		out = append(out, name)
	}
	sort.Strings(out)
	return out, true
}

func pluginPermissionIsShell(path string) bool {
	switch filepath.Base(path) {
	case "sh", "bash", "zsh", "fish":
		return true
	default:
		return false
	}
}

// shellWrappedScriptArgument returns the script argument that follows the
// first -c or -lc flag in args, if any.
func shellWrappedScriptArgument(args []string) (string, bool) {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "-c" || args[i] == "-lc" {
			return args[i+1], true
		}
	}
	return "", false
}

// noForkBuiltins are shell built-ins that do not fork a separate process
// and therefore do not contribute a command to the effective-command set.
var noForkBuiltins = map[string]bool{
	"cd": true, "pushd": true, "popd": true,
	"export": true, "set": true, "unset": true,
	"readonly": true, "local": true, "declare": true, "typeset": true,
	"alias": true, "unalias": true,
	"shift": true, "return": true, "break": true, "continue": true,
	":": true, "true": true, "false": true,
	"exec": true,
}

func walkShellScriptCommands(script string, out map[string]struct{}) bool {
	script = strings.TrimSpace(script)
	if script == "" {
		return false
	}
	file, err := syntax.NewParser().Parse(strings.NewReader(script), "")
	if err != nil || file == nil {
		return false
	}
	for _, stmt := range file.Stmts {
		if !walkStmtCommands(stmt, out) {
			return false
		}
	}
	return true
}

func walkStmtCommands(stmt *syntax.Stmt, out map[string]struct{}) bool {
	if stmt == nil {
		return true
	}
	for _, r := range stmt.Redirs {
		if r.Word != nil {
			if _, ok := shellWrappedLiteralWord(r.Word); !ok {
				return false
			}
		}
		if r.Hdoc != nil {
			if _, ok := shellWrappedLiteralWord(r.Hdoc); !ok {
				return false
			}
		}
	}
	return walkCommandCommands(stmt.Cmd, out)
}

func walkCommandCommands(cmd syntax.Command, out map[string]struct{}) bool {
	switch c := cmd.(type) {
	case *syntax.CallExpr:
		return walkCallExprCommands(c, out)
	case *syntax.BinaryCmd:
		return walkStmtCommands(c.X, out) && walkStmtCommands(c.Y, out)
	case *syntax.Block:
		return walkStmtsCommands(c.Stmts, out)
	case *syntax.Subshell:
		return walkStmtsCommands(c.Stmts, out)
	case *syntax.IfClause:
		for cur := c; cur != nil; cur = cur.Else {
			if !walkStmtsCommands(cur.Cond, out) {
				return false
			}
			if !walkStmtsCommands(cur.Then, out) {
				return false
			}
		}
		return true
	case *syntax.ForClause:
		return walkStmtsCommands(c.Do, out)
	case *syntax.WhileClause:
		return walkStmtsCommands(c.Cond, out) && walkStmtsCommands(c.Do, out)
	case *syntax.CaseClause:
		for _, item := range c.Items {
			if !walkStmtsCommands(item.Stmts, out) {
				return false
			}
		}
		return true
	case *syntax.TimeClause:
		return walkStmtCommands(c.Stmt, out)
	case *syntax.CoprocClause:
		return walkStmtCommands(c.Stmt, out)
	case *syntax.FuncDecl:
		return walkStmtCommands(c.Body, out)
	case *syntax.DeclClause:
		// export/declare/local/readonly/typeset are no-fork built-ins, but
		// their values may contain command substitution. Reject the whole
		// script if any value isn't a pure literal.
		for _, a := range c.Args {
			if a.Value != nil {
				if _, ok := shellWrappedLiteralWord(a.Value); !ok {
					return false
				}
			}
		}
		return true
	case *syntax.LetClause, *syntax.ArithmCmd, *syntax.TestClause:
		// Arithmetic and test expressions don't fork.
		return true
	default:
		return false
	}
}

func walkStmtsCommands(stmts []*syntax.Stmt, out map[string]struct{}) bool {
	for _, s := range stmts {
		if !walkStmtCommands(s, out) {
			return false
		}
	}
	return true
}

func walkCallExprCommands(call *syntax.CallExpr, out map[string]struct{}) bool {
	for _, a := range call.Assigns {
		if a.Value != nil {
			if _, ok := shellWrappedLiteralWord(a.Value); !ok {
				return false
			}
		}
	}
	if len(call.Args) == 0 {
		// Pure environment assignments to the current shell — no fork.
		return true
	}
	fields := make([]string, 0, len(call.Args))
	for _, w := range call.Args {
		lit, ok := shellWrappedLiteralWord(w)
		if !ok {
			return false
		}
		fields = append(fields, lit)
	}
	if fields[0] == "exec" && len(fields) > 1 {
		fields = fields[1:]
	}
	name := fields[0]
	if name == "eval" || name == "source" || name == "." {
		return false
	}
	if noForkBuiltins[name] {
		return true
	}
	if pluginPermissionIsShell(name) {
		// Recurse into nested shell wrappers like bash -c "...".
		script, ok := shellWrappedScriptArgument(fields[1:])
		if !ok {
			return false
		}
		return walkShellScriptCommands(script, out)
	}
	out[filepath.Base(name)] = struct{}{}
	return true
}

func shellWrappedLiteralWord(word *syntax.Word) (string, bool) {
	if word == nil || len(word.Parts) == 0 {
		return "", false
	}
	var b strings.Builder
	for _, part := range word.Parts {
		switch p := part.(type) {
		case *syntax.Lit:
			b.WriteString(p.Value)
		case *syntax.SglQuoted:
			b.WriteString(p.Value)
		case *syntax.DblQuoted:
			inner, ok := shellWrappedLiteralParts(p.Parts)
			if !ok {
				return "", false
			}
			b.WriteString(inner)
		default:
			return "", false
		}
	}
	return b.String(), true
}

func shellWrappedLiteralParts(parts []syntax.WordPart) (string, bool) {
	var b strings.Builder
	for _, part := range parts {
		switch p := part.(type) {
		case *syntax.Lit:
			b.WriteString(p.Value)
		case *syntax.SglQuoted:
			b.WriteString(p.Value)
		default:
			return "", false
		}
	}
	return b.String(), true
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
