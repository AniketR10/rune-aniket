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

package vctrlcmd

import (
	"context"
	"errors"

	"github.com/ernestrc/go-multierror"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/clipboard"
	"unstable.build/rune/internal/ide/vctrl"
	"unstable.build/rune/internal/text"
)

// SubscribeGitCommands returns a slice of git list related commands
// that can be registered for the returned CommandHandler.
func SubscribeGitCommands(
	file workspaceapi.URI, registry text.FileCommandRegistry, ed text.Handler,
	svc vctrl.Service, clip clipboard.Register, noti browserapi.Notifications,
) (text.Handler, error) {
	ret := gitCommandHandler{
		file:     file,
		registry: registry,
		Handler:  ed,
		webLink:  newCopyRemoteURL(svc, clip, noti),
	}
	var retErr error
	for _, cmd := range gitCommands {
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
	commandGitLink = "gitlink"
)

var gitCommands = []textapi.CommandManual{
	{
		Name:     commandGitLink,
		Summary:  "Copies to clipboard the web permalink of the line at the cursor.",
		Synopsis: "[<remote>]",
	},
}

type gitCommandHandler struct {
	text.Handler
	file     workspaceapi.URI
	registry text.FileCommandRegistry
	webLink  text.CommandHandler
}

func (u gitCommandHandler) HandleCommand(
	ctx context.Context, cmd textapi.Command,
) (err error) {
	switch cmd.Name {
	case commandGitLink:
		err = u.webLink.HandleCommand(ctx, cmd)
	default:
		err = errors.New("extraneous command")
	}
	return
}

func (u gitCommandHandler) Complete(ctx context.Context, cmd textapi.Command) (
	ret iterator.Iterator[string], arg0 string, err error,
) {
	switch cmd.Name {
	case commandGitLink:
		ret, arg0, err = u.webLink.Complete(ctx, cmd)
	default:
		err = errors.New("extraneous command")
	}
	return
}

func (g gitCommandHandler) Close() (ret error) {
	ret = g.Handler.Close()
	for _, cmd := range gitCommands {
		err := g.registry.UnsubscribeCommandForFile(g.file, cmd.Name)
		if err != nil {
			ret = multierror.Append(ret, err)
		}
	}
	return
}
