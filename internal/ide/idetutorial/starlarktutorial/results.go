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

package starlarktutorial

import (
	"errors"
	"fmt"

	"go.starlark.net/starlark"
)

// commandResult is the immutable Starlark value returned by the
// wait_command builtin. Authors read .name and .args inline and use
// them in subsequent control flow.
type commandResult struct {
	name string
	args starlark.Tuple
}

func newCommandResult(name string, args []string) *commandResult {
	tup := make(starlark.Tuple, len(args))
	for i, a := range args {
		tup[i] = starlark.String(a)
	}
	return &commandResult{name: name, args: tup}
}

func (r *commandResult) String() string {
	return fmt.Sprintf("command_result(name=%q, args=%s)", r.name, r.args.String())
}

func (r *commandResult) Type() string         { return "command_result" }
func (r *commandResult) Freeze()              {}
func (r *commandResult) Truth() starlark.Bool { return starlark.True }

func (r *commandResult) Hash() (uint32, error) {
	return 0, errors.New("command_result is unhashable")
}

func (r *commandResult) Attr(name string) (starlark.Value, error) {
	switch name {
	case "name":
		return starlark.String(r.name), nil
	case "args":
		return r.args, nil
	}
	return nil, nil
}

func (r *commandResult) AttrNames() []string { return []string{"name", "args"} }

var (
	_ starlark.Value    = (*commandResult)(nil)
	_ starlark.HasAttrs = (*commandResult)(nil)
)

// choiceResult is the immutable Starlark value returned by the choice
// and confirm builtins. choice exposes the full (value, index,
// selected) triple; confirm wraps the boolean result and surfaces a
// minimal selected/index/value triple consistent with choice so
// author code never has to switch on which builtin produced the
// result.
type choiceResult struct {
	value    string
	index    int
	selected bool
}

func newChoiceResult(idx int, value string, selected bool) *choiceResult {
	return &choiceResult{value: value, index: idx, selected: selected}
}

func (r *choiceResult) String() string {
	return fmt.Sprintf("choice_result(value=%q, index=%d, selected=%t)",
		r.value, r.index, r.selected)
}

func (r *choiceResult) Type() string         { return "choice_result" }
func (r *choiceResult) Freeze()              {}
func (r *choiceResult) Truth() starlark.Bool { return starlark.Bool(r.selected) }

func (r *choiceResult) Hash() (uint32, error) {
	return 0, errors.New("choice_result is unhashable")
}

func (r *choiceResult) Attr(name string) (starlark.Value, error) {
	switch name {
	case "value":
		return starlark.String(r.value), nil
	case "index":
		return starlark.MakeInt(r.index), nil
	case "selected":
		return starlark.Bool(r.selected), nil
	}
	return nil, nil
}

func (r *choiceResult) AttrNames() []string {
	return []string{"value", "index", "selected"}
}

var (
	_ starlark.Value    = (*choiceResult)(nil)
	_ starlark.HasAttrs = (*choiceResult)(nil)
)
