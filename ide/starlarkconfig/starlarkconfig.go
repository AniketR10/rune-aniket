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

// Package starlarkconfig contains the shared Starlark <-> Go conversion
// and config-decode helpers used by both the main rune config loader
// (ide/config_starlark.go) and the extension/package config merge path
// (ide/idepkg/manager.go).
//
// The decoder understands two modes controlled by Source.Base:
//
//   - Base mode (Base == nil): the script must bind a top-level
//     `config = {...}` dict. The returned map is that dict.
//   - Overlay mode (Base != nil): `config` is predeclared with the given
//     base map. The script may mutate it (config["x"] = y, etc.) or rebind
//     it entirely. The returned map is the final value of `config`.
//
// The helpers are intentionally low-level: callers wrap them with their own
// domain concerns (YAML fallback, filename suffix detection, param
// injection, serialization of merged results, etc.).
package starlarkconfig

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

// ConfigGlobal is the name of the top-level dict the script reads from and
// writes to. Anything else is ignored.
const ConfigGlobal = "config"

// ErrMissingConfig is returned by Decode in base mode when the script does
// not bind a top-level `config` dict. Callers that treat a missing binding
// as an empty configuration (for example, a user config file that only
// contains comments) can branch on this with errors.Is.
var ErrMissingConfig = errors.New("starlark: missing top-level \"config\" dict")

// Source describes one config invocation. See package docs for the two modes.
type Source struct {
	// Src is the script source.
	Src []byte

	// Filename is used in error messages and as the Starlark thread name.
	// Defaults to "config.star" when empty.
	Filename string

	// Params are exposed as extra predeclared globals. Keys must be valid
	// Starlark identifiers and must not collide with ConfigGlobal when
	// Base is non-nil.
	Params map[string]any

	// Base, when non-nil, switches the decoder to overlay mode.
	Base map[string]any
}

// Decode parses and executes src. See Source for the two modes.
//
// The script runs with stdlib Starlark only — no file I/O, no load(), no
// environment access. Conditionals (if/elif/else) and loops (for) at the
// top level are enabled via FileOptions.TopLevelControl.
func Decode(s Source) (map[string]any, error) {
	filename := s.Filename
	if filename == "" {
		filename = "config.star"
	}

	thread := &starlark.Thread{
		Name:  filename,
		Print: func(*starlark.Thread, string) {},
		Load: func(_ *starlark.Thread, module string) (starlark.StringDict, error) {
			return nil, fmt.Errorf("load() is not allowed in %s: cannot load %q",
				filename, module)
		},
	}
	opts := &syntax.FileOptions{
		TopLevelControl: true,
		GlobalReassign:  true,
		Recursion:       true,
	}

	predeclared, err := StringDictFromMap(s.Params)
	if err != nil {
		return nil, fmt.Errorf("starlark: encode params: %w", err)
	}

	if s.Base != nil {
		baseVal, err := FromGo(s.Base)
		if err != nil {
			return nil, fmt.Errorf("starlark: encode base config: %w", err)
		}
		if _, taken := predeclared[ConfigGlobal]; taken {
			return nil, fmt.Errorf("starlark: param %q collides with "+
				"predeclared base config", ConfigGlobal)
		}
		predeclared[ConfigGlobal] = baseVal
	}

	globals, err := starlark.ExecFileOptions(opts, thread, filename, s.Src, predeclared)
	if err != nil {
		var evalErr *starlark.EvalError
		if errors.As(err, &evalErr) {
			return nil, fmt.Errorf("starlark: %s", evalErr.Backtrace())
		}
		return nil, fmt.Errorf("starlark: %w", err)
	}

	// If the script never rebinds `config`, it won't appear in globals —
	// in overlay mode the predeclared dict still holds the latest value
	// because dict mutations modify the object in place.
	val, ok := globals[ConfigGlobal]
	if !ok {
		val, ok = predeclared[ConfigGlobal]
	}
	if !ok {
		return nil, ErrMissingConfig
	}
	dict, ok := val.(*starlark.Dict)
	if !ok {
		return nil, fmt.Errorf("starlark: %q must be a dict, got %s",
			ConfigGlobal, val.Type())
	}
	out, err := DictToMap(dict)
	if err != nil {
		return nil, fmt.Errorf("starlark: %w", err)
	}
	return out, nil
}

// ToGo converts an arbitrary Starlark value to the closest Go equivalent
// used by the rest of the config pipeline (map[string]any / []any / scalar
// primitives / string).
func ToGo(v starlark.Value) (any, error) {
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
			gv, err := ToGo(item)
			if err != nil {
				return nil, err
			}
			out = append(out, gv)
		}
		return out, nil
	case starlark.Tuple:
		out := make([]any, 0, t.Len())
		for i := range t.Len() {
			gv, err := ToGo(t.Index(i))
			if err != nil {
				return nil, err
			}
			out = append(out, gv)
		}
		return out, nil
	case *starlark.Dict:
		return DictToMap(t)
	default:
		return nil, fmt.Errorf("unsupported starlark value of type %s", v.Type())
	}
}

// DictToMap converts a Starlark dict with string keys to a Go map.
// Non-string keys are rejected because the downstream config consumers
// (config.MapConfig) assume string-keyed maps.
func DictToMap(d *starlark.Dict) (map[string]any, error) {
	out := make(map[string]any, d.Len())
	for _, item := range d.Items() {
		keyVal, value := item[0], item[1]
		keyStr, ok := keyVal.(starlark.String)
		if !ok {
			return nil, fmt.Errorf("dict keys must be strings, got %s", keyVal.Type())
		}
		gv, err := ToGo(value)
		if err != nil {
			return nil, err
		}
		out[string(keyStr)] = gv
	}
	return out, nil
}

