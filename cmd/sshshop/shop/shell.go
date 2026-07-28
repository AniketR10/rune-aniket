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

package shop

import (
	"context"
	"fmt"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"

	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/ide/ideshell"
)

func (r *Root) mustInitShellTab() {
	if r.shellTab != nil {
		return
	}
	registry := ideshell.NewRegistry()
	registry.Register("help", "Show available commands", shopHelpHandler{r: registry})
	r.registerShellCommands(registry)
	h := repl.New(registry, r.scheduleNextTick, r.interrupter,
		repl.WithPrompt("shop> "))
	r.shell = h
	r.shellHelp = registry
	uri := mustParseURI("shell:///shop")
	r.shellTab = r.b.NewTab(uri, '>', shellTabName, browser.NopHandler(h), nil)
}

type shopHelpHandler struct {
	r *ideshell.CommandRegistry
}

func (h shopHelpHandler) HandleCommand(
	ctx context.Context, cmd repl.Command, _ repl.ProgressWriter,
) (iterator.Iterator[component.Responsive], error) {
	return h.r.Help(ctx, cmd.Args)
}

func (h shopHelpHandler) Complete(
	ctx context.Context, _ string, args []string,
) (iterator.Iterator[string], error) {
	if len(args) == 0 {
		return iterator.Empty[string](), nil
	}
	return h.r.Complete(ctx, args[len(args)-1], nil)
}

func (h shopHelpHandler) Help(
	_ context.Context, _ []string,
) (iterator.Iterator[component.Responsive], error) {
	return newStaticShellCommand(
		"Help",
		"Show available commands or help for a specific command.",
	).HandleCommand(context.Background(), repl.Command{}, repl.NopProgressWriter())
}

func (r *Root) registerShellCommands(reg *ideshell.CommandRegistry) {
	reg.Register("download", "Download the sshshop CLI", newStaticShellCommand(
		"Download",
		"A dedicated sshshop CLI is coming soon.",
		"For now, stay in the SSH session and browse the storefront directly.",
	))
	reg.Register("sign-up", "Create a new account", newStaticShellCommand(
		"Sign up",
		"Account creation will eventually prompt for an email and send a magic link.",
		"For now this is a placeholder flow.",
	))
	reg.Register("sign-in", "Sign into an existing account", newStaticShellCommand(
		"Sign in",
		"Existing-account login will eventually ask for your email and verify a magic-link code.",
		"For now this is a placeholder flow.",
	))
}

type staticShellCommand string

func (s staticShellCommand) HandleCommand(
	_ context.Context, _ repl.Command, _ repl.ProgressWriter,
) (iterator.Iterator[component.Responsive], error) {
	parts := strings.Split(string(s), "\n")
	ret := make([]component.Responsive, 0, len(parts))
	for _, p := range parts {
		ret = append(ret, component.NewResponsiveString(p, component.StringResponsiveConfig{}))
	}
	return iterator.FromSlice(ret), nil
}

func (s staticShellCommand) Complete(
	_ context.Context, _ string, _ []string,
) (iterator.Iterator[string], error) {
	return iterator.Empty[string](), nil
}

func (s staticShellCommand) Help(
	_ context.Context, _ []string,
) (iterator.Iterator[component.Responsive], error) {
	return s.HandleCommand(context.Background(), repl.Command{}, repl.NopProgressWriter())
}

func shellHelpText(title string, lines ...string) string {
	all := []string{title}
	all = append(all, lines...)
	return strings.Join(all, "\n")
}

func newStaticShellCommand(title string, lines ...string) staticShellCommand {
	return staticShellCommand(shellHelpText(title, lines...))
}

func (r *Root) showShell() error {
	if r.shellTab == nil {
		return fmt.Errorf("shell unavailable")
	}
	return r.invokeWindow().SetContent(r.shellTab)
}
