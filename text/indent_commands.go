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

package text

import (
	"context"
	"errors"

	"github.com/ernestrc/go-multierror"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

// CommandReindent is the command name for reindenting the current line or selection.
const CommandReindent = "reindent"

var indentCommands = []textapi.CommandManual{{
	Name:     CommandReindent,
	Summary:  "Reindents the current line or the active selection using the syntax tree.",
	Synopsis: "",
}}

// SubscribeIndentCommands returns indent-related commands that can be registered
// for the returned Handler.
func SubscribeIndentCommands(
	file workspaceapi.URI, registry FileCommandRegistry,
	cursor *Cursor, handler Handler,
) (Handler, error) {
	ret := indentCommandHandler{
		file:     file,
		cursor:   cursor,
		registry: registry,
		Handler:  handler,
	}
	var retErr error
	for _, cmd := range indentCommands {
		if err := registry.SubscribeCommandForFile(file, cmd, ret); err != nil {
			retErr = multierror.Append(retErr, err)
		}
	}
	if retErr != nil {
		return nil, retErr
	}
	return ret, nil
}

type indentCommandHandler struct {
	Handler
	cursor   *Cursor
	file     workspaceapi.URI
	registry FileCommandRegistry
}

func (u indentCommandHandler) Close() (ret error) {
	ret = u.Handler.Close()
	for _, cmd := range indentCommands {
		err := u.registry.UnsubscribeCommandForFile(u.file, cmd.Name)
		if err != nil {
			ret = multierror.Append(ret, err)
		}
	}
	return
}

func (u indentCommandHandler) HandleCommand(
	ctx context.Context, cmd textapi.Command,
) (err error) {
	switch cmd.Name {
	case CommandReindent:
		if _, ok := u.cursor.SelectionMode(); ok {
			u.cursor.ReindentSelection()
			return nil
		}
		if !u.cursor.Reindent() {
			return errors.New("auto-indentation not available at the current position")
		}
		return nil
	default:
		return errors.New("extraneous command")
	}
}

func (u indentCommandHandler) Complete(ctx context.Context, cmd textapi.Command) (
	ret iterator.Iterator[string], _ string, err error,
) {
	ret = iterator.Empty[string]()
	return
}
