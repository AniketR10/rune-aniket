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

// CommandToggleLineComment toggles the configured line-comment prefix on the
// current line or selected lines.
const CommandToggleLineComment = "commentlinetoggle"

// CommandToggleBlockComment toggles the configured block-comment delimiters
// around the current selection or block under the cursor.
const CommandToggleBlockComment = "commentblocktoggle"

var commentCommands = []textapi.CommandManual{
	{
		Name:     CommandToggleLineComment,
		Summary:  "Toggles the line comment prefix on the current line or selection.",
		Synopsis: "",
	},
	{
		Name:     CommandToggleBlockComment,
		Summary:  "Toggles block comment delimiters around the current selection.",
		Synopsis: "",
	},
}

// SubscribeCommentCommands returns comment-related commands that can be
// registered for the returned Handler.
func SubscribeCommentCommands(
	file workspaceapi.URI, registry FileCommandRegistry,
	cursor *Cursor, handler Handler,
) (Handler, error) {
	ret := commentCommandHandler{
		file:     file,
		cursor:   cursor,
		registry: registry,
		Handler:  handler,
	}
	var retErr error
	for _, cmd := range commentCommands {
		if err := registry.SubscribeCommandForFile(file, cmd, ret); err != nil {
			retErr = multierror.Append(retErr, err)
		}
	}
	if retErr != nil {
		return nil, retErr
	}
	return ret, nil
}

type commentCommandHandler struct {
	Handler
	cursor   *Cursor
	file     workspaceapi.URI
	registry FileCommandRegistry
}

func (u commentCommandHandler) Close() (ret error) {
	ret = u.Handler.Close()
	for _, cmd := range commentCommands {
		err := u.registry.UnsubscribeCommandForFile(u.file, cmd.Name)
		if err != nil {
			ret = multierror.Append(ret, err)
		}
	}
	return
}

func (u commentCommandHandler) HandleCommand(
	ctx context.Context, cmd textapi.Command,
) (err error) {
	switch cmd.Name {
	case CommandToggleLineComment:
		if !u.cursor.ToggleLineComment() {
			return errors.New("line comment not configured for this file")
		}
		return nil
	case CommandToggleBlockComment:
		if !u.cursor.ToggleBlockComment() {
			return errors.New("block comment not available at the current position")
		}
		return nil
	default:
		return errors.New("extraneous command")
	}
}

func (u commentCommandHandler) Complete(ctx context.Context, cmd textapi.Command) (
	ret iterator.Iterator[string], _ string, err error,
) {
	ret = iterator.Empty[string]()
	return
}
