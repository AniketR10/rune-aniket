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

package ide

import (
	"context"
	"errors"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"unstable.build/go-tui/handler/finder"
	"unstable.build/go-tui/handler/locationsearch"
	"unstable.build/go-tui/text/cmdenv"
)

// locationpicker runs the given program via $SHELL -c and presents
// each stdout line as an entry in a floating fuzzy-search picker
// with a syntax-highlighted preview pane on top. Lines must follow
// the `path[:line[:col]]` convention; selecting an entry opens the
// file in the previously focused window at the parsed coordinates.
func (e *ex) locationpicker(ctx context.Context, args ...string) error {
	if len(args) == 0 {
		return errors.New("expected at least one argument")
	}
	// args reach this handler already unquoted by the command prompt
	// (and stripped of any alias-substituted quoting via
	// regroupAndUnquote in text/component.go). Re-shell-quote each
	// token so that joining them with spaces and handing the result
	// to `$SHELL -c` recovers the original argument boundaries even
	// when individual values contain whitespace, single quotes,
	// regex metacharacters, or shell metacharacters. Without this
	// round-trip, an alias body like
	//   "locationpicker git grep -n --column $1"
	// invoked as `:gitgrep 'foo bar'` would reach the shell as
	// `git grep -n --column foo bar`, splitting the user's
	// single-token query into two grep arguments.
	cmdStr := strings.Join(cmdenv.QuoteArgsForShellFields(args), " ")

	prevWin := e.invokeWindow()
	apibrowser := newBrowserAdapter(e.Browser())
	clients := finder.Clients{
		Storage:        e.storage,
		ResourceOpener: apibrowser,
		WindowManager:  apibrowser,
		Interrupter:    apibrowser,
		Notifications:  apibrowser,
		// Wrap the text.Component-backed Editor (rather than the
		// user-supplied text.Editor like vi.Editor()) so that
		// finder.setContent can resolve the just-opened tab's
		// text.Handler via Editor(uri) and dispatch SetCursor to
		// it. The user-level text.Editor's Editor(uri) is "not
		// supported", which silently broke cursor dispatch on
		// :gitgrep.
		Editor:     newEditorAdapter(&e.comp),
		FileSystem: e.workspace,
		Executor:   workspaceExecutorAdapter{e: e.executor},
	}

	cfg := locationsearch.DefaultConfig()
	cfg.HistoryDocumentID = "locationpicker.history"
	h, err := locationsearch.New(
		ctx, clients, prevWin,
		cfg,
		e.parser,
		e.emulatorConfig.ScheduleNextTick,
		cmdStr,
		nil,
	)
	if err != nil {
		return err
	}

	win, err := e.Browser().Floating(h, browserapi.FloatingConfig{
		Alignment: component.AlignmentCentered,
	})
	if err != nil {
		_ = h.Close()
		return err
	}
	h.SetWindow(win)
	return nil
}
