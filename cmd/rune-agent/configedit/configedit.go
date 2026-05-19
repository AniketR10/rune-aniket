// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.
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

// Package configedit wraps the workspace agent configuration (host
// snapshot + on-disk overlay) behind deferred-resolution value types.
//
// Callers obtain a Getter (read-only), a Setter (write-only) or a
// Config (both) and request specific keys via GetX(key) — which return
// concrete value types (Bool, Int, String, ...). Calling Resolve(ctx)
// on a value reads the latest value, consulting the in-memory overlay
// first and the underlying snapshot second. Write methods on the
// Setter mutate both the overlay and persist to .rune/config.yaml so
// the new value is visible to subsequent Resolve calls in the same
// session and across restarts.
//
// Domain-specific helpers (force_builtin_tools, max_tokens, skill
// dirs, ...) live in the packages that own the corresponding key
// names. This package only knows about generic Go types.
package configedit

import (
	"context"
	"errors"

	"github.com/unstablebuild/rune-go-sdk/term"
)

// ErrNotFound is returned by Resolve when the requested key is not
// present in the overlay or the underlying snapshot.
var ErrNotFound = errors.New("configedit: key not found")

// ErrInvalidType is returned by Resolve when the stored value has a
// type incompatible with the requested accessor.
var ErrInvalidType = errors.New("configedit: invalid value type")

// Getter is the read-only view of a Config.
type Getter interface {
	GetBool(key string) Bool
	GetInt(key string) Int
	GetFloat(key string) Float
	GetString(key string) String
	GetMap(key string) Map
	GetSlice(key string) Slice
	GetRune(key string) Rune
	GetColor(key string) Color
	GetAttribute(key string) Attribute
	GetConfig(key string) ConfigValue
	Iterate(fn func(key string, value any))
}

// Setter is the write-only view of a Config. All setters persist the
// change and update the in-memory overlay so subsequent
// Getter.GetX(key).Resolve(ctx) calls observe the new value
// immediately. When ephemeral is true the value is only written to
// the in-memory overlay and is not persisted to .rune/config.yaml;
// the change is visible within the same process but lost on restart.
type Setter interface {
	SetBool(ctx context.Context, key string, v bool, ephemeral bool) error
	SetInt(ctx context.Context, key string, v int, ephemeral bool) error
	SetFloat(ctx context.Context, key string, v float64, ephemeral bool) error
	SetString(ctx context.Context, key string, v string, ephemeral bool) error

	// AppendStringSlice adds v to the string sequence at key,
	// creating the sequence if absent. Returns ErrAlreadyPresent if
	// v is already in the sequence.
	AppendStringSlice(ctx context.Context, key string, v string, ephemeral bool) error

	// RemoveStringSlice removes v from the string sequence at key.
	// Returns ErrNotPresent if v is not in the sequence.
	RemoveStringSlice(ctx context.Context, key string, v string, ephemeral bool) error
}

// Config is the read+write view. Most callers should pass a Config so
// the receiver can both query the current state and mutate it.
type Config interface {
	Getter
	Setter
}

// ErrAlreadyPresent is returned by AppendStringSlice when the value is
// already present in the sequence.
var ErrAlreadyPresent = errors.New("configedit: value already present")

// ErrNotPresent is returned by RemoveStringSlice when the value is not
// in the sequence.
var ErrNotPresent = errors.New("configedit: value not present")

// Bool defers reading a boolean configuration value.
type Bool struct {
	resolve func(context.Context) (bool, error)
}

// Resolve returns the current value or an error.
func (b Bool) Resolve(ctx context.Context) (bool, error) {
	if b.resolve == nil {
		return false, ErrNotFound
	}
	return b.resolve(ctx)
}

// Int defers reading an integer configuration value.
type Int struct {
	resolve func(context.Context) (int, error)
}

// Resolve returns the current value or an error.
func (i Int) Resolve(ctx context.Context) (int, error) {
	if i.resolve == nil {
		return 0, ErrNotFound
	}
	return i.resolve(ctx)
}

// Float defers reading a floating-point configuration value.
type Float struct {
	resolve func(context.Context) (float64, error)
}

// Resolve returns the current value or an error.
func (f Float) Resolve(ctx context.Context) (float64, error) {
	if f.resolve == nil {
		return 0, ErrNotFound
	}
	return f.resolve(ctx)
}

