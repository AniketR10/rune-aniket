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

package walkdir

import "context"

type workerCountContextKey struct{}

// ContextWithWorkerCount returns a context configured to use workers as the
// walkdir worker count. Non-positive values are ignored by walkdir operations.
func ContextWithWorkerCount(ctx context.Context, workers int) context.Context {
	return context.WithValue(ctx, workerCountContextKey{}, workers)
}

func workerCountFromContext(ctx context.Context) int {
	workers, ok := ctx.Value(workerCountContextKey{}).(int)
	if !ok || workers <= 0 {
		return defaultWorkers
	}
	return workers
}

type scanBufferSizeContextKey struct{}

// ContextWithScanBufferSize returns a context configured to use the given
// per-file scan buffer size (in bytes) for ReadLines. Files containing a
// single line longer than the default bufio.MaxScanTokenSize (64 KiB) are
// otherwise silently skipped by the underlying scanner; callers that know
// their inputs may contain long lines (e.g. single-line JSON documents) can
// raise this cap. Non-positive values are ignored by walkdir operations.
func ContextWithScanBufferSize(ctx context.Context, size int) context.Context {
	return context.WithValue(ctx, scanBufferSizeContextKey{}, size)
}

func scanBufferSizeFromContext(ctx context.Context) int {
	size, ok := ctx.Value(scanBufferSizeContextKey{}).(int)
	if !ok || size <= 0 {
		return 0
	}
	return size
}
