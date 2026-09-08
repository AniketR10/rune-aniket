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

package font

import (
	"os"
	"strings"
)

// Face indices into builtinfont.CJKTTC. The collection holds Noto Sans
// CJK JP,KR,SC,TC,HK followed by Noto Sans Mono CJK in the same order.
// Only the Mono faces are usable in a terminal: their ASCII advance is
// exactly one cell, whereas the proportional faces overflow it.
const (
	cjkFaceMonoJP = 5
	cjkFaceMonoKR = 6
	cjkFaceMonoSC = 7
	cjkFaceMonoTC = 8
	cjkFaceMonoHK = 9
)

// cjkFaceIndex picks the Han-unification variant matching the user's
// locale, following POSIX precedence LC_ALL > LC_CTYPE > LANG.
func cjkFaceIndex() int {
	return cjkFaceIndexForLocale(currentLocale())
}

func currentLocale() string {
	for _, key := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	return ""
}

func cjkFaceIndexForLocale(locale string) int {
	// strip the ".UTF-8" codeset and any "@modifier" suffix
	lang := locale
	if i := strings.IndexAny(lang, ".@"); i >= 0 {
		lang = lang[:i]
	}
	lang = strings.ReplaceAll(strings.ToLower(lang), "-", "_")

	switch {
	case strings.HasPrefix(lang, "ja"):
		return cjkFaceMonoJP
	case strings.HasPrefix(lang, "ko"):
		return cjkFaceMonoKR
	case lang == "zh_hk", lang == "zh_mo",
		strings.HasPrefix(lang, "zh_hant_hk"), strings.HasPrefix(lang, "zh_hant_mo"):
		return cjkFaceMonoHK
	case lang == "zh_tw", strings.HasPrefix(lang, "zh_hant"):
		return cjkFaceMonoTC
	default:
		return cjkFaceMonoSC
	}
}
