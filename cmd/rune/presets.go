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

package main

import (
	_ "embed"
	"fmt"
)

//go:embed preset_modal.yaml
var presetModalYAML string

//go:embed preset_emacs.yaml
var presetEmacsYAML string

// renderPreset returns the preset-config file body for the given
// editor choice. The modal choice enables vim mode everywhere; the
// standard choice uses platform-native standard editor bindings;
// the emacs choice uses an Emacs keymap. The deprecated "modeless" alias
// resolves to the standard preset.
func renderPreset(editor string) (string, error) {
	switch editor {
	case editorModal:
		return presetModalYAML, nil
	case editorStandard, editorModeless:
		return presetStandardYAML, nil
	case editorEmacs:
		return presetEmacsYAML, nil
	}
	return "", fmt.Errorf("unknown editor choice: %q", editor)
}
