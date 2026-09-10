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

package configedit

import (
	"context"

	"github.com/unstablebuild/rune-go-sdk/api/config"
)

// NopConfig returns a Config whose getters always resolve to
// ErrNotFound and whose setters silently accept all writes. Useful in
// tests and ephemeral agents that have no workspace filesystem.
func NopConfig() Config { return nopConfig{} }

type nopConfig struct{}

func (nopConfig) GetBool(string) Bool           { return Bool{} }
func (nopConfig) GetInt(string) Int             { return Int{} }
func (nopConfig) GetFloat(string) Float         { return Float{} }
func (nopConfig) GetString(string) String       { return String{} }
func (nopConfig) GetMap(string) Map             { return Map{} }
func (nopConfig) GetSlice(string) Slice         { return Slice{} }
func (nopConfig) GetRune(string) Rune           { return Rune{} }
func (nopConfig) GetColor(string) Color         { return Color{} }
func (nopConfig) GetAttribute(string) Attribute { return Attribute{} }
func (nopConfig) GetConfig(string) ConfigValue  { return ConfigValue{} }
func (nopConfig) Iterate(func(string, any))     {}

func (nopConfig) SetBool(context.Context, string, bool, bool) error             { return nil }
func (nopConfig) SetInt(context.Context, string, int, bool) error               { return nil }
func (nopConfig) SetFloat(context.Context, string, float64, bool) error         { return nil }
func (nopConfig) SetString(context.Context, string, string, bool) error         { return nil }
func (nopConfig) AppendStringSlice(context.Context, string, string, bool) error { return nil }
func (nopConfig) RemoveStringSlice(context.Context, string, string, bool) error { return nil }

// FromSnapshot returns a Config that reads from the given SDK config
// snapshot and ignores all writes (they succeed without persisting).
// Useful for tests that want to seed values without owning a real
// .rune/config.yaml.
//
// Panics if snapshot is nil; callers that want a do-nothing Config
// should use NopConfig() instead.
func FromSnapshot(snapshot config.Config) Config {
	if snapshot == nil {
		panic("configedit: FromSnapshot called with nil snapshot; use NopConfig() instead")
	}
	return &readOnlyConfig{snapshotGetter: &snapshotGetter{snapshot: snapshot}}
}

type readOnlyConfig struct {
	*snapshotGetter
}

func (*readOnlyConfig) SetBool(context.Context, string, bool, bool) error             { return nil }
func (*readOnlyConfig) SetInt(context.Context, string, int, bool) error               { return nil }
func (*readOnlyConfig) SetFloat(context.Context, string, float64, bool) error         { return nil }
func (*readOnlyConfig) SetString(context.Context, string, string, bool) error         { return nil }
func (*readOnlyConfig) AppendStringSlice(context.Context, string, string, bool) error { return nil }
func (*readOnlyConfig) RemoveStringSlice(context.Context, string, string, bool) error { return nil }
