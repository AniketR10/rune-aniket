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
	"sort"
	"strings"
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/go-tui/text"
)

const (
	authorizerREPLCommand       = "authorizer"
	authorizerREPLCommandList   = "list"
	authorizerREPLCommandRevoke = "revoke"
)

func registerAuthorizerREPLCommand(
	editor text.Editor, authorizer *Authorizer,
) error {
	return editor.RegisterREPLCommand(textapi.CommandManual{
		Name:     authorizerREPLCommand,
		Summary:  "Manage persisted plugin authorizer decisions.",
		Synopsis: "<list|revoke> [permission]",
		Commands: []textapi.CommandManual{
			{
				Name:    authorizerREPLCommandList,
				Summary: "List persisted plugin permission decisions.",
			},
			{
				Name:     authorizerREPLCommandRevoke,
				Summary:  "Revoke a persisted plugin permission decision.",
				Synopsis: "<permission>",
			},
		},
	}, authorizerREPLHandler{authorizer: authorizer})
}

type authorizerREPLHandler struct {
	authorizer *Authorizer
}

func (h authorizerREPLHandler) HandleCommand(
	ctx context.Context, cmd repl.Command, _ repl.ProgressWriter,
) (iterator.Iterator[component.Responsive], error) {
	if len(cmd.Args) == 0 {
		return authorizerREPLHelp(), nil
	}
	switch cmd.Args[0] {
	case authorizerREPLCommandList:
		return h.handleList(ctx)
	case authorizerREPLCommandRevoke:
		return h.handleRevoke(ctx, cmd.Args[1:])
	default:
		return nil, fmt.Errorf("unknown authorizer subcommand %q", cmd.Args[0])
	}
}

func (h authorizerREPLHandler) Complete(
	ctx context.Context, _ string, args []string,
) (iterator.Iterator[string], error) {
	if len(args) == 0 {
		return iterator.FromSlice([]string{
			authorizerREPLCommandList,
			authorizerREPLCommandRevoke,
		}), nil
	}
	if len(args) == 1 {
		var ret []string
		for _, subcommand := range []string{
			authorizerREPLCommandList,
			authorizerREPLCommandRevoke,
		} {
			if strings.HasPrefix(subcommand, args[0]) {
				ret = append(ret, subcommand)
			}
		}
		return iterator.FromSlice(ret), nil
	}
	if args[0] != authorizerREPLCommandRevoke {
		return iterator.Empty[string](), nil
	}
	entries, err := h.authorizer.storedDecisions(ctx)
	if err != nil {
		return nil, err
	}
	prefix := args[len(args)-1]
	var ret []string
	for _, entry := range entries {
		id := entry.ID()
		if strings.HasPrefix(id, prefix) || strings.Contains(id, ":"+prefix) ||
			strings.HasPrefix(entry.Key, prefix) {
			ret = append(ret, id)
		}
	}
	return iterator.FromSlice(ret), nil
}

func (h authorizerREPLHandler) Help(
	context.Context, []string,
) (iterator.Iterator[component.Responsive], error) {
	return authorizerREPLHelp(), nil
}

func (h authorizerREPLHandler) handleList(
	ctx context.Context,
) (iterator.Iterator[component.Responsive], error) {
	entries, err := h.authorizer.permissionDecisions(ctx, time.Now())
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return authorizerREPLLines("No extension permission decisions."), nil
	}
	lines := make([]string, 0, len(entries)+1)
	lines = append(lines, "Extension permission decisions:")
	for _, entry := range entries {
		lines = append(lines, fmt.Sprintf("- %s — %s", entry.ID(), entry.Display()))
	}
	return authorizerREPLLines(lines...), nil
}

func (h authorizerREPLHandler) handleRevoke(
	ctx context.Context, args []string,
) (iterator.Iterator[component.Responsive], error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("expected permission to revoke")
	}
	selected := strings.Join(args, " ")
	entry, ok, err := h.authorizer.findStoredDecision(ctx, selected)
	if err != nil {
		return nil, err
	}
	if !ok {
		if isPermissionStorageKey(selected) {
			if err := h.authorizer.deleteStoredDecision(ctx, selected); err != nil {
				return nil, err
			}
			return authorizerREPLLines(fmt.Sprintf("Revoked %s", selected)), nil
		}
		return nil, fmt.Errorf("plugin permission decision not found: %s", selected)
	}
	if err := h.authorizer.deleteStoredDecision(ctx, entry.Key); err != nil {
		return nil, err
	}
	return authorizerREPLLines(fmt.Sprintf("Revoked %s", entry.Display())), nil
}

func authorizerREPLHelp() iterator.Iterator[component.Responsive] {
	return authorizerREPLLines(
		"Usage: authorizer <list|revoke>",
		"Lists or revokes persisted extension permission decisions.",
	)
}

func authorizerREPLLines(lines ...string) iterator.Iterator[component.Responsive] {
	out := make([]component.Responsive, len(lines))
	for i, line := range lines {
		out[i] = component.NewResponsiveString(line, component.StringResponsiveConfig{})
	}
	return iterator.FromSlice(out)
}

type pluginPermissionStoredDecisionEntry struct {
	Key        string
	Decision   string
	Path       string
	Args       []string
	Permission string
	Scope      string
	Expires    time.Time
}

