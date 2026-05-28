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
