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

// Package utf8validate provides small helpers used by the agent and
// its tools to keep model-facing strings valid UTF-8: sanitisation,
// invalid-byte counting, binary-content detection, and the canonical
// stub returned in place of binary blobs.
package utf8validate

import (
	"encoding/hex"
	"fmt"
	"strings"
	"unicode/utf8"
)

// binaryScanLimit bounds how many bytes IsBinary inspects.
const binaryScanLimit = 8 * 1024

// binarySuspiciousRatio is the fraction of suspicious bytes above which
// content is classified as binary.
const binarySuspiciousRatio = 0.30

// Sanitize returns s with each invalid UTF-8 byte replaced by the
// Unicode replacement character (U+FFFD). When s is already valid
// UTF-8 the original string is returned without allocation. Unlike
// strings.ToValidUTF8, runs of invalid bytes are not collapsed: one
// U+FFFD is emitted per offending byte so the count stays accurate.
func Sanitize(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			b.WriteRune('\ufffd')
			i++
			continue
		}
		b.WriteString(s[i : i+size])
		i += size
	}
	return b.String()
}

// CountInvalidBytes returns the number of bytes in s that are not
// part of a valid UTF-8 sequence. These are the bytes that Sanitize
// would replace with U+FFFD. Zero is returned for valid UTF-8 strings.
func CountInvalidBytes(s string) int {
	if utf8.ValidString(s) {
		return 0
	}
	var count int
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			count++
		}
		i += size
	}
	return count
}

// IsBinary reports whether data looks like binary content using a
// git/ripgrep style heuristic: any NUL byte in the first 8 KiB, or
// more than 30% of inspected bytes outside printable ASCII /
// whitespace and not part of a valid UTF-8 multi-byte sequence.
// Empty input is treated as text.
func IsBinary(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	scan := data
	if len(scan) > binaryScanLimit {
		scan = scan[:binaryScanLimit]
	}
	var suspicious int
	for i := 0; i < len(scan); {
		b := scan[i]
		if b == 0x00 {
			return true
		}
		// Printable ASCII or common whitespace.
		if b == 0x09 || b == 0x0A || b == 0x0D || (b >= 0x20 && b <= 0x7E) {
			i++
			continue
		}
		// Try to decode a UTF-8 multi-byte rune. A valid rune of size > 1
		// is text; the RuneError sentinel (with size 1) marks an invalid
		// byte that counts as suspicious.
		r, size := utf8.DecodeRune(scan[i:])
		if r != utf8.RuneError && size > 1 {
			i += size
			continue
		}
		suspicious++
		i++
	}
	return float64(suspicious)/float64(len(scan)) > binarySuspiciousRatio
}

// BinaryStub returns the canonical metadata line returned by tools
// when they refuse to inline binary content. It includes the file
// name, byte length, sha256 hex digest, and a short hint telling the
// model how to inspect the bytes if it really needs to (so it doesn't
// loop retrying read_file on the same file). The hash is rendered as
// lowercase hex.
func BinaryStub(name string, size int, sum [32]byte) string {
	return fmt.Sprintf(
		"<binary file: %s, %d bytes, sha256=%s — "+
			"refused to inline binary content; "+
			"use bash with `file`, `xxd`, `hexdump -C`, or `strings` "+
			"on this path if you need to inspect the bytes>",
		name, size, hex.EncodeToString(sum[:]),
	)
}

// InvalidBytesMarker returns the trailing marker appended after
// sanitising text-with-stray-bytes. Returns the empty string when n is
// not positive.
func InvalidBytesMarker(n int) string {
	if n <= 0 {
		return ""
	}
	noun := "byte"
	if n != 1 {
		noun = "bytes"
	}
	return fmt.Sprintf("(%d invalid UTF-8 %s replaced with U+FFFD)", n, noun)
}