// String defers reading a string configuration value.
type String struct {
	resolve func(context.Context) (string, error)
}

// Resolve returns the current value or an error.
func (s String) Resolve(ctx context.Context) (string, error) {
	if s.resolve == nil {
		return "", ErrNotFound
	}
	return s.resolve(ctx)
}

// Map defers reading a map configuration value.
type Map struct {
	resolve func(context.Context) (map[string]any, error)
}

// Resolve returns the current value or an error.
func (m Map) Resolve(ctx context.Context) (map[string]any, error) {
	if m.resolve == nil {
		return nil, ErrNotFound
	}
	return m.resolve(ctx)
}

// Slice defers reading a sequence configuration value.
type Slice struct {
	resolve func(context.Context) ([]any, error)
}

// Resolve returns the current value or an error.
func (s Slice) Resolve(ctx context.Context) ([]any, error) {
	if s.resolve == nil {
		return nil, ErrNotFound
	}
	return s.resolve(ctx)
}

// Rune defers reading a rune configuration value.
type Rune struct {
	resolve func(context.Context) (rune, error)
}

// Resolve returns the current value or an error.
func (r Rune) Resolve(ctx context.Context) (rune, error) {
	if r.resolve == nil {
		return 0, ErrNotFound
	}
	return r.resolve(ctx)
}

// Color defers reading a terminal color configuration value.
type Color struct {
	resolve func(context.Context) (term.Color, error)
}

// Resolve returns the current value or an error.
func (c Color) Resolve(ctx context.Context) (term.Color, error) {
	if c.resolve == nil {
		return 0, ErrNotFound
	}
	return c.resolve(ctx)
}

// Attribute defers reading a terminal-attribute configuration value.
type Attribute struct {
	resolve func(context.Context) (term.AttrMask, error)
}

// Resolve returns the current value or an error.
func (a Attribute) Resolve(ctx context.Context) (term.AttrMask, error) {
	if a.resolve == nil {
		return 0, ErrNotFound
	}
	return a.resolve(ctx)
}

// ConfigValue defers reading a nested Config (the rune-agent extension
// stores some keys under nested maps, e.g. provider configs).
// Resolving yields a Getter rooted at that nested map.
type ConfigValue struct {
	resolve func(context.Context) (Getter, error)
}

// Resolve returns the current nested config or an error.
func (c ConfigValue) Resolve(ctx context.Context) (Getter, error) {
	if c.resolve == nil {
		return nil, ErrNotFound
	}
	return c.resolve(ctx)
}

// Helper constructors for ad-hoc values (mainly for tests).

// NewBool returns a Bool whose Resolve invokes fn.
func NewBool(fn func(context.Context) (bool, error)) Bool { return Bool{resolve: fn} }

// NewInt returns an Int whose Resolve invokes fn.
func NewInt(fn func(context.Context) (int, error)) Int { return Int{resolve: fn} }

// NewFloat returns a Float whose Resolve invokes fn.
func NewFloat(fn func(context.Context) (float64, error)) Float { return Float{resolve: fn} }

// NewString returns a String whose Resolve invokes fn.
func NewString(fn func(context.Context) (string, error)) String {
	return String{resolve: fn}
}

// NewMap returns a Map whose Resolve invokes fn.
func NewMap(fn func(context.Context) (map[string]any, error)) Map { return Map{resolve: fn} }

// NewSlice returns a Slice whose Resolve invokes fn.
func NewSlice(fn func(context.Context) ([]any, error)) Slice { return Slice{resolve: fn} }

// NewRune returns a Rune whose Resolve invokes fn.
func NewRune(fn func(context.Context) (rune, error)) Rune { return Rune{resolve: fn} }

// NewColor returns a Color whose Resolve invokes fn.
func NewColor(fn func(context.Context) (term.Color, error)) Color { return Color{resolve: fn} }

// NewAttribute returns an Attribute whose Resolve invokes fn.
func NewAttribute(fn func(context.Context) (term.AttrMask, error)) Attribute {
	return Attribute{resolve: fn}
}

// NewConfigValue returns a ConfigValue whose Resolve invokes fn.
func NewConfigValue(fn func(context.Context) (Getter, error)) ConfigValue {
	return ConfigValue{resolve: fn}
}
