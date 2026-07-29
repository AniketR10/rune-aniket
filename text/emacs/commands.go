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

package emacs

import (
	"context"
	"errors"

	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/text"
)

// CommandUndo reverses the most recent buffer change.
const CommandUndo = "undo"

const undoPrefixArg = "prefix"

var undoCommand = textapi.CommandManual{
	Name:    CommandUndo,
	Summary: "Undo the most recent buffer change.",
}

func subscribeUndoCommand(
	file workspaceapi.URI, registry text.FileCommandRegistry,
	h *emacsHandler, wrapped text.Handler,
) (text.Handler, error) {
	ret := undoCommandHandler{
		Handler:  wrapped,
		h:        h,
		file:     file,
		registry: registry,
	}
	if err := registry.SubscribeCommandForFile(file, undoCommand, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

type undoCommandHandler struct {
	text.Handler
	h        *emacsHandler
	file     workspaceapi.URI
	registry text.FileCommandRegistry
}

func (h undoCommandHandler) Close() (ret error) {
	ret = h.Handler.Close()
	if err := h.registry.UnsubscribeCommandForFile(h.file, CommandUndo); err != nil {
		return errors.Join(ret, err)
	}
	return ret
}

func (h undoCommandHandler) HandleCommand(
	_ context.Context, cmd textapi.Command,
) error {
	if len(cmd.Args) == 1 && cmd.Args[0] == undoPrefixArg {
		h.h.undoFromPrefix()
		return nil
	}
	h.h.undoPrefix = nil
	h.h.undo()
	return nil
}

func (h undoCommandHandler) Complete(context.Context, textapi.Command) (
	iterator.Iterator[string], string, error,
) {
	return iterator.Empty[string](), "", nil
}
