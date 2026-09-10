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

package extutil

import (
	"errors"
	"fmt"

	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/clipboard"
	"unstable.build/rune/internal/browser"
	"unstable.build/rune/internal/component"
	"unstable.build/rune/internal/text"
	"unstable.build/rune/internal/text/emacs"
	"unstable.build/rune/internal/text/standard"
	"unstable.build/rune/internal/text/vi"
)

// Tabspaces extract editor.tabspaces from the given cfg.
func Tabspaces(cfg config.Config) (int, error) {
	editorConfig, err := cfg.GetConfig("editor")
	if err != nil {
		if err != config.ErrNotFound {
			err = fmt.Errorf("failed to get 'editor' from config: %v", err)
			return 0, err
		}
		return component.DefaultTabspaces, nil
	}

	ret, err := editorConfig.GetInt("tabspaces")
	if err != nil {
		if err != config.ErrNotFound {
			err = fmt.Errorf("failed to get 'tabspaces' from config: %v", err)
			return 0, err
		}
		ret = component.DefaultTabspaces
	}
	return ret, nil
}

// WindowManagerFrame extract browser.window_manager.frame from the given cfg.
func WindowManagerFrame(cfg config.Config) (bool, error) {
	browserConfig, err := cfg.GetConfig("browser")
	if err != nil {
		if err != config.ErrNotFound {
			err = fmt.Errorf("failed to get 'browser' from config: %v", err)
			return false, err
		}
		return browser.DefaultConfig().Frame, nil
	}

	wmConfig, err := browserConfig.GetConfig("window_manager")
	if err != nil {
		if err != config.ErrNotFound {
			err = fmt.Errorf("failed to get 'window_manager' from config: %v", err)
			return false, err
		}
		return browser.DefaultConfig().Frame, nil
	}

	ret, err := wmConfig.GetBool("frame")
	if err != nil {
		if err != config.ErrNotFound {
			err = fmt.Errorf("failed to get 'frame' from config: %v", err)
			return false, err
		}
		ret = browser.DefaultConfig().Frame
	}
	return ret, nil
}

// Clipboard returns the configured clipboard.
func Clipboard(cfg config.Config) (clipboard.Register, error) {
	sys, err := cfg.GetString("clipboard")
	if err != nil && err != config.ErrNotFound {
		err = fmt.Errorf("failed to get 'clipboard' from config: %v", err)
		return nil, err
	}

	if err == config.ErrNotFound {
		sys = "memory"
	}

	switch sys {
	case "memory":
		return clipboard.NewInMemory(), nil
	case "system":
		return text.NewSystemClipboard(), nil
	default:
		return nil, errors.New("unknown clipboard")
	}
}

// Editor returns the editor implementation as configured.
func Editor(clipboard clipboard.Register, cfg config.Config) (text.Editor, error) {
	mode, err := editorMode(cfg)
	if err != nil {
		return nil, err
	}
	switch mode {
	case "modal":
		return viEditor(clipboard), nil
	case "emacs":
		return emacsEditor(clipboard), nil
	case "exo":
		fallback, err := exoFallback(cfg)
		if err != nil {
			return nil, err
		}
		switch fallback {
		case "modal":
			return viEditor(clipboard), nil
		case "emacs":
			return emacsEditor(clipboard), nil
		}
		return standardEditor(clipboard), nil
	default:
		return standardEditor(clipboard), nil
	}
}

// EditorModal reports whether the configured compose editor is modal
// (vi-style). It mirrors the resolution Editor performs, including the
// editor.exo fallback. A bare <Enter> submits in modal normal mode and
// inserts a newline in insert mode, so callers gate submission on this.
func EditorModal(cfg config.Config) (bool, error) {
	mode, err := editorMode(cfg)
	if err != nil {
		return false, err
	}
	switch mode {
	case "modal":
		return true, nil
	case "exo":
		fallback, err := exoFallback(cfg)
		if err != nil {
			return false, err
		}
		return fallback == "modal", nil
	default:
		return false, nil
	}
}

// viEditor builds a vi editor with all chrome bars disabled so an
// extension-hosted compose buffer shows only the text area.
func viEditor(clipboard clipboard.Register) text.Editor {
	return vi.Editor(
		vi.WithClipboard(clipboard),
		vi.WithWrap(true),
		vi.WithSearch(false),
		vi.WithStatusBarConfig(false, text.StatusBarConfig{}),
		vi.WithAuxiliaryBar(false, text.AuxBarConfig{}),
		vi.WithIconsBar(false, text.IconsBarConfig{}),
		vi.WithGitBar(false, text.IconsBarConfig{}),
	)
}

