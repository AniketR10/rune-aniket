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

package plugin

import (
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/internal/term/vte"
)

// Option represents a Handler configuration option.
type Option func(*handlerConfig)

// WithVTEConfig instructs Handler to use
// the given vte.Config as the underlying vte.Handler config.
func WithVTEConfig(cfg vte.Config) Option {
	return func(hcfg *handlerConfig) {
		watcher := hcfg.cfg.Watcher
		hcfg.cfg = cfg
		if watcher != nil {
			hcfg.cfg.Watcher = workspaceapi.MultiProcessWatcher(
				watcher, hcfg.cfg.Watcher,
			)
		}
	}
}

// WithProcessWatcher instructs Handler to add the given ProcessWatcher
// to the vte.Config used. If WithVTEConfig is passed, then this ProcessWatcher
// will be added to the Watcher defined there.
func WithProcessWatcher(w workspaceapi.ProcessWatcher) Option {
	return func(hcfg *handlerConfig) {
		if hcfg.cfg.Watcher != nil {
			hcfg.cfg.Watcher = workspaceapi.MultiProcessWatcher(
				hcfg.cfg.Watcher, w,
			)
		} else {
			hcfg.cfg.Watcher = w
		}
	}
}

// WithFrame returns an option that configures
// whether Handler draws a divider between the top
// bar and the vte.
func WithFrame(frame bool) Option {
	return func(cfg *handlerConfig) {
		cfg.frame = frame
	}
}

// WithTitle returns an option that sets the title of the
// plugin handler, situated in the top bar. Passing an empty
// title, or not passing this option defaults to using
// the command and arguments as title.
func WithTitle(title string) Option {
	return func(cfg *handlerConfig) {
		cfg.title = title
	}
}

// WithFrameCharSet returns an option that configures
// Handler to use the given character set to draw
// a divider between the top bar and the vte. It's a no-op
// if frame is set to false.
func WithFrameCharSet(charSet component.FrameCharSet) Option {
	return func(cfg *handlerConfig) {
		cfg.frameCharSet = charSet
	}
}

// WithFrameAttr returns an option that configures
// the divider attributes.
func WithFrameAttr(attr term.Attributes) Option {
	return func(cfg *handlerConfig) {
		cfg.frameAttr = attr
	}
}

// WithBarConfig returns an option that configures
// the top bar.
func WithBarConfig(barConfig BarConfig) Option {
	return func(cfg *handlerConfig) {
		cfg.bar = barConfig
	}
}

// WithoutBarCommand returns an option that removes the command and
// args from the top bar layout. Useful when the command is displayed
// elsewhere, such as on a floating window's bar.
func WithoutBarCommand() Option {
	return func(cfg *handlerConfig) {
		cfg.noBarCommand = true
	}
}

// WithCommandExpander returns an option that installs a
// vte.CommandExpander into the underlying vte.Config. The expander
// runs in a background goroutine after the pty is created but
// before the plugin's command is started; it can therefore perform
// slow I/O (e.g. resolving $(...) via the workspace executor)
// without blocking the event loop or the floating window's UI.
func WithCommandExpander(expander vte.CommandExpander) Option {
	return func(cfg *handlerConfig) {
		cfg.cfg.CommandExpander = expander
	}
}

type handlerConfig struct {
	bar          BarConfig
	cfg          vte.Config
	frame        bool
	frameCharSet component.FrameCharSet
	frameAttr    term.Attributes
	title        string
	noBarCommand bool
}

func defaultConfig() handlerConfig {
	return handlerConfig{
		bar:          DefaultBarConfig(),
		cfg:          vte.DefaultConfig(),
		frame:        true,
		frameCharSet: component.FrameCharSetDefault(),
		frameAttr:    term.Attributes{},
	}
}
