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

// Package loginshell exposes the `login` and `logout` REPL commands in
// the IDE shell. login streams the OAuth authorization URL inline as a
// markdown component and then reports the result, replacing the
// notification-based feedback used by the old command-prompt commands.
package loginshell

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/unstablebuild/ox-api/auth"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/go-tui/cmd/rune/ide/apiclient"
	"unstable.build/go-tui/component/markdown"
)

// Client is the subset of *apiclient.Client the login shell needs.
type Client interface {
	Login(ctx context.Context) apiclient.LoginSession
	Logout(ctx context.Context) error
	AccountStatus(ctx context.Context) (auth.RPCUser, bool, error)
}

const (
	loginSummary  = "Authenticate your Rune client."
	logoutSummary = "Log out from the current session so you can login with a different account."
)

// Login returns the command manual and REPL handler for the `login`
// command, which streams the OAuth authorization URL inline and then
// reports the result.
func Login(client Client) (textapi.CommandManual, textapi.REPLHandler) {
	return textapi.CommandManual{Name: "login", Summary: loginSummary},
		loginHandler{client: client}
}

// Logout returns the command manual and REPL handler for the `logout`
// command, which clears the current authentication credentials.
func Logout(client Client) (textapi.CommandManual, textapi.REPLHandler) {
	return textapi.CommandManual{Name: "logout", Summary: logoutSummary},
		logoutHandler{client: client}
}

type loginHandler struct {
	client Client
}

func (h loginHandler) HandleCommand(
	ctx context.Context, _ repl.Command, _ repl.ProgressWriter,
) (iterator.Iterator[component.Responsive], error) {
	session := h.client.Login(ctx)
	urlEmitted := false
	waited := false
	return iterator.FromFunc(
		func(ctx context.Context) (component.Responsive, bool, error) {
			if !urlEmitted {
				urlEmitted = true
				select {
				case <-ctx.Done():
					return nil, false, ctx.Err()
				case u, ok := <-session.URL:
					if ok && u != nil {
						return markdownResponsive(formatLoginStart(u)), true, nil
					}
				}
			}
			if waited {
				return nil, false, nil
			}
			waited = true
			select {
			case <-ctx.Done():
				return nil, false, ctx.Err()
			case err := <-session.Done:
				if err != nil {
					return markdownResponsive(
						fmt.Sprintf("Login did not complete: `%v`.", err)), true, nil
				}
				return markdownResponsive(h.loginSuccess(ctx)), true, nil
			}
		},
		func() error { return nil },
	), nil
}

func (h loginHandler) loginSuccess(ctx context.Context) string {
	user, ok, err := h.client.AccountStatus(ctx)
	if err != nil || !ok {
		return "Login successful."
	}
	return formatAccountStatus(user)
}

func (loginHandler) Complete(
	context.Context, string, []string,
) (iterator.Iterator[string], error) {
	return iterator.Empty[string](), nil
}

func (loginHandler) Help(
	context.Context, []string,
) (iterator.Iterator[component.Responsive], error) {
	return markdownOutput(loginSummary), nil
}

type logoutHandler struct {
	client Client
}

func (h logoutHandler) HandleCommand(
	ctx context.Context, _ repl.Command, _ repl.ProgressWriter,
) (iterator.Iterator[component.Responsive], error) {
	if err := h.client.Logout(ctx); err != nil {
		return nil, err
	}
	return markdownOutput("Logged out."), nil
}

func (logoutHandler) Complete(
	context.Context, string, []string,
) (iterator.Iterator[string], error) {
	return iterator.Empty[string](), nil
}

func (logoutHandler) Help(
	context.Context, []string,
) (iterator.Iterator[component.Responsive], error) {
	return markdownOutput(logoutSummary), nil
}

func formatLoginStart(u *url.URL) string {
	var b strings.Builder
	b.WriteString("Open this URL in your browser to complete login:\n\n")
	b.WriteByte('<')
	b.WriteString(u.String())
	b.WriteString(">\n")
	return b.String()
}

func formatAccountStatus(u auth.RPCUser) string {
	var b strings.Builder
	b.WriteString("## Login successful\n\n")
	if u.Email != "" {
		fmt.Fprintf(&b, "- **Account**: `%s`\n", u.Email)
	}
	fmt.Fprintf(&b, "- **Plan**: %s\n", planLabel(u.Role))
	if u.Role == auth.RoleOneOff && !u.PlanEnds.IsZero() {
		fmt.Fprintf(&b, "- **Upgrades covered through**: %s\n",
			u.PlanEnds.Format("2006-01-02"))
	} else if u.Role >= auth.RolePaid && !u.PlanEnds.IsZero() {
		fmt.Fprintf(&b, "- **Renews**: %s\n", u.PlanEnds.Format("2006-01-02"))
	}
	if u.Role != auth.RoleOneOff && u.Role < auth.RolePaid {
		b.WriteString("\nUpgrade to a paid plan to unlock Rune.\n")
	}
	return b.String()
}

func planLabel(role auth.Role) string {
	switch {
	case role == auth.RoleOneOff:
		return "One-off"
	case role >= auth.RolePaid:
		return "Paid"
	default:
		return "Free"
	}
}

func markdownOutput(content string) iterator.Iterator[component.Responsive] {
	md, err := markdown.New(content)
	if err != nil {
		r := component.NewResponsiveString(content, component.StringResponsiveConfig{})
		return iterator.FromSlice([]component.Responsive{r})
	}
	return iterator.FromSlice([]component.Responsive{md})
}

func markdownResponsive(content string) component.Responsive {
	it := markdownOutput(content)
	v, _ := it.Next(context.Background())
	return v
}