// standardEditor builds a standard editor with all chrome bars disabled
// so an extension-hosted compose buffer shows only the text area.
func standardEditor(clipboard clipboard.Register) text.Editor {
	return standard.Editor(
		standard.WithClipboard(clipboard),
		standard.WithWrap(true),
		standard.WithStatusBarConfig(false, text.StatusBarConfig{}),
		standard.WithAuxiliaryBar(false, text.AuxBarConfig{}),
		standard.WithIconsBar(false, text.IconsBarConfig{}),
		standard.WithGitBar(false, text.IconsBarConfig{}),
	)
}

// emacsEditor builds an emacs editor with all chrome bars disabled so an
// extension-hosted compose buffer shows only the text area.
func emacsEditor(clipboard clipboard.Register) text.Editor {
	return emacs.Editor(
		emacs.WithClipboard(clipboard),
		emacs.WithWrap(true),
		emacs.WithStatusBarConfig(false, text.StatusBarConfig{}),
		emacs.WithAuxiliaryBar(false, text.AuxBarConfig{}),
		emacs.WithIconsBar(false, text.IconsBarConfig{}),
		emacs.WithGitBar(false, text.IconsBarConfig{}),
	)
}

// Wrap returns the current editor implementation is configured with wrap mode.
func Wrap(cfg config.Config) (bool, error) {
	var def bool
	edConfig, err := cfg.GetConfig("editor")
	if err != nil {
		if err != config.ErrNotFound {
			err = fmt.Errorf("failed to get 'editor' from config: %v", err)
			return false, err
		}
		return false, nil
	}

	mode, err := editorMode(cfg)
	if err != nil {
		return false, err
	}
	modeConfig, err := edConfig.GetConfig(mode)
	if err != nil {
		if err != config.ErrNotFound {
			err = fmt.Errorf("failed to get '%s' from editor config: %v", mode, err)
			return false, err
		}
		return def, nil
	}

	wrap, err := modeConfig.GetBool("wrap")
	if err != nil {
		if err != config.ErrNotFound {
			err = fmt.Errorf("failed to get 'wrap' from editor config: %v", err)
			return false, err
		}
	}
	return wrap, nil
}

func editorMode(cfg config.Config) (string, error) {
	def := "modal"
	edConfig, err := cfg.GetConfig("editor")
	if err != nil {
		if err != config.ErrNotFound {
			err = fmt.Errorf("failed to get 'editor' from config: %v", err)
			return "", err
		}
		return def, nil
	}

	mode, err := edConfig.GetString("mode")
	if err != nil {
		if err != config.ErrNotFound {
			err = fmt.Errorf("failed to get 'mode' from editor config: %v", err)
			return "", err
		}
		mode = def
	}
	if mode == "modeless" {
		return "standard", nil
	}
	return mode, nil
}

// exoFallback returns the Rune-native fallback editor used when
// editor.mode is "exo". The full external editor is not viable inside
// an extension process, so compose input uses this fallback. Valid
// values are "modal", "standard", or "emacs"; the deprecated "modeless"
// alias resolves to "standard". Defaults to "standard".
func exoFallback(cfg config.Config) (string, error) {
	def := "standard"
	edConfig, err := cfg.GetConfig("editor")
	if err != nil {
		if err != config.ErrNotFound {
			return "", fmt.Errorf("failed to get 'editor' from config: %v", err)
		}
		return def, nil
	}

	exoConfig, err := edConfig.GetConfig("exo")
	if err != nil {
		if err != config.ErrNotFound {
			return "", fmt.Errorf("failed to get 'exo' from editor config: %v", err)
		}
		return def, nil
	}

	fallback, err := exoConfig.GetString("fallback")
	if err != nil {
		if err != config.ErrNotFound {
			return "", fmt.Errorf("failed to get 'fallback' from editor.exo config: %v", err)
		}
		return def, nil
	}

	switch fallback {
	case "modal", "standard", "emacs":
		return fallback, nil
	case "modeless":
		return "standard", nil
	}
	return def, nil
}
