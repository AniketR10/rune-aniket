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

package html

import (
	"net/http"

	"github.com/unstablebuild/rune-go-sdk/term"
	htmlcomp "unstable.build/rune/internal/component/html"
	"unstable.build/rune/internal/component/markdown"
)

// Option configures a [Handler].
type Option func(*Handler)

// WithHTTPClient sets the HTTP client used for fetching HTML content.
// Defaults to [http.DefaultClient].
func WithHTTPClient(client *http.Client) Option {
	return func(h *Handler) {
		h.httpClient = client
	}
}

// WithMarkdownConfig sets the markdown rendering configuration used
// when converting fetched HTML content. Defaults to
// [markdown.DefaultConfig].
func WithMarkdownConfig(cfg markdown.Config) Option {
	return func(h *Handler) {
		h.mdCfg = cfg
	}
}

// WithSelectionAttrs sets the attributes used to highlight selected
// text. Defaults to [term.AttrReverse].
func WithSelectionAttrs(attrs term.Attributes) Option {
	return func(h *Handler) {
		h.selectionAttrs = attrs
	}
}

// WithComponentOptions appends additional options that are forwarded
// to every [htmlcomp.Component] created by the handler.
func WithComponentOptions(opts ...htmlcomp.Option) Option {
	return func(h *Handler) {
		h.compOpts = append(h.compOpts, opts...)
	}
}

// BarPosition specifies where the navigation bar appears.
type BarPosition int

const (
	// BarTop places the navigation bar at the top of the handler.
	BarTop BarPosition = iota
	// BarBottom places the navigation bar at the bottom of the handler.
	BarBottom
)

// WithNavigationBar adds a navigation bar with back/forward buttons
// and a URL input box at the specified position.
func WithNavigationBar(pos BarPosition) Option {
	return func(h *Handler) {
		h.barPos = pos
		h.bar = newNavigationBar("")
	}
}
