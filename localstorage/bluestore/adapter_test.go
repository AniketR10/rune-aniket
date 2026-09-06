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

package bluestore

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
)

// errService is a document.Service whose mutating methods return a
// preconfigured error, used to verify sentinel translation.
type errService struct {
	document.Service
	err error
}

func (s errService) Create(context.Context, string, any) error { return s.err }
func (s errService) Set(context.Context, string, any) error    { return s.err }

func TestAdapterTranslatesWrappedNotFound(t *testing.T) {
	ctx := context.Background()

	// Wrap the sentinel so identity comparison (err == document.ErrNotFound)
	// would fail; only errors.Is-based translation recovers it.
	wrapped := fmt.Errorf("backend layer: %w", document.ErrNotFound)

	t.Run("Create", func(t *testing.T) {
		a := AdaptTo(errService{err: wrapped})
		err := a.Create(ctx, "id", map[string]any{})
		assert.ErrorIs(t, err, storageapi.ErrNotFound)
	})

	t.Run("Set", func(t *testing.T) {
		a := AdaptTo(errService{err: wrapped})
		err := a.Set(ctx, "id", map[string]any{})
		assert.ErrorIs(t, err, storageapi.ErrNotFound)
	})
}
