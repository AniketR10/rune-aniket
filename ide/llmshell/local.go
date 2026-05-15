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
	"os"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/go-tui/llm/llamacpp"
)

// localHandler implements the `models local` subtree.
type localHandler struct {
	registry *llamacpp.Registry
}

func newLocalHandler(reg *llamacpp.Registry) *localHandler {
	return &localHandler{registry: reg}
}

// HandleCommand satisfies repl.CommandHandler for the local subtree.
// cmd.Args is the remaining args after the `local` prefix has been
// stripped by the parent dispatcher.
func (h *localHandler) HandleCommand(
	ctx context.Context, cmd repl.Command, pw repl.ProgressWriter,
) (iterator.Iterator[component.Responsive], error) {
	if len(cmd.Args) == 0 {
		return nil, errors.New("usage: models local <list|download|delete> [args]")
	}
	switch cmd.Args[0] {
	case "list":
		if len(cmd.Args) != 1 {
			return nil, errors.New("usage: models local list")
		}
		return h.listModels(), nil
	case "download":
		return h.handleDownload(ctx, cmd.Args[1:], pw)
	case "delete":
		return h.handleDelete(ctx, cmd.Args[1:])
	default:
		return nil, fmt.Errorf("unknown local subcommand: %s", cmd.Args[0])
	}
}

// Complete satisfies repl.CommandHandler for the local subtree.
func (h *localHandler) Complete(
	_ context.Context, _ string, args []string,
) (iterator.Iterator[string], error) {
	subs := []string{"list", "download", "delete"}
	switch len(args) {
	case 0:
		return iterator.FromSlice(subs), nil
	case 1:
		return iterator.FromSlice(filterNames(subs, args[0])), nil
	case 2:
		if args[0] == "delete" {
			return h.completeDelete(args[1]), nil
		}
	}
	return iterator.FromSlice[string](nil), nil
}

func (h *localHandler) listModels() iterator.Iterator[component.Responsive] {
	refs, err := h.registry.CachedReferences()
	if err != nil {
		return markdownOutput(fmt.Sprintf("local list: %v", err))
	}
	if len(refs) == 0 {
		return markdownOutput("*(no local models downloaded)*")
	}
	var b strings.Builder
	b.WriteString("## Local Models\n\n")
	for _, ref := range refs {
		fmt.Fprintf(&b, "- **%s**\n", ref.String())
	}
	return markdownOutput(b.String())
}

func (h *localHandler) handleDelete(
	ctx context.Context, args []string,
) (iterator.Iterator[component.Responsive], error) {
	if len(args) != 1 {
		return nil, errors.New("usage: models local delete <reference>")
	}
	ref, err := llamacpp.ParseReference(args[0])
	if err != nil {
		return nil, fmt.Errorf("parse reference: %w", err)
	}
	if err := h.registry.Delete(ctx, ref); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%s is not present in the local cache", ref.String())
		}
		return nil, fmt.Errorf("local delete %s: %w", ref.String(), err)
	}
	return markdownOutput(fmt.Sprintf("Deleted `%s` from the local cache", ref.String())), nil
}

func (h *localHandler) completeDelete(prefix string) iterator.Iterator[string] {
	refs, err := h.registry.CachedReferences()
	if err != nil {
		return iterator.FromSlice[string](nil)
	}
	var out []string
	for _, ref := range refs {
		name := ref.String()
		if strings.HasPrefix(name, prefix) {
			out = append(out, name)
		}
	}
	return iterator.FromSlice(out)
}
