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

package gemini

import (
	"context"

	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

// fallbackIterator wraps the live Gemini catalog iterator and yields the
// static catalog when the live query fails before producing any entry.
// Listing models requires a working API key (the generativelanguage
// models endpoint rejects unauthenticated requests), so a bad or missing
// key would otherwise leave callers — notably bootstrap, which runs
// before a key is verified — with an empty list. The underlying error is
// still surfaced through Err so the user can see and fix the cause
// (e.g. an invalid key) the next time the model is actually used.
type fallbackIterator struct {
	live     iterator.Iterator[llmapi.ModelEntry]
	fallback []llmapi.ModelEntry

	yielded   bool // a live entry was returned
	failed    bool // the live query errored
	liveErr   error
	fallbackI int
}

func withStaticFallback(
	live iterator.Iterator[llmapi.ModelEntry], fallback []llmapi.ModelEntry,
) iterator.Iterator[llmapi.ModelEntry] {
	return &fallbackIterator{live: live, fallback: fallback}
}

func (f *fallbackIterator) Next(ctx context.Context) (llmapi.ModelEntry, bool) {
	if !f.failed {
		if entry, ok := f.live.Next(ctx); ok {
			f.yielded = true
			return entry, true
		}
		// The live iterator stopped. Distinguish clean exhaustion from a
		// failure: only fall back when the query failed without yielding
		// anything. A successful empty list is left empty; a mid-stream
		// failure keeps its partial results and surfaces the error.
		f.liveErr = f.live.Err()
		f.failed = true
		if f.liveErr == nil || f.yielded {
			return llmapi.ModelEntry{}, false
		}
	}

	if f.fallbackI >= len(f.fallback) {
		return llmapi.ModelEntry{}, false
	}
	entry := f.fallback[f.fallbackI]
	f.fallbackI++
	return entry, true
}

// Err returns the live query error even when the static fallback was
// served, so callers can surface the underlying cause.
func (f *fallbackIterator) Err() error {
	return f.liveErr
}

func (f *fallbackIterator) Close() error {
	return f.live.Close()
}
