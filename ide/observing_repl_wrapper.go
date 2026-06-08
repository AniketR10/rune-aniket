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

	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

// observingREPLHandler decorates a companion-shell REPL handler so that
// every submitted command is reported to the IDE commandObserver. The
// submission is reported uniformly as a "shell" command — matching the
// existing "shell" ex-command observation — with the REPL command name
// prepended to its arguments. This lets tutorials observe companion-shell
// REPL submissions (e.g. "pkg install rune-agent") through the same
// wait_command(command="shell") path used for ex commands.
type observingREPLHandler struct {
	underlying textapi.REPLHandler
	observer   commandObserver
	name       string
}

var _ textapi.REPLHandler = observingREPLHandler{}

// HandleCommand forwards to the underlying handler and reports the
// submission to the observer once the real result is known.
func (w observingREPLHandler) HandleCommand(
	ctx context.Context, cmd repl.Command, pw repl.ProgressWriter,
) (iterator.Iterator[component.Responsive], error) {
	it, err := w.underlying.HandleCommand(ctx, cmd, pw)
	if w.observer != nil {
		args := append([]string{w.name}, cmd.Args...)
		w.observer.observeCommand("shell", "shell", args, err)
	}
	return it, err
}

// Complete forwards completion requests unchanged.
func (w observingREPLHandler) Complete(
	ctx context.Context, cmd string, args []string,
) (iterator.Iterator[string], error) {
	return w.underlying.Complete(ctx, cmd, args)
}

// Help forwards help requests unchanged.
func (w observingREPLHandler) Help(
	ctx context.Context, args []string,
) (iterator.Iterator[component.Responsive], error) {
	return w.underlying.Help(ctx, args)
}
