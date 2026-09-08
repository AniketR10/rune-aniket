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

package texttest

import (
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/rune/internal/text"
)

// NopWorkspaceRegistry is a text.WorkspaceCommandRegistry that does nothing.
func NopWorkspaceRegistry() text.WorkspaceCommandRegistry {
	return testWorkspaceRegistry{}
}

type testWorkspaceRegistry struct {
}

func (t testWorkspaceRegistry) SubscribeCommandForWorkspace(
	workspace workspaceapi.URI, cmd textapi.CommandManual, handler text.CommandHandler) error {
	return nil
}

func (t testWorkspaceRegistry) UnsubscribeCommandForWorkspace(workspace workspaceapi.URI, name string) error {
	return nil
}