func (e pluginPermissionStoredDecisionEntry) Display() string {
	if e.Path == "" || e.Permission == "" {
		return e.Key
	}
	scope := e.Scope
	if scope == "" {
		scope = "persisted"
	}
	ret := fmt.Sprintf("%s %s %s %s %v", scope, e.Decision, e.Permission, e.Path, e.Args)
	if !e.Expires.IsZero() {
		ret += fmt.Sprintf(" expires %s", e.Expires.Format(time.RFC3339))
	}
	return ret
}

func (e pluginPermissionStoredDecisionEntry) ID() string {
	if e.Key == "" {
		return ""
	}
	scope := e.Scope
	if scope == "" {
		scope = "persisted"
	}
	parts := []string{
		sanitizePluginPermissionIDPart(scope),
		sanitizePluginPermissionIDPart(e.Decision),
		sanitizePluginPermissionIDPart(e.Permission),
		sanitizePluginPermissionIDPart(programBaseName(e.Path)),
		shortPluginPermissionKey(e.Key),
	}
	return strings.Join(parts, ":")
}

func sanitizePluginPermissionIDPart(part string) string {
	part = strings.ToLower(part)
	var b strings.Builder
	for _, r := range part {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	ret := strings.Trim(b.String(), "-")
	if ret == "" {
		return "unknown"
	}
	return ret
}

func programBaseName(path string) string {
	path = strings.TrimRight(path, "/")
	if path == "" {
		return "unknown"
	}
	idx := strings.LastIndex(path, "/")
	if idx == -1 {
		return path
	}
	return path[idx+1:]
}

func shortPluginPermissionKey(key string) string {
	parts := strings.Split(key, ":")
	if len(parts) >= 3 && parts[2] != "" {
		if len(parts[2]) <= 8 {
			return parts[2]
		}
		return parts[2][:8]
	}
	if len(key) <= 8 {
		return sanitizePluginPermissionIDPart(key)
	}
	return sanitizePluginPermissionIDPart(key[:8])
}

func (a *Authorizer) permissionDecisions(
	ctx context.Context, now time.Time,
) ([]pluginPermissionStoredDecisionEntry, error) {
	entries, err := a.storedDecisions(ctx)
	if err != nil {
		return nil, err
	}
	entries = append(entries, a.transientDecisions(now)...)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Display() < entries[j].Display()
	})
	return entries, nil
}

func (a *Authorizer) storedDecisions(
	ctx context.Context,
) ([]pluginPermissionStoredDecisionEntry, error) {
	if a == nil || a.storage == nil {
		return nil, nil
	}
	it, err := a.storage.List(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("list plugin permission decisions: %w", err)
	}
	defer it.Close()
	var ret []pluginPermissionStoredDecisionEntry
	for it.HasNext() {
		var stored storedPermissionDecision
		if err := it.NextTo(&stored); err != nil {
			return nil, fmt.Errorf("read plugin permission decision: %w", err)
		}
		if stored.Key == "" || !isPermissionStorageKey(stored.Key) {
			continue
		}
		ret = append(ret, pluginPermissionStoredDecisionEntry{
			Key:        stored.Key,
			Decision:   stored.Decision,
			Path:       stored.Path,
			Args:       append([]string(nil), stored.Args...),
			Permission: string(stored.Permission),
			Scope:      "persisted",
		})
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Display() < ret[j].Display()
	})
	return ret, nil
}

func (a *Authorizer) transientDecisions(
	now time.Time,
) []pluginPermissionStoredDecisionEntry {
	if a == nil {
		return nil
	}
	a.onceMu.Lock()
	defer a.onceMu.Unlock()
	a.purgeExpiredOnceDecisionsLocked(now)
	ret := make([]pluginPermissionStoredDecisionEntry, 0, len(a.once))
	for key, decision := range a.once {
		ret = append(ret, pluginPermissionStoredDecisionEntry{
			Key:        key,
			Decision:   decision.Decision,
			Path:       decision.Path,
			Args:       append([]string(nil), decision.Args...),
			Permission: string(decision.Permission),
			Scope:      "transient",
			Expires:    decision.Expires,
		})
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Display() < ret[j].Display()
	})
	return ret
}

func (a *Authorizer) findStoredDecision(
	ctx context.Context, selected string,
) (pluginPermissionStoredDecisionEntry, bool, error) {
	entries, err := a.storedDecisions(ctx)
	if err != nil {
		return pluginPermissionStoredDecisionEntry{}, false, err
	}
	for _, entry := range entries {
		if selected == entry.Key || selected == entry.ID() || selected == entry.Display() {
			return entry, true, nil
		}
	}
	return pluginPermissionStoredDecisionEntry{}, false, nil
}

func isPermissionStorageKey(key string) bool {
	return strings.HasPrefix(key, pluginPermissionStoragePrefix) ||
		strings.HasPrefix(key, extensionPermissionStoragePrefix)
}

func (a *Authorizer) deleteStoredDecision(
	ctx context.Context, key string,
) error {
	if a == nil || a.storage == nil {
		return nil
	}
	if err := a.storage.Delete(ctx, key); err != nil {
		return fmt.Errorf("delete plugin permission decision: %w", err)
	}
	return nil
}
