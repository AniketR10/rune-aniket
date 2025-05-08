// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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

package textapi

import (
	"context"

	"github.com/unstablebuild/blue/iterator"
	"unstable.build/go-tui/api/browserapi"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/term"
)

// Command represents a command issued by the user.
type Command struct {
	Name string
	Args []string

	// optional. If command is dispatched while non-tab is in focus,
	// then these fields will be zero-valued.
	URI      workspaceapi.URI
	Resource Handler
	Window   browserapi.Window
	Cursor   struct {
		Content term.Coordinates
		Window  term.Coordinates
	}
}

// CommandHandler is a callback interface that wraps the basic method Command.
type CommandHandler interface {
	// Handle is called when user issued a command previously registered via SubscribeCommand.
	HandleCommand(context.Context, Command) (err error)

	// Complete takes command args and returns a list of expanded options for them.
	// It also returns a expanded version of the last arg, or an empty string
	// if the last arg could/should not be automatically expanded.
	Complete(ctx context.Context, cmd string, args []string) (
		iterator.Iterator[string], error,
	)
}

// CommandManual represents a command's manual and documentation.
type CommandManual struct {
	Name string

	// Summary is a short 80-100 character description.
	Summary string

	// Synopsis is a single line synopsis of how
	// this CLI is to be used. It should ONLY include
	// the semantic information about how arguments are parsed.
	//
	// Example: [<options>] [<revision-range>] [[--] <path>...]
	Synopsis string

	// Commands is a list of accepted commands or nil
	// if no commands are expected.
	Commands []CommandManual
}

// FuncCommandHandler returns an CommandHandler that calls fn
// every time HandleCommand is invoked and completer when
// Complete is invoked.
func FuncCommandHandler(
	fn func(context.Context, Command) error,
	completer func(context.Context, string, []string) (iterator.Iterator[string], error),
) CommandHandler {
	return fnCommandHandler{
		cb:         fn,
		completeFn: completer,
	}
}

// NopCommandCompleter returns an CommandHandler that calls fn
// every time HandleCommand is invoked, but does not have a completion function.
func NopCommandCompleter(
	fn func(context.Context, Command) error,
) CommandHandler {
	return fnCommandHandler{
		cb: fn,
	}
}

type fnCommandHandler struct {
	cb         func(context.Context, Command) error
	completeFn func(context.Context, string, []string) (iterator.Iterator[string], error)
}

func (f fnCommandHandler) HandleCommand(ctx context.Context, c Command) error {
	return f.cb(ctx, c)
}

func (f fnCommandHandler) Complete(ctx context.Context, cmd string, args []string) (
	iterator.Iterator[string], error,
) {
	if f.completeFn != nil {
		return f.completeFn(ctx, cmd, args)
	}
	return iterator.FromSlice[string](nil), nil
}