// StringDictFromMap converts a Go map[string]any into a flat
// starlark.StringDict suitable for use as predeclared globals. Only the
// top level is expected to be a map; values themselves may be nested
// (dicts, lists, or primitives), which are converted recursively.
func StringDictFromMap(params map[string]any) (starlark.StringDict, error) {
	out := make(starlark.StringDict, len(params))
	for k, v := range params {
		sv, err := FromGo(v)
		if err != nil {
			return nil, fmt.Errorf("param %q: %w", k, err)
		}
		out[k] = sv
	}
	return out, nil
}

// FromGo converts a Go value to its closest Starlark equivalent. Map keys
// are emitted in sorted order so the resulting dicts have a deterministic
// iteration order downstream.
func FromGo(v any) (starlark.Value, error) {
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
			sv, err := FromGo(e)
			if err != nil {
				return nil, err
			}
			elems = append(elems, sv)
		}
		return starlark.NewList(elems), nil
	case map[string]any:
		d := starlark.NewDict(len(t))
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sv, err := FromGo(t[k])
			if err != nil {
				return nil, err
			}
			// SetKey on a freshly-allocated, unfrozen dict keyed by
			// a string is infallible: insert only errors on frozen
			// tables or unhashable keys, and neither applies here.
			_ = d.SetKey(starlark.String(k), sv)
		}
		return d, nil
	default:
		return nil, fmt.Errorf("unsupported go value of type %T", v)
	}
}

// Write writes v as a Starlark literal (None/True/False, quoted strings,
// numbers, [ ... ] lists, { ... } dicts) to w. Map keys are emitted sorted
// for determinism.
func Write(w io.Writer, v any, indent int) error {
	switch t := v.(type) {
	case nil:
		_, err := io.WriteString(w, "None")
		return err
	case bool:
		if t {
			_, err := io.WriteString(w, "True")
			return err
		}
		_, err := io.WriteString(w, "False")
		return err
	case string:
		_, err := fmt.Fprintf(w, "%q", t)
		return err
	case int, int64, float64:
		_, err := fmt.Fprintf(w, "%v", t)
		return err
	case []any:
		if len(t) == 0 {
			_, err := io.WriteString(w, "[]")
			return err
		}
		if _, err := io.WriteString(w, "[\n"); err != nil {
			return err
		}
		for i, item := range t {
			if _, err := io.WriteString(w, strings.Repeat(" ", indent+4)); err != nil {
				return err
			}
			if err := Write(w, item, indent+4); err != nil {
				return err
			}
			if i < len(t)-1 {
				if _, err := io.WriteString(w, ","); err != nil {
					return err
				}
			}
			if _, err := io.WriteString(w, "\n"); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(w, strings.Repeat(" ", indent)); err != nil {
			return err
		}
		_, err := io.WriteString(w, "]")
		return err
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if len(keys) == 0 {
			_, err := io.WriteString(w, "{}")
			return err
		}
		if _, err := io.WriteString(w, "{\n"); err != nil {
			return err
		}
		for i, k := range keys {
			if _, err := io.WriteString(w, strings.Repeat(" ", indent+4)); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(w, "%q: ", k); err != nil {
				return err
			}
			if err := Write(w, t[k], indent+4); err != nil {
				return err
			}
			if i < len(keys)-1 {
				if _, err := io.WriteString(w, ","); err != nil {
					return err
				}
			}
			if _, err := io.WriteString(w, "\n"); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(w, strings.Repeat(" ", indent)); err != nil {
			return err
		}
		_, err := io.WriteString(w, "}")
		return err
	default:
		_, err := fmt.Fprintf(w, "%q", fmt.Sprint(t))
		return err
	}
}

// WriteConfigFileAtomic writes cfg to path as a source file assigning the
// top-level `config` variable: `config = { ... }`. The write is atomic
// (temp file + rename) and creates parent directories if needed.
func WriteConfigFileAtomic(path string, cfg map[string]any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o777); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".config-*.star")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	return atomicSwap(tmp, cfg, tmp.Name(), path, os.Rename, os.Remove)
}

// atomicSwap writes cfg into w, closes it, and then renames src → dest.
// On any error the temp file at src is best-effort cleaned up.
//
// The rename and cleanup functions are injected so tests can exercise
// every error path without relying on hard-to-provoke filesystem states.
// The production caller always passes os.Rename and os.Remove.
func atomicSwap(
	w io.WriteCloser,
	cfg map[string]any,
	src, dest string,
	rename func(oldpath, newpath string) error,
	remove func(name string) error,
) error {
	if err := writeConfigAssignment(w, cfg); err != nil {
		_ = w.Close()
		_ = remove(src)
		return err
	}
	if err := w.Close(); err != nil {
		_ = remove(src)
		return fmt.Errorf("close temp: %w", err)
	}
	if err := rename(src, dest); err != nil {
		_ = remove(src)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// writeConfigAssignment writes `config = <cfg>\n` to w. It is extracted
// from WriteConfigFileAtomic so each underlying write error path is
// directly testable with an injected io.Writer.
func writeConfigAssignment(w io.Writer, cfg map[string]any) error {
	if _, err := io.WriteString(w, "config = "); err != nil {
		return fmt.Errorf("write prefix: %w", err)
	}
	if err := Write(w, cfg, 0); err != nil {
		return fmt.Errorf("write value: %w", err)
	}
	if _, err := io.WriteString(w, "\n"); err != nil {
		return fmt.Errorf("write newline: %w", err)
	}
	return nil
}
