// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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
