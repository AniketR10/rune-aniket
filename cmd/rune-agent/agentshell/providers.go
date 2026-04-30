// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.
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

package agentshell

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/go-tui/cmd/rune-agent/llm/codex"
)

func (s *shell) handleProviders(
	ctx context.Context, args []string,
) (iterator.Iterator[component.Responsive], error) {
	if len(args) == 0 {
		return nil, errors.New("usage: providers <codex> <login|status>")
	}
	switch args[0] {
	case "codex":
		return s.handleCodexProvider(ctx, args[1:])
	default:
		return nil, fmt.Errorf("unknown provider: %s", args[0])
	}
}

func (s *shell) handleCodexProvider(
	ctx context.Context, args []string,
) (iterator.Iterator[component.Responsive], error) {
	if len(args) == 0 {
		return nil, errors.New("usage: providers codex <login|status>")
	}
	switch args[0] {
	case "login":
		return s.codexLogin(ctx)
	case "status":
		return s.codexStatus(ctx)
	default:
		return nil, fmt.Errorf("unknown codex subcommand: %s", args[0])
	}
}

func (s *shell) codexStatus(ctx context.Context) (iterator.Iterator[component.Responsive], error) {
	status, err := codex.Status(ctx, s.storage)
	if err != nil {
		return nil, err
	}
	return markdownOutput(formatCodexStatus(status)), nil
}

func (s *shell) codexLogin(ctx context.Context) (iterator.Iterator[component.Responsive], error) {
	session, err := codex.StartLogin(ctx, s.storage)
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
		b.WriteString("Run `agent providers codex login` to authenticate.\n")
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
		fmt.Fprintf(&b, "- **Expires**: %s\n", status.Expiry.Format(timeFormatRFC3339()))
	}
	if !status.LastRefresh.IsZero() {
		fmt.Fprintf(&b, "- **Last refresh**: %s\n", status.LastRefresh.Format(timeFormatRFC3339()))
	}
	if !status.HasRefreshToken {
		b.WriteString("- **Refresh token**: missing\n")
	}
	return b.String()
}

func (s *shell) completeProviders(args []string) iterator.Iterator[string] {
	if len(args) == 0 {
		return iterator.FromSlice([]string{"codex"})
	}
	if len(args) == 1 {
		return iterator.FromSlice(filterNames([]string{"codex"}, args[0]))
	}
	if args[0] != "codex" {
		return iterator.FromSlice[string](nil)
	}
	if len(args) == 2 {
		return iterator.FromSlice(filterNames([]string{"login", "status"}, args[1]))
	}
	return iterator.FromSlice[string](nil)
}

func timeFormatRFC3339() string { return "2006-01-02T15:04:05Z07:00" }
