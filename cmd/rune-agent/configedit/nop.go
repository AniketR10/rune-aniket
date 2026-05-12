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

func (nopConfig) SetBool(context.Context, string, bool) error             { return nil }
func (nopConfig) SetInt(context.Context, string, int) error               { return nil }
func (nopConfig) SetFloat(context.Context, string, float64) error         { return nil }
func (nopConfig) SetString(context.Context, string, string) error         { return nil }
func (nopConfig) AppendStringSlice(context.Context, string, string) error { return nil }
func (nopConfig) RemoveStringSlice(context.Context, string, string) error { return nil }

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

func (*readOnlyConfig) SetBool(context.Context, string, bool) error             { return nil }
func (*readOnlyConfig) SetInt(context.Context, string, int) error               { return nil }
func (*readOnlyConfig) SetFloat(context.Context, string, float64) error         { return nil }
func (*readOnlyConfig) SetString(context.Context, string, string) error         { return nil }
func (*readOnlyConfig) AppendStringSlice(context.Context, string, string) error { return nil }
func (*readOnlyConfig) RemoveStringSlice(context.Context, string, string) error { return nil }
