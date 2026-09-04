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

package main

import (
	_ "embed"

	"unstable.build/rune/ide"
)

//go:embed tutorials/basics.star
var basicsTutorial string

//go:embed tutorials/navigation.star
var navigationTutorial string

//go:embed tutorials/agent.star
var agentTutorial string

var embeddedTutorialPlaylist = []ide.TutorialPlaylistItem{
	{
		Name:        "basics",
		Description: "Learn the essential Rune workspace and window management commands and key bindings.",
	},
	{
		Name:        "navigation",
		Description: "Learn about structural navigation and how to exploit Rune's code intelligence tools.",
	},
	{
		Name:        "agent",
		Description: "Install Rune Agent, connect a model provider, start a conversation, and get help.",
	},
}

func embeddedTutorialOptions() []ide.Option {
	return []ide.Option{
		ide.WithStarlarkTutorial("basics", basicsTutorial),
		ide.WithStarlarkTutorial("navigation", navigationTutorial),
		ide.WithStarlarkTutorial("agent", agentTutorial),
		ide.WithTutorialPlaylist(embeddedTutorialPlaylist...),
	}
}
