// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package command

import "strings"

// argHintOverrides supplies the placeholders for commands the synopsis
// cannot describe usefully: those spelling out a whole URI grammar in
// one whitespace-free token, which splits into a single unreadable
// slot, and config aliases, which carry no synopsis at all. Keys are
// the full command path (e.g. "lsp diagnostics").
var argHintOverrides = map[string][]string{
	"workspaceopen":  {"<path>"},
	"edit":           {"<path>"},
	"view":           {"<path>"},
	"readfile":       {"<path>"},
	"worktreenew":    {"<name>"},
	"worktreeopen":   {"<name>"},
	"worktreeremove": {"<name>"},
}

// synopsisSlots splits a Manual.Synopsis into one placeholder per
// positional argument. Whitespace inside a bracket or parenthesis
// group belongs to the enclosing slot, so "[<a> [<b>]] <c>" yields
// two slots rather than three.
func synopsisSlots(synopsis string) []string {
	var slots []string
	var depth int
	var start = -1
	for i, r := range synopsis {
		switch r {
		case '[', '(':
			depth++
		case ']', ')':
			if depth > 0 {
				depth--
			}
		}
		if depth == 0 && (r == ' ' || r == '\t') {
			if start >= 0 {
				slots = append(slots, synopsis[start:i])
				start = -1
			}
			continue
		}
		if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		slots = append(slots, synopsis[start:])
	}
	return slots
}

// argHint returns the placeholder for the argument that follows args,
// or "" when the synopsis does not describe another argument. A
// trailing variadic slot ("[<args>...]") keeps matching every argument
// past it.
func argHint(command, synopsis string, args []string) string {
	slots, ok := argHintOverrides[command]
	if !ok {
		slots = synopsisSlots(synopsis)
	}
	if len(slots) == 0 {
		return ""
	}
	idx := pendingSlot(slots, args)
	if idx < len(slots) {
		return slots[idx]
	}
	if last := slots[len(slots)-1]; strings.Contains(last, "...") {
		return last
	}
	return ""
}

// pendingSlot reports how many slots the already typed args consumed.
// Counting them would desynchronize the hint as soon as the user skips
// an optional slot, so a literal slot (one naming no placeholder, like
// the `--` of `tasknew`) anchors the walk: typing it consumes every
// optional slot standing before it.
func pendingSlot(slots, args []string) int {
	var idx int
	for _, arg := range args {
		if anchor := literalSlotIndex(slots, idx, arg); anchor >= 0 {
			idx = anchor + 1
			continue
		}
		idx++
	}
	return idx
}

// literalSlotIndex returns the index of the slot at or after from that
// arg spells out verbatim, or -1 when reaching it would require
// skipping a mandatory slot.
func literalSlotIndex(slots []string, from int, arg string) int {
	for i := from; i < len(slots); i++ {
		if slots[i] == arg {
			return i
		}
		if !optionalSlot(slots[i]) {
			return -1
		}
	}
	return -1
}

func optionalSlot(slot string) bool {
	return strings.HasPrefix(slot, "[") && strings.HasSuffix(slot, "]")
}
