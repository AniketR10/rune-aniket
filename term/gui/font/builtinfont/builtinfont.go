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

//revive:disable:exported
package builtinfont

import _ "embed"

//go:embed JetBrainsMonoNerdFontPropo-ExtraBold.ttf
var BoldTTF []byte

//go:embed JetBrainsMonoNerdFontPropo-Regular.ttf
var RegularTTF []byte

//go:embed JetBrainsMonoNerdFontPropo-Italic.ttf
var ItalicTTF []byte

//go:embed JetBrainsMonoNerdFontPropo-ExtraBoldItalic.ttf
var BoldItalicTTF []byte

//go:embed Braille.ttf
var BrailleTTF []byte

//go:embed MesloLGL-Regular.ttf
var FallbackTTF []byte

//go:embed Symbola.ttf
var SymbolTTF []byte

//go:embed NotoColorEmoji.ttf
var EmojiTTF []byte

//go:embed NotoSansCJK-Regular.ttc
var CJKTTC []byte
