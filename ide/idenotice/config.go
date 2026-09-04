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

// Package idenotice displays a per-workspace floating notice on
// workspace open. The notice content can be a literal string or a
// path resolved against the workspace filesystem, and can be shown
// once (deduplicated via a content fingerprint persisted in storage)
// or always.
package idenotice

import (
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

// Valid values for Config.Show.
const (
	ShowOnce   = "once"
	ShowAlways = "always"
)

// Config describes how the notice should be resolved and shown.
//
// When both Path and Literal are set, Literal wins. When neither is
// set, Crier.Show is a no-op.
type Config struct {
	// A ".md" extension renders the file as markdown; anything else
	// is rendered as plain text.
	Path string
	// Literal is always rendered as markdown.
	Literal string
	// Empty Show defaults to ShowOnce.
	Show string
	// Required when Show == ShowOnce.
	Storage      storageapi.Service
	WorkspaceURI workspaceapi.URI
}

func (c Config) effectiveShow() string {
	if c.Show == "" {
		return ShowOnce
	}
	return c.Show
}
