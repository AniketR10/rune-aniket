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
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/go-tui/llm/codex"
)

// providersHandler implements the `models providers` subtree.
type providersHandler struct {
	storage storageapi.Service
}

func newProvidersHandler(storage storageapi.Service) *providersHandler {
	return &providersHandler{storage: storage}
}

// HandleCommand satisfies repl.CommandHandler for the providers subtree.
// cmd.Args is the remaining args after the `providers` prefix has been
// stripped by the parent dispatcher.
func (h *providersHandler) HandleCommand(
	ctx context.Context, cmd repl.Command, _ repl.ProgressWriter,
) (iterator.Iterator[component.Responsive], error) {
	if len(cmd.Args) == 0 {
		return nil, errors.New("usage: models providers <codex> <login|status>")
	}
	switch cmd.Args[0] {
	case "codex":
		return h.handleCodex(ctx, cmd.Args[1:])
	default:
		return nil, fmt.Errorf("unknown provider: %s", cmd.Args[0])
	}
}

// Complete satisfies repl.CommandHandler for the providers subtree.
func (h *providersHandler) Complete(
	_ context.Context, _ string, args []string,
) (iterator.Iterator[string], error) {
	switch len(args) {
	case 0:
		return iterator.FromSlice([]string{"codex"}), nil
	case 1:
		return iterator.FromSlice(filterNames([]string{"codex"}, args[0])), nil
	case 2:
		if args[0] != "codex" {
			return iterator.FromSlice[string](nil), nil
		}
		return iterator.FromSlice(filterNames([]string{"login", "status"}, args[1])), nil
	}
	return iterator.FromSlice[string](nil), nil
}

func (h *providersHandler) handleCodex(
	ctx context.Context, args []string,
) (iterator.Iterator[component.Responsive], error) {
	if len(args) == 0 {
		return nil, errors.New("usage: models providers codex <login|status>")
	}
	switch args[0] {
	case "status":
		return h.codexStatus(ctx)
	case "login":
		return h.codexLogin(ctx)
	default:
		return nil, fmt.Errorf("unknown codex subcommand: %s", args[0])
	}
}

func (h *providersHandler) codexStatus(
	ctx context.Context,
) (iterator.Iterator[component.Responsive], error) {
	status, err := codex.Status(ctx, h.storage)
	if err != nil {
		return nil, err
	}
	return markdownOutput(formatCodexStatus(status)), nil
}

func (h *providersHandler) codexLogin(
	ctx context.Context,
) (iterator.Iterator[component.Responsive], error) {
	session, err := codex.StartLogin(ctx, h.storage)
	if err != nil {
		return nil, err
	}
	started := false
	waited := false
	return iterator.FromFunc(
		func(ctx context.Context) (component.Responsive, bool, error) {
			if !started {
				started = true
				return markdownResponsive(formatCodexLoginStart(session)), true, nil
			}
			if waited {
				return nil, false, nil
			}
			waited = true
			cred, err := session.Wait(ctx)
			if err != nil {
				return nil, false, err
			}
			status := codex.AuthStatus{
				Authenticated:   true,
				Email:           cred.Email,
				AccountID:       cred.AccountID,
				PlanType:        cred.PlanType,
				Expiry:          cred.Expiry,
				LastRefresh:     cred.LastRefresh,
				HasRefreshToken: cred.RefreshToken != "",
			}
			return markdownResponsive(formatCodexStatus(status)), true, nil
		},
		session.Close,
	), nil
}

func formatCodexLoginStart(session *codex.LoginSession) string {
	var b strings.Builder
	b.WriteString("Please visit this URL to authenticate with provider.\n\n")
	b.WriteByte('<')
	b.WriteString(session.AuthURL())
	b.WriteString(">\n")
	if err := session.BrowserError(); err != nil {
		fmt.Fprintf(&b, "\nCould not open a browser automatically: `%v`.\n", err)
	}
	return b.String()
}

func formatCodexStatus(status codex.AuthStatus) string {
	var b strings.Builder
	b.WriteString("## Codex Provider\n\n")
	if !status.Authenticated {
		b.WriteString("- **Status**: not authenticated\n\n")
		b.WriteString("Run `models providers codex login` to authenticate.\n")
		return b.String()
	}
	if status.Expired {
		b.WriteString("- **Status**: authenticated credential expired\n")
	} else {
		b.WriteString("- **Status**: authenticated\n")
	}
	if status.Email != "" {
		fmt.Fprintf(&b, "- **Account**: %s\n", status.Email)
	}
	if status.AccountID != "" {
		fmt.Fprintf(&b, "- **Account ID**: `%s`\n", status.AccountID)
	}
	if status.PlanType != "" {
		fmt.Fprintf(&b, "- **Plan**: %s\n", status.PlanType)
	}
	if !status.Expiry.IsZero() {
		fmt.Fprintf(&b, "- **Expires**: %s\n", status.Expiry.Format("2006-01-02T15:04:05Z07:00"))
	}
	if !status.LastRefresh.IsZero() {
		fmt.Fprintf(&b, "- **Last refresh**: %s\n", status.LastRefresh.Format("2006-01-02T15:04:05Z07:00"))
	}
	if !status.HasRefreshToken {
		b.WriteString("- **Refresh token**: missing\n")
	}
	return b.String()
}
