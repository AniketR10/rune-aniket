// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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

package ide

import (
	"errors"
	"fmt"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

// starlarkConfigGlobal is the name of the top-level dict the script reads
// from and writes to. Anything else is ignored.
const starlarkConfigGlobal = "config"

// starlarkConfigSource describes one Starlark config invocation.
type starlarkConfigSource struct {
	// Source and filename used in error messages.
	src      []byte
	filename string

	// Params are exposed as extra predeclared globals. The keys must be
	// valid Starlark identifiers.
	params map[string]any

	// Base, when non-nil, switches the decoder to *overlay mode*: the
	// script starts with a predeclared `config` pre-populated from this
	// map, and may mutate it freely (config["x"] = y, del config["x"],
	// etc.). The resulting config global is returned as-is — no deep
	// merge.
	//
	// When nil the decoder runs in *base mode*: the script must bind a
	// top-level `config = {...}` dict itself, which is then returned.
	base map[string]any
}

// decodeStarlarkConfig parses and executes src. See starlarkConfigSource
// for the two modes.
//
// The script runs with stdlib Starlark only — no file I/O, no load(), no
// environment access. Conditionals (if/elif/else) and loops (for) at the
// top level are enabled via FileOptions.TopLevelControl.
func decodeStarlarkConfig(s starlarkConfigSource) (map[string]any, error) {
	filename := s.filename
	if filename == "" {
		filename = "rune.star"
	}

	thread := &starlark.Thread{
		Name:  "rune.star",
		Print: func(*starlark.Thread, string) {},
		Load: func(_ *starlark.Thread, module string) (starlark.StringDict, error) {
			return nil, fmt.Errorf("load() is not allowed in rune.star: cannot load %q", module)
		},
	}
	opts := &syntax.FileOptions{
		TopLevelControl: true,
		GlobalReassign:  true,
		Recursion:       true,
	}

	predeclared, err := goMapToStarlarkDict(s.params)
	if err != nil {
		return nil, fmt.Errorf("starlark: encode params: %w", err)
	}

	// Overlay mode: pre-populate `config` so the script can read/mutate it.
	if s.base != nil {
		baseVal, err := goToStarlark(s.base)
		if err != nil {
			return nil, fmt.Errorf("starlark: encode base config: %w", err)
		}
		if _, taken := predeclared[starlarkConfigGlobal]; taken {
			return nil, fmt.Errorf("starlark: param %q collides with "+
				"predeclared base config", starlarkConfigGlobal)
		}
		predeclared[starlarkConfigGlobal] = baseVal
	}

	globals, err := starlark.ExecFileOptions(opts, thread, filename, s.src, predeclared)
	if err != nil {
		var evalErr *starlark.EvalError
		if errors.As(err, &evalErr) {
			return nil, fmt.Errorf("starlark: %s", evalErr.Backtrace())
		}
		return nil, fmt.Errorf("starlark: %w", err)
	}

	// If the script never rebinds `config`, it won't appear in globals —
	// in overlay mode the predeclared dict still holds the latest value
	// because Starlark dict mutations modify the object in place.
	val, ok := globals[starlarkConfigGlobal]
	if !ok {
		val, ok = predeclared[starlarkConfigGlobal]
	}
	if !ok {
		return nil, fmt.Errorf("starlark: expected top-level %q dict", starlarkConfigGlobal)
	}
	dict, ok := val.(*starlark.Dict)
	if !ok {
		return nil, fmt.Errorf("starlark: %q must be a dict, got %s",
			starlarkConfigGlobal, val.Type())
	}
	out, err := starlarkDictToMap(dict)
	if err != nil {
		return nil, fmt.Errorf("starlark: %w", err)
	}
	return out, nil
}

// starlarkToGo converts an arbitrary Starlark value to the closest Go
// equivalent used by the rest of the config pipeline (map[string]any /
// []any / scalar primitives / string).
func starlarkToGo(v starlark.Value) (any, error) {
	switch t := v.(type) {
	case starlark.NoneType:
		return nil, nil
	case starlark.Bool:
		return bool(t), nil
	case starlark.Int:
		// Try int64 first, fall back to float64 only if it overflows.
		if i, ok := t.Int64(); ok {
			return int(i), nil
		}
		f, _ := starlark.AsFloat(t)
		return f, nil
	case starlark.Float:
		return float64(t), nil
	case starlark.String:
		return string(t), nil
	case *starlark.List:
		out := make([]any, 0, t.Len())
		iter := t.Iterate()
		defer iter.Done()
		var item starlark.Value
		for iter.Next(&item) {
			gv, err := starlarkToGo(item)
			if err != nil {
				return nil, err
			}
			out = append(out, gv)
		}
		return out, nil
	case starlark.Tuple:
		out := make([]any, 0, t.Len())
		for i := range t.Len() {
			gv, err := starlarkToGo(t.Index(i))
			if err != nil {
				return nil, err
			}
			out = append(out, gv)
		}
		return out, nil
	case *starlark.Dict:
		return starlarkDictToMap(t)
	default:
		return nil, fmt.Errorf("unsupported starlark value of type %s", v.Type())
	}
}

// starlarkDictToMap converts a Starlark dict with string keys to a Go map.
// Non-string keys are rejected because the downstream config consumers
// (config.MapConfig) assume string-keyed maps.
func starlarkDictToMap(d *starlark.Dict) (map[string]any, error) {
	out := make(map[string]any, d.Len())
	for _, item := range d.Items() {
		keyVal, value := item[0], item[1]
		keyStr, ok := keyVal.(starlark.String)
		if !ok {
			return nil, fmt.Errorf("dict keys must be strings, got %s", keyVal.Type())
		}
		gv, err := starlarkToGo(value)
		if err != nil {
			return nil, err
		}
		out[string(keyStr)] = gv
	}
	return out, nil
}

// goMapToStarlarkDict converts a Go map[string]any into a flat Starlark
// StringDict suitable for use as predeclared globals. Only the top level is
// expected to be a map; values themselves may be nested (dicts, lists, or
// primitives), which are converted recursively.
func goMapToStarlarkDict(params map[string]any) (starlark.StringDict, error) {
	out := make(starlark.StringDict, len(params))
	for k, v := range params {
		sv, err := goToStarlark(v)
		if err != nil {
			return nil, fmt.Errorf("param %q: %w", k, err)
		}
		out[k] = sv
	}
	return out, nil
}

func goToStarlark(v any) (starlark.Value, error) {
	switch t := v.(type) {
	case nil:
		return starlark.None, nil
	case bool:
		return starlark.Bool(t), nil
	case string:
		return starlark.String(t), nil
	case int:
		return starlark.MakeInt(t), nil
	case int64:
		return starlark.MakeInt64(t), nil
	case float64:
		return starlark.Float(t), nil
	case []any:
		elems := make([]starlark.Value, 0, len(t))
		for _, e := range t {
			sv, err := goToStarlark(e)
			if err != nil {
				return nil, err
			}
			elems = append(elems, sv)
		}
		return starlark.NewList(elems), nil
	case map[string]any:
		d := starlark.NewDict(len(t))
		for k, v := range t {
			sv, err := goToStarlark(v)
			if err != nil {
				return nil, err
			}
			if err := d.SetKey(starlark.String(k), sv); err != nil {
				return nil, err
			}
		}
		return d, nil
	default:
		return nil, fmt.Errorf("unsupported go value of type %T", v)
	}
}
