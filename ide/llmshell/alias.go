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

package llmshell

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/go-tui/llm/llmrouter"
)

// reservedAliasNames are the alias names the router recognizes even when
// unset (an unset alias other than `default` resolves to `default`).
// They are always shown by `models alias list`.
var reservedAliasNames = []string{llmapi.DefaultModel, "query", "compact", "dream"}

type aliasHandler struct {
	router *llmrouter.Router
}

func newAliasHandler(router *llmrouter.Router) *aliasHandler {
	return &aliasHandler{router: router}
}

// HandleCommand handles the alias subtree. cmd.Args is the remaining
// args after the `alias` prefix has been stripped by the parent
// dispatcher.
func (h *aliasHandler) HandleCommand(
	ctx context.Context, cmd repl.Command, _ repl.ProgressWriter,
) (iterator.Iterator[component.Responsive], error) {
	if len(cmd.Args) == 0 {
		return h.listAliases(ctx), nil
	}
	switch cmd.Args[0] {
	case "list":
		if len(cmd.Args) != 1 {
			return nil, errors.New("usage: models alias list")
		}
		return h.listAliases(ctx), nil
	case "set":
		return h.handleSet(ctx, cmd.Args[1:])
	case "remove":
		return h.handleRemove(ctx, cmd.Args[1:])
	default:
		if len(cmd.Args) == 2 {
			return h.handleSet(ctx, cmd.Args)
		}
		return nil, fmt.Errorf("unknown alias subcommand: %s", cmd.Args[0])
	}
}

func (h *aliasHandler) Complete(
	ctx context.Context, _ string, args []string,
) (iterator.Iterator[string], error) {
	subs := []string{"list", "set", "remove"}
	switch len(args) {
	case 0:
		return iterator.FromSlice(subs), nil
	case 1:
		return iterator.FromSlice(filterNames(subs, args[0])), nil
	case 2:
		if args[0] == "remove" || args[0] == "set" {
			return iterator.FromSlice(h.completeAliasNames(ctx, args[1])), nil
		}
		// Bare `models alias <name> <provider/model>` form: the second
		// token is the target, not a subcommand.
		return iterator.FromSlice(h.completeModelTargets(ctx, args[1])), nil
	case 3:
		// `models alias set <name> <provider/model>`.
		if args[0] == "set" {
			return iterator.FromSlice(h.completeModelTargets(ctx, args[2])), nil
		}
	}
	return iterator.FromSlice[string](nil), nil
}

func (h *aliasHandler) handleSet(
	ctx context.Context, args []string,
) (iterator.Iterator[component.Responsive], error) {
	if len(args) != 2 {
		return nil, errors.New("usage: models alias set <name> <provider/name>")
	}
	name, target := args[0], args[1]
	if err := h.router.SetAlias(ctx, name, target); err != nil {
		return nil, err
	}
	return markdownOutput(fmt.Sprintf("Set alias `%s` → `%s`", name, target)), nil
}

func (h *aliasHandler) handleRemove(
	ctx context.Context, args []string,
) (iterator.Iterator[component.Responsive], error) {
	if len(args) != 1 {
		return nil, errors.New("usage: models alias remove <name>")
	}
	name := args[0]
	if err := h.router.RemoveAlias(ctx, name); err != nil {
		if errors.Is(err, llmrouter.ErrAliasNotFound) {
			return nil, fmt.Errorf("alias %q is not set", name)
		}
		return nil, err
	}
	return markdownOutput(fmt.Sprintf("Removed alias `%s`", name)), nil
}

func (h *aliasHandler) listAliases(ctx context.Context) iterator.Iterator[component.Responsive] {
	stored, err := h.router.Aliases(ctx)
	if err != nil {
		return markdownOutput(fmt.Sprintf("alias list: %v", err))
	}
	reserved := llmrouter.ReservedAliasNames()
	names := make([]string, 0, len(stored)+len(reserved))
	seen := map[string]bool{}
	for _, n := range reserved {
		names = append(names, n)
		seen[n] = true
	}
	for n := range stored {
		if !seen[n] {
			names = append(names, n)
			seen[n] = true
		}
	}
	sort.Strings(names)

	var b strings.Builder
	b.WriteString("## Model Aliases\n\n")
	for _, name := range names {
		entry, ok := stored[name]
		b.WriteString(h.formatAlias(ctx, name, entry, ok))
	}
	return markdownOutput(b.String())
}

func (h *aliasHandler) formatAlias(
	ctx context.Context, name string, stored llmapi.ModelEntry, ok bool,
) string {
	if ok {
		return fmt.Sprintf("- **%s** → `%s/%s`\n", name, stored.Provider, stored.Name)
	}
	if entry, err := h.router.GetModel(ctx, llmapi.ModelEntry{Name: name}); err == nil {
		return fmt.Sprintf("- **%s** → `%s/%s` *(auto)*\n", name, entry.Provider, entry.Name)
	}
	return fmt.Sprintf("- **%s** → *(unset)*\n", name)
}

func (h *aliasHandler) completeAliasNames(ctx context.Context, prefix string) []string {
	stored, err := h.router.Aliases(ctx)
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var names []string
	for _, n := range reservedAliasNames {
		names = append(names, n)
		seen[n] = true
	}
	for n := range stored {
		if !seen[n] {
			names = append(names, n)
		}
	}
	return filterNames(names, prefix)
}

func (h *aliasHandler) completeModelTargets(ctx context.Context, prefix string) []string {
	it := h.router.Models()
	defer func() { _ = it.Close() }()
	var targets []string
	for {
		entry, ok := it.Next(ctx)
		if !ok {
			break
		}
		if entry.Provider == "" || entry.Name == "" {
			continue
		}
		targets = append(targets, entry.Provider+"/"+entry.Name)
	}
	sort.Strings(targets)
	return filterNames(targets, prefix)
}
