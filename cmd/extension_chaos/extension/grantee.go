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

package extension

import (
	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
)

var (
	// ChaosHandlerCommands returns the commands that this extension is
	// interested in registering.
	ChaosHandlerCommands = []textapi.CommandManual{
		{
			Name: commandChaosHandler,
			Summary: "Creates a new split window with a broken TUI handler." +
				"Two modes can be specified (panic or slow)." +
				"If no argument is passed then 'panic' is assumed.",
			Synopsis: "[panic | slow [<duration>]]",
		},
		{
			Name: commandChaosUpdateEventLatency,
			Summary: "Updates the (added) latency of the event subscriber " +
				"installed by the chaos extension. If no duration is passed, then " +
				"this acts as a reset to zero, which is the default.",
			Synopsis: "[<duration>]",
		},
		{
			Name: commandChaosUpdateCommandLatency,
			Summary: "Updates the (added) latency of the chaos extension's command handler. " +
				"If no duration is passed, then this acts as a reset to zero, " +
				"which is the default. Note that this hinders the ability",
			Synopsis: "[<duration>]",
		},
	}

	// ChaosHandlerEvents returns the events that this extension is
	// interested in subscribing to.
	ChaosHandlerEvents = []textapi.EventType{
		textapi.EventTypeOpen,
		textapi.EventTypeClose,
		textapi.EventTypeEdit,
		textapi.EventTypeFlush,
		textapi.EventTypeScroll,
		textapi.EventTypeHidden,
		textapi.EventTypeVisible,
		textapi.EventTypeFocus,
		textapi.EventTypeUnfocus,
		textapi.EventTypeCursor,
	}
	// ChaosHandlerPermissions are the required permissions for this extension to run.
	// Deprecated: use NewExtension metadata instead.
	ChaosHandlerPermissions = []extensionapi.Permission{
		extensionapi.PermissionBrowserWindowManager,
		extensionapi.PermissionEditor,
		extensionapi.PermissionCommands,
	}
)
