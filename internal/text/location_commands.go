// Copyright (C) 2017-2026 The Rune Authors
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
	"fmt"
	"sort"
	"strings"

	"github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// SubscribeLocationCommands returns a slice of location list related commands
// that can be registered for the returned CommandHandler.
func SubscribeLocationCommands(
	file workspaceapi.URI, registry FileCommandRegistry, ed Handler,
) (Handler, error) {
	ret := locationCommandHandler{
		file:                     file,
		registry:                 registry,
		Handler:                  ed,
		userLocationLists:        make(map[string]struct{}),
		highlightedLocationLists: make(map[string]struct{}),
	}
	var retErr error
	for _, cmd := range locationCommands {
		if err := registry.SubscribeCommandForFile(file, cmd, ret); err != nil {
			retErr = multierror.Append(retErr, err)
		}
	}
	if retErr != nil {
		return nil, retErr
	}
	return ret, nil
}

const (
	// CommandLocationJump is the command name for jumping to locations.
	CommandLocationJump = "jumptolocation"
	// CommandLocationJumpLine is the command name for jumping to the line
	// of a location and positioning at the first non-blank character.
	CommandLocationJumpLine = "jumptolocationline"
	// CommandCreateLocation is the command name for creating locations.
	CommandCreateLocation = "locationcreate"
	// CommandDeleteAllLocations is the command name for deleting all locations on a list.
	CommandDeleteAllLocations = "locationdeleteall"
	// CommandHighlightLocations is the command name for highlighting all locations on a list.
	CommandHighlightLocations = "locationhighlight"
	commandToggleLocation     = "locationtoggle"
	commandDeleteLocation     = "locationdelete"
	defaultUserLocationList   = "mark"
)

var locationCommands = []textapi.CommandManual{
	{
		Name:     CommandLocationJump,
		Summary:  "Jumps to locations on the given location list.",
		Synopsis: "(next|prev) <location-list>",
		Commands: []textapi.CommandManual{
			{
				Name:     "next",
				Summary:  "Jumps to the next location.",
				Synopsis: "<location-list>",
			},
			{
				Name:     "previous",
				Summary:  "Jumps to the previous location.",
				Synopsis: "<location-list>",
			},
		},
	},
	{
		Name:     CommandLocationJumpLine,
		Summary:  "Jumps to the line of a location and positions at the first non-blank character.",
		Synopsis: "(next|prev) <location-list>",
		Commands: []textapi.CommandManual{
			{
				Name:     "next",
				Summary:  "Jumps to the line of the next location.",
				Synopsis: "<location-list>",
			},
			{
				Name:     "previous",
				Summary:  "Jumps to the line of the previous location.",
				Synopsis: "<location-list>",
			},
		},
	},
	{
		Name: CommandCreateLocation,
		Summary: fmt.Sprintf("Saves the current cursor location as a location that can be "+
			"used to jump to via `locationjump %[1]s`. By default, the location list name is `%[1]s` "+
			"but this can be overriden by passing a location list name.",
			defaultUserLocationList),
		Synopsis: "[<location-list>]",
	},
	{
		Name: commandToggleLocation,
		Summary: fmt.Sprintf("Creates or deletes the current cursor location as a location that can be "+
			"used to jump to via `locationjump %[1]s`. By default, the location list name is `%[1]s` "+
			"but this can be overriden by passing a location list name.",
			defaultUserLocationList),
		Synopsis: "[<location-list>]",
	},
	{
		Name: commandDeleteLocation,
		Summary: fmt.Sprintf("Delete the current cursor location in the given location list. "+
			"By default, the location list name is `%[1]s` "+
			"but this can be overriden by passing a location list name.", defaultUserLocationList),
		Synopsis: "[<location-list>]",
	},
	{
		Name: CommandDeleteAllLocations,
		Summary: fmt.Sprintf("Removes all of the locations of the given location list. The default location"+
			" list is `%s`.", defaultUserLocationList),
		Synopsis: "[<location-list>]",
	},
	{
		Name: CommandHighlightLocations,
		Summary: fmt.Sprintf("Highlights all locations of the given user-created location list. The default"+
			" user-created location list is `%s`.", defaultUserLocationList),
		Synopsis: "[<location-list>]",
	},
}

type locationCommandHandler struct {
	file     workspaceapi.URI
	registry FileCommandRegistry
	Handler
	userLocationLists        map[string]struct{}
	highlightedLocationLists map[string]struct{}
}

