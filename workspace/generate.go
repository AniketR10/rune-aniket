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

package workspace

//go:generate mockgen -destination=./workspacetest/workspace_gomock.go -package workspacetest -source workspace.go
//go:generate mockgen -destination=./workspaceapitest/workspace_gomock.go -package workspaceapitest github.com/unstablebuild/rune-go-sdk/api/workspaceapi  FileSystem,File,Executor,Terminal
//go:generate mockgen -destination=./schemetest/scheme_gomock.go -package schemetest github.com/unstablebuild/rune-go-sdk/api/schemeapi  Scheme
