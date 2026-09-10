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

package markdown

import (
	"net/url"

	"github.com/unstablebuild/rune-go-sdk/term"
)

// Option configures a Handler.
type Option func(*Handler)

// WithOnLinkClick sets a callback for when a link is clicked.
// The callback returns true if it handled the link, false otherwise.
// If the callback returns false (or is not set), clicking local anchors
// (URLs starting with #) will scroll to the corresponding header.
func WithOnLinkClick(fn func(*url.URL) bool) Option {
	return func(h *Handler) {
		h.onLinkClick = fn
	}
}

// WithSelectionAttrs sets the attributes used to highlight selected text.
// The attributes are unioned with existing cell attributes.
// Defaults to term.Attributes with AttrReverse.
func WithSelectionAttrs(attrs term.Attributes) Option {
	return func(h *Handler) {
		h.selectionAttrs = attrs
	}
}
