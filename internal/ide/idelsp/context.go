// Copyright (C) 2017-2026 Unstable Build, LLC
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

package idelsp

import "context"

type contextKey struct{}

// Metadata carries per-request LSP metadata through
// context.
type Metadata struct {
	ServerName string
	RootURI    string
}

// ContextWithMetadata returns a context carrying m.
func ContextWithMetadata(
	ctx context.Context, m Metadata,
) context.Context {
	return context.WithValue(ctx, contextKey{}, m)
}

func metadataFromContext(
	ctx context.Context,
) (Metadata, bool) {
	m, ok := ctx.Value(contextKey{}).(Metadata)
	return m, ok
}