func (u locationCommandHandler) Close() (ret error) {
	ret = u.Handler.Close()
	for _, cmd := range locationCommands {
		err := u.registry.UnsubscribeCommandForFile(u.file, cmd.Name)
		if err != nil {
			ret = multierror.Append(ret, err)
		}
	}
	return
}

func (u locationCommandHandler) HandleCommand(
	ctx context.Context, cmd textapi.Command,
) (err error) {
	switch cmd.Name {
	case CommandLocationJump:
		err = u.handleLocationJump(cmd)
	case CommandLocationJumpLine:
		err = u.handleLocationJumpLine(cmd)
	case CommandCreateLocation:
		err = u.handleCreateLocation(cmd)
	case commandDeleteLocation:
		err = u.handleDeleteLocation(cmd)
	case commandToggleLocation:
		err = u.handleDeleteLocation(cmd)
		if err != nil {
			err = u.handleCreateLocation(cmd)
		}
	case CommandDeleteAllLocations:
		err = u.handleDeleteAllLocations(cmd)
	case CommandHighlightLocations:
		err = u.handleHighlightLocations(cmd)
	default:
		err = errors.New("extraneous command")
	}
	return
}

func (u locationCommandHandler) Complete(ctx context.Context, cmd textapi.Command) (
	ret iterator.Iterator[string], _ string, err error,
) {
	switch cmd.Name {
	case CommandLocationJump:
		ret, err = u.completeLocationJump(cmd)
	case CommandLocationJumpLine:
		ret, err = u.completeLocationJump(cmd)
	case CommandCreateLocation, commandToggleLocation:
		ret, err = u.completeCreateLocation(cmd)
	case commandDeleteLocation:
		ret, err = u.completeDeleteLocation(cmd)
	case CommandDeleteAllLocations, CommandHighlightLocations:
		ret, err = u.completeCreateLocation(cmd)
	default:
		err = errors.New("extraneous command")
	}
	return
}

func (u locationCommandHandler) handleLocationJump(cmd textapi.Command) error {
	if len(cmd.Args) == 0 {
		return errors.New("missing next/previous and location list id")
	}
	if len(cmd.Args) == 1 {
		return errors.New("missing location list id")
	}
	lists := u.Handler.LocationLists()
	for _, list := range lists {
		if list.ID != cmd.Args[1] {
			continue
		}
		switch cmd.Args[0] {
		case "next":
			ok := u.Handler.MoveToNextLocation(cmd.Args[1])
			if !ok {
				log.Debugf("reached end of location list")
			}
			return nil
		case "previous":
			ok := u.Handler.MoveToPrevLocation(cmd.Args[1])
			if !ok {
				log.Debugf("reached start of location list")
			}
			return nil
		}
		return errors.New("only 'next' or 'previous' is accepted")
	}
	return errors.New("location list does not exist")
}

func (u locationCommandHandler) handleLocationJumpLine(cmd textapi.Command) error {
	if err := u.handleLocationJump(cmd); err != nil {
		return err
	}
	u.moveToFirstNonBlank()
	return nil
}

func (u locationCommandHandler) moveToFirstNonBlank() {
	pos := u.Handler.CursorAtScroll()
	cells := u.Handler.CellView().RawCells()
	if pos.Y < 0 || pos.Y >= len(cells) {
		return
	}
	line := cells[pos.Y]
	for x, c := range line {
		ch := c.Ch
		if ch != 0 && ch != ' ' && ch != '\t' {
			u.Handler.SetCursorAtScroll(term.Coordinates{X: x, Y: pos.Y})
			return
		}
	}
}

func (u locationCommandHandler) completeLocationJump(
	cmd textapi.Command,
) (ret iterator.Iterator[string], err error) {
	if len(cmd.Args) == 1 {
		ret = iterator.FromSlice([]string{"previous", "next"})
		return
	}
	lists := u.Handler.LocationLists()
	if len(cmd.Args) == 2 {
		var ids []string
		for _, list := range lists {
			// lists starting with _ are not displayed.
			// This is useful to hide lists that might
			// be needed for internal implementations.
			if strings.HasPrefix(list.ID, "_") {
				continue
			}
			ids = append(ids, list.ID)
		}
		ret = iterator.FromSlice(ids)
		return
	}
	ret = iterator.Empty[string]()
	return
}

func (u locationCommandHandler) getCursorMark() (term.Coordinates, term.Coordinates) {
	cursor := u.CursorAtScroll()
	return cursor, term.Coordinates{Y: cursor.Y, X: cursor.X + 1}
}

