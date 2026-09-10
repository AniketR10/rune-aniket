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

package ide

import (
	"errors"
	"fmt"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/term"
)

type echoKey struct {
	term.KeyComb
	instructWait   bool
	instructPrompt bool
	instructReg    string
}

// parseEchoKeys recursively parses key combinations combined with instructions,
// encoded between {} characters.
func parseEchoKeys(sequence string) (ret []echoKey, err error) {
	idxOpen := strings.IndexRune(sequence, '{')
	if idxOpen < 0 {
		idxOpen = len(sequence)
	}

	// parse up until first instruction
	var keys []term.KeyComb
	keys, err = term.ParseKeys(sequence[0:idxOpen])
	for _, key := range keys {
		ret = append(ret, echoKey{KeyComb: key})
	}
	if err != nil {
		ret = nil
		return
	}

	remainder := sequence[idxOpen:]
	if remainder == "" {
		return
	}

	idxClose := strings.IndexRune(remainder, '}')
	if idxClose < 0 {
		err = errors.New("unterminated key: '{' found but no matching '}' found")
		ret = nil
		return
	}
	instruction := remainder[0 : idxClose+1]
	switch instruction {
	case "{wait}":
		ret = append(ret, echoKey{instructWait: true})
	case "{prompt}":
		ret = append(ret, echoKey{instructPrompt: true})
	case "{register}":
		remainder = remainder[idxClose+1:]
		if remainder == "" {
			err = errors.New("missing register ID after {register}")
			break
		}
		registerID := remainder[:1]
		ret = append(ret, echoKey{instructReg: registerID})
		remainder = remainder[1:]
		idxClose = -1
	default:
		err = fmt.Errorf("invalid instruction: %s", instruction)
	}
	if err != nil {
		ret = nil
		return
	}

	var recKeys []echoKey
	recKeys, err = parseEchoKeys(remainder[idxClose+1:])
	if err != nil {
		ret = nil
		return
	}
	ret = append(ret, recKeys...)
	return
}
