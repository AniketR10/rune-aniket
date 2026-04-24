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


package ideshell

import (
	"context"

	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/term/sh"
)

// New creates a repl.Handler wired with a CommandRegistry,
// sh layer, and the built-in help command. The returned
// registry can be used to register additional commands.
func New(
	scheduleNextTick func(func()) bool,
	interrupter term.Interrupter,
	opts ...repl.Option,
) (*repl.Handler, *CommandRegistry) {
	r := NewRegistry()
	registerBaseCommands(r)
	h := repl.New(sh.New(r), scheduleNextTick, interrupter, opts...)
	return h, r
}

func registerBaseCommands(r *CommandRegistry) {
	r.Register("help", "Show available commands", &helpHandler{r: r})
}

type helpHandler struct {
	r *CommandRegistry
}

func (h *helpHandler) HandleCommand(
	ctx context.Context, cmd repl.Command, _ repl.ProgressWriter,
) (iterator.Iterator[component.Responsive], error) {
	return h.r.Help(ctx, cmd.Args)
}

func (h *helpHandler) Complete(
	ctx context.Context, _ string, args []string,
) (iterator.Iterator[string], error) {
	if len(args) == 0 {
		return iterator.Empty[string](), nil
	}
	return h.r.Complete(ctx, args[len(args)-1], nil)
}

func (h *helpHandler) Help(
	_ context.Context, _ []string,
) (iterator.Iterator[component.Responsive], error) {
	return toLines(
		"Show available commands or help for a specific command",
	), nil
}

func toLines(ss ...string) iterator.Iterator[component.Responsive] {
	out := make([]component.Responsive, len(ss))
	for i, s := range ss {
		out[i] = toResponsive(s)
	}
	return iterator.FromSlice(out)
}

func toResponsive(s string) component.Responsive {
	return component.NewResponsiveString(
		s, component.StringResponsiveConfig{},
	)
}
