// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

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