func (u locationCommandHandler) handleDeleteAllLocations(cmd textapi.Command) (err error) {
	list := defaultUserLocationList
	if len(cmd.Args) > 0 {
		list = cmd.Args[0]
	}
	u.Handler.SetLocationList(textapi.LocationPriorityInfo, list, nil)
	clear(u.userLocationLists)
	return
}

func (u locationCommandHandler) handleHighlightLocations(cmd textapi.Command) (err error) {
	list := defaultUserLocationList
	if len(cmd.Args) > 0 {
		list = cmd.Args[0]
	}
	if _, ok := u.highlightedLocationLists[list]; ok {
		u.Handler.SetLocationList(
			textapi.LocationPriorityCritical,
			selectionLocationListID,
			LocationSlice(nil),
		)
		delete(u.highlightedLocationLists, list)
		return nil
	}

	for _, locationList := range u.Handler.LocationLists() {
		if locationList.ID != list {
			continue
		}
		clear(u.highlightedLocationLists)
		locations := make([]textapi.Location, len(locationList.Locations))
		for i, loc := range locationList.Locations {
			loc.Attr = term.Attributes{Attrs: term.AttrReverse}
			locations[i] = loc
		}
		u.Handler.SetLocationList(
			textapi.LocationPriorityCritical,
			selectionLocationListID,
			LocationSlice(locations),
		)
		u.highlightedLocationLists[list] = struct{}{}
		return nil
	}
	return errors.New("location list does not exist")
}

func (u locationCommandHandler) handleDeleteLocation(cmd textapi.Command) (err error) {
	list := defaultUserLocationList
	if len(cmd.Args) > 0 {
		list = cmd.Args[0]
	}
	var curr []textapi.Location
	lists := u.Handler.LocationLists()
	for _, l := range lists {
		if l.ID == list {
			curr = l.Locations
		}
	}
	from, to := u.getCursorMark()
	var success bool
	for i, loc := range curr {
		if loc.From == from && loc.To == to {
			curr[i] = curr[len(curr)-1]
			curr = curr[:len(curr)-1]
			success = true
			break
		}
	}
	if !success {
		return fmt.Errorf("there's no location at the given cursor position for given location list")
	}
	sort.Slice(curr, func(i, j int) bool {
		res := term.CoordinatesDiff(curr[i].From, curr[j].From)
		return res.Y < 0 || res.Y == 0 && res.X < 0
	})
	u.Handler.SetLocationList(textapi.LocationPriorityInfo, list, LocationSlice(curr))
	if list != defaultUserLocationList && len(curr) == 0 {
		delete(u.userLocationLists, list)
	}
	return
}

func (u locationCommandHandler) completeDeleteLocation(
	cmd textapi.Command,
) (ret iterator.Iterator[string], err error) {
	if len(cmd.Args) == 1 {
		lists := u.Handler.LocationLists()
		from, to := u.getCursorMark()
		for _, list := range lists {
			_, isUser := u.userLocationLists[list.ID]
			if !isUser && list.ID != defaultUserLocationList {
				continue
			}
			for _, loc := range list.Locations {
				if loc.From == from && loc.To == to {
					return iterator.FromSlice([]string{list.ID}), nil
				}
			}
		}
	}
	// avoid history completion
	ret = iterator.FromSlice([]string{""})
	return
}

func (u locationCommandHandler) handleCreateLocation(cmd textapi.Command) (err error) {
	list := defaultUserLocationList
	if len(cmd.Args) > 0 {
		list = cmd.Args[0]
	}
	var curr []textapi.Location
	lists := u.Handler.LocationLists()
	for _, l := range lists {
		if l.ID == list {
			curr = l.Locations
		}
	}
	from, to := u.getCursorMark()
	curr = append(curr, textapi.Location{
		From: from,
		To:   to,
		Attr: term.Attributes{
			Bg: term.ColorGray,
		},
	})
	u.Handler.SetLocationList(textapi.LocationPriorityInfo, list, LocationSlice(curr))
	if list != defaultUserLocationList {
		u.userLocationLists[list] = struct{}{}
	}
	return
}

func (u locationCommandHandler) completeCreateLocation(
	cmd textapi.Command,
) (ret iterator.Iterator[string], err error) {
	if len(cmd.Args) == 1 {
		locationLists := []string{defaultUserLocationList}
		for id := range u.userLocationLists {
			locationLists = append(locationLists, id)
		}
		ret = iterator.FromSlice(locationLists)
		return
	}
	ret = iterator.Empty[string]()
	return
}
