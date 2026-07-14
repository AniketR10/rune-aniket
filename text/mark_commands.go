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
	"github.com/unstablebuild/rune-go-sdk/term"
)

// CommandExchangePointAndMark swaps the cursor (point) with the most recent
// mark, matching Emacs exchange-point-and-mark (C-x C-x).
const CommandExchangePointAndMark = "exchangepointandmark"

var markCommands = []textapi.CommandManual{
	{
		Name:     CommandExchangePointAndMark,
		Summary:  "Swap the cursor position with the most recent mark.",
		Synopsis: "",
	},
}

// SubscribeMarkCommands registers the mark-related commands that operate on
// the location list identified by markListID for the given file.
func SubscribeMarkCommands(
	file workspaceapi.URI, registry FileCommandRegistry,
	cursor *Cursor, markListID string, handler Handler,
) (Handler, error) {
	ret := markCommandHandler{
		file:       file,
		cursor:     cursor,
		markListID: markListID,
		registry:   registry,
		Handler:    handler,
	}
	var retErr error
	for _, cmd := range markCommands {
		if err := registry.SubscribeCommandForFile(file, cmd, ret); err != nil {
			retErr = multierror.Append(retErr, err)
		}
	}
	if retErr != nil {
		return nil, retErr
	}
	return ret, nil
}

type markCommandHandler struct {
	Handler
	cursor     *Cursor
	file       workspaceapi.URI
	markListID string
	registry   FileCommandRegistry
}

func (u markCommandHandler) Close() (ret error) {
	ret = u.Handler.Close()
	for _, cmd := range markCommands {
		if err := u.registry.UnsubscribeCommandForFile(u.file, cmd.Name); err != nil {
			ret = multierror.Append(ret, err)
		}
	}
	return
}

func (u markCommandHandler) HandleCommand(
	ctx context.Context, cmd textapi.Command,
) error {
	switch cmd.Name {
	case CommandExchangePointAndMark:
		if !u.exchangePointAndMark() {
			return errors.New("no mark set in this buffer")
		}
		return nil
	default:
		return errors.New("extraneous command")
	}
}

func (u markCommandHandler) exchangePointAndMark() bool {
	locs := u.markLocations()
	if len(locs) == 0 {
		return false
	}
	last := locs[len(locs)-1]
	point := u.cursor.CursorAtScroll()
	if _, ok := u.cursor.MoveToScroll(last.From); !ok {
		return false
	}
	locs[len(locs)-1] = textapi.Location{
		From: point,
		To:   term.Coordinates{Y: point.Y, X: point.X + 1},
		Attr: last.Attr,
	}
	u.cursor.SetLocationList(textapi.LocationPriorityInfo, u.markListID,
		LocationSlice(locs))
	return true
}

func (u markCommandHandler) markLocations() []textapi.Location {
	for _, list := range u.cursor.LocationLists() {
		if list.ID != u.markListID {
			continue
		}
		locs := make([]textapi.Location, len(list.Locations))
		copy(locs, list.Locations)
		return locs
	}
	return nil
}

func (u markCommandHandler) Complete(ctx context.Context, cmd textapi.Command) (
	iterator.Iterator[string], string, error,
) {
	return iterator.Empty[string](), "", nil
}
