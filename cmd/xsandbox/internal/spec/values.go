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

package spec

import (
	"fmt"

	"go.starlark.net/starlark"
	"unstable.build/go-tui/cmd/xsandbox/internal/record"
	"unstable.build/go-tui/ide/starlarkconfig"
)

// predicateValue wraps a record.Predicate as an opaque Starlark value
// usable inside expect_rpc where dicts.
type predicateValue struct {
	p record.Predicate
}

var _ starlark.Value = predicateValue{}

func (v predicateValue) String() string        { return v.p.String() }
func (v predicateValue) Type() string          { return "matcher" }
func (v predicateValue) Freeze()               {}
func (v predicateValue) Truth() starlark.Bool  { return starlark.True }
func (v predicateValue) Hash() (uint32, error) { return starlark.String(v.p.String()).Hash() }

func predicateBuiltin(kind string, wantsArg bool) func(
	*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple,
) (starlark.Value, error) {
	return func(
		_ *starlark.Thread, b *starlark.Builtin,
		args starlark.Tuple, kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		var arg string
		if wantsArg {
			if err := starlark.UnpackArgs(b.Name(), args, kwargs, "value", &arg); err != nil {
				return nil, err
			}
		} else if err := starlark.UnpackArgs(b.Name(), args, kwargs); err != nil {
			return nil, err
		}
		return predicateValue{p: record.Predicate{Kind: kind, Arg: arg}}, nil
	}
}

// commandHandle is returned by expect_command / expect_repl_command
// so subsequent invoke calls can reference the registration.
type commandHandle struct {
	name string
	repl bool
}

var _ starlark.Value = commandHandle{}

func (h commandHandle) String() string {
	if h.repl {
		return fmt.Sprintf("<repl command %q>", h.name)
	}
	return fmt.Sprintf("<command %q>", h.name)
}
func (h commandHandle) Type() string          { return "command" }
func (h commandHandle) Freeze()               {}
func (h commandHandle) Truth() starlark.Bool  { return starlark.True }
func (h commandHandle) Hash() (uint32, error) { return starlark.String(h.name).Hash() }

// windowHandle is returned by expect_window so render and send_key can
// reference the installed handler.
type windowHandle struct {
	id     string
	method string
}

var _ starlark.Value = windowHandle{}

func (h windowHandle) String() string        { return fmt.Sprintf("<window %q>", h.method) }
func (h windowHandle) Type() string          { return "window" }
func (h windowHandle) Freeze()               {}
func (h windowHandle) Truth() starlark.Bool  { return starlark.True }
func (h windowHandle) Hash() (uint32, error) { return starlark.String(h.id).Hash() }

// windowHandleID extracts the handle id from a value returned by
// expect_window.
func windowHandleID(v starlark.Value) (string, error) {
	h, ok := v.(windowHandle)
	if !ok {
		return "", fmt.Errorf("window must be a handle from expect_window, got %s", v.Type())
	}
	return h.id, nil
}

// matcherFromDict converts a where dict to a record.Matcher. Values
// may be plain Starlark values (compared exactly) or predicate values
// from present()/contains()/regex(), including nested inside dicts.
func matcherFromDict(d *starlark.Dict) (*record.Matcher, error) {
	if d == nil {
		return nil, nil
	}
	fields := make(map[string]any, d.Len())
	for _, item := range d.Items() {
		key, ok := item[0].(starlark.String)
		if !ok {
			return nil, fmt.Errorf("keys must be strings, got %s", item[0].Type())
		}
		v, err := matcherValue(item[1])
		if err != nil {
			return nil, fmt.Errorf("key %q: %w", string(key), err)
		}
		fields[string(key)] = v
	}
	return record.NewMatcher(fields), nil
}

func matcherValue(v starlark.Value) (any, error) {
	switch t := v.(type) {
	case predicateValue:
		return t.p, nil
	case *starlark.Dict:
		out := make(map[string]any, t.Len())
		for _, item := range t.Items() {
			key, ok := item[0].(starlark.String)
			if !ok {
				return nil, fmt.Errorf("keys must be strings, got %s", item[0].Type())
			}
			nested, err := matcherValue(item[1])
			if err != nil {
				return nil, err
			}
			out[string(key)] = nested
		}
		return out, nil
	default:
		return starlarkconfig.ToGo(v)
	}
}
