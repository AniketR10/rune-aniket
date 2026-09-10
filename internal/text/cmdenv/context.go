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

package cmdenv

import "context"

type cmdSubstKey struct{}

// WithCommandSubstitution opts ctx into expansion that preserves
// $(...) and backticks verbatim instead of rejecting them. The
// caller is responsible for resolving the substitutions downstream
// (a real shell, CommandSubstResolver, etc).
func WithCommandSubstitution(ctx context.Context) context.Context {
	return context.WithValue(ctx, cmdSubstKey{}, true)
}

// AllowsCommandSubstitution reports whether ctx was opted in via
// WithCommandSubstitution.
func AllowsCommandSubstitution(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	v, _ := ctx.Value(cmdSubstKey{}).(bool)
	return v
}
