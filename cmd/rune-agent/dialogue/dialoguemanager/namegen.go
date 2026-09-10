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

package dialoguemanager

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"

	petname "github.com/dustinkirkland/golang-petname"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
)

const maxNameRetries = 10

// GenerateUniqueID generates a petname-based dialogue ID with the
// given prefix, checking the store to ensure no existing dialogue
// has the same ID. The resulting format is "{prefix}{petname}".
// After maxNameRetries collisions it appends a short random suffix
// to guarantee uniqueness.
func GenerateUniqueID(ctx context.Context, store Store, prefix string) string {
	for range maxNameRetries {
		id := prefix + petname.Generate(2, "-")
		_, err := store.Get(ctx, id)
		switch {
		case errors.Is(err, storageapi.ErrNotFound):
			return id // unique
		case err != nil:
			slog.Warn("GenerateUniqueID: store check failed, using generated ID",
				"id", id, "error", err)
			return id
		}
		// err == nil means collision — retry.
	}
	// Fallback: append random suffix.
	var buf [4]byte
	_, _ = rand.Read(buf[:])
	return prefix + petname.Generate(2, "-") + "-" + fmt.Sprintf("%x", buf[:])
}
