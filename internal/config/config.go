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

package config

import (
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// GetFrameCharset is a helper which extracts and parses a component.FrameCharSet.
func GetFrameCharset(c config.Config, key string, def component.FrameCharSet) (
	component.FrameCharSet, error,
) {
	cfg, err := c.GetConfig(key)
	if err != nil {
		return def, err
	}

	cs := def
	r, err := cfg.GetRune("topleft")
	if err == nil {
		cs.TopLeft = r
	}
	r, err = cfg.GetRune("topright")
	if err == nil {
		cs.TopRight = r
	}
	r, err = cfg.GetRune("bottomleft")
	if err == nil {
		cs.BottomLeft = r
	}
	r, err = cfg.GetRune("bottomright")
	if err == nil {
		cs.BottomRight = r
	}
	r, err = cfg.GetRune("horizontaltop")
	if err == nil {
		cs.HorizontalTop = r
	}
	r, err = cfg.GetRune("horizontalbottom")
	if err == nil {
		cs.HorizontalBottom = r
	}
	r, err = cfg.GetRune("verticalleft")
	if err == nil {
		cs.VerticalLeft = r
	}
	r, err = cfg.GetRune("verticalright")
	if err == nil {
		cs.VerticalRight = r
	}
	return cs, nil
}

// GetKey is a helper which extracts and parses a term.KeyComb as a string
// from a Config.
func GetKey(c config.Config, key string) (term.KeyComb, error) {
	s, err := c.GetString(key)
	if err != nil {
		return term.KeyComb{}, err
	}
	return term.ParseKey(s)
}
