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

package idecmd

import "fmt"

// checkReferences returns an error when token references a
// positional argument $N (N=1..9) with N greater than argCount.
func checkReferences(token string, argCount int) error {
	for _, name := range scanDollarRefs(token) {
		if len(name) != 1 || name[0] < '1' || name[0] > '9' {
			continue
		}
		pos := int(name[0]-'0') - 1
		if pos >= argCount {
			return fmt.Errorf(
				"alias expects an argument at position %d ($%s)",
				pos+1, name)
		}
	}
	return nil
}

// recordReferences inserts each 0-based positional index referenced
// by token (and within argCount bounds) into out.
func recordReferences(token string, argCount int, out map[int]struct{}) {
	for _, name := range scanDollarRefs(token) {
		if len(name) != 1 || name[0] < '1' || name[0] > '9' {
			continue
		}
		pos := int(name[0]-'0') - 1
		if pos < argCount {
			out[pos] = struct{}{}
		}
	}
}

// scanDollarRefs returns names referenced via $NAME / ${NAME} / $N
// inside s. Escaped \$ is skipped, and $$ is treated as a literal $.
// Duplicates are not deduplicated.
func scanDollarRefs(s string) []string {
	var names []string
	i := 0
	n := len(s)
	for i < n {
		c := s[i]
		if c == '\\' && i+1 < n {
			i += 2
			continue
		}
		if c != '$' {
			i++
			continue
		}
		if i+1 < n && s[i+1] == '$' {
			i += 2
			continue
		}
		i++
		if i >= n {
			break
		}
		if s[i] == '{' {
			i++
			start := i
			for i < n && s[i] != '}' {
				i++
			}
			if i > start {
				names = append(names, s[start:i])
			}
			if i < n {
				i++
			}
			continue
		}
		start := i
		if !isDollarHead(s[i]) {
			continue
		}
		i++
		if s[start] >= '0' && s[start] <= '9' {
			names = append(names, s[start:i])
			continue
		}
		for i < n && isDollarTail(s[i]) {
			i++
		}
		names = append(names, s[start:i])
	}
	return names
}

func isDollarHead(b byte) bool {
	return (b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		b == '_' ||
		(b >= '0' && b <= '9')
}

func isDollarTail(b byte) bool {
	return (b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9') ||
		b == '_'
}
