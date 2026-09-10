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

package ide

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/docmarshal/docbson"
	"unstable.build/rune/internal/localstorage"
)

// storageSentinelPresent reports whether the extension wrote its
// sentinel document through the IDE storage API. Extension storage is
// the multi-process-safe localstorage under <dataDir>/extensions,
// partitioned by extension ID.
func storageSentinelPresent(ctx context.Context, dataDir, extID, lang string) bool {
	svc := localstorage.New(ctx, filepath.Join(dataDir, "extensions"),
		docbson.Marshaler())
	defer func() { _ = svc.Close() }()
	part := storageapi.WithPartition(svc, extID)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var doc struct{ Lang string }
	if err := part.Get(ctx, "sentinel", &doc); err != nil {
		return false
	}
	return doc.Lang == lang
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
