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

package agentools

import (
	"context"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"sync"

	"unstable.build/rune/cmd/rune-agent/agent"
)

// FileTracker records content hashes of files when they are read, so
// that write tools can detect stale content before applying changes.
// All methods are safe for concurrent use.
type FileTracker struct {
	hashes sync.Map // absolute path → [sha256.Size]byte

	readsMu sync.Mutex
	reads   map[string]map[readKey][]string // absPath → readKey → tool call IDs

	discoveriesMu sync.Mutex
	discoveries   map[string][]string // toolCallID → []absPath
}

// readKey identifies a specific kind of read on a file. Two reads are
// considered duplicates (and thus the earlier one is stale) only when
// both toolName and variant match. The variant allows tools like
// read_file to distinguish ranged reads (e.g. "100:50") from full-file
// reads (""), so that reading lines 1-50 does not drop a prior read
// of lines 100-150.
type readKey struct {
	toolName string
	variant  string
}

// NewFileTracker creates a new FileTracker.
func NewFileTracker() *FileTracker {
	return &FileTracker{
		reads:       make(map[string]map[readKey][]string),
		discoveries: make(map[string][]string),
	}
}

// Record stores the SHA-256 hash of data for the given absolute path.
func (ft *FileTracker) Record(absPath string, data []byte) {
	hash := sha256.Sum256(data)
	ft.hashes.Store(absPath, hash)
}

// Verify checks that the current file content matches the hash
// recorded by the last Record call. It returns nil if no hash was
// previously recorded (the file was never read through the tracker).
func (ft *FileTracker) Verify(absPath string, currentData []byte) error {
	stored, ok := ft.hashes.Load(absPath)
	if !ok {
		return nil
	}
	current := sha256.Sum256(currentData)
	if current != stored.([sha256.Size]byte) {
		return fmt.Errorf(
			"file %q has changed since it was last read; read the file again before editing",
			filepath.Base(absPath),
		)
	}
	return nil
}

// TrackRead returns stale read IDs for the given (toolName, variant,
// absPath) triple, clears them, and records the current tool call
// (from ctx) as a new read. Only previous calls with the same tool
// name and variant on the same file are considered stale — other
// tools' or variants' results for the same file are preserved.
//
// The variant distinguishes different "views" of the same file.
// For example, read_file passes the range ("100:50") so that reading
// one range does not drop a prior read of a different range.
// Tools without range semantics should pass "".
//
// It is safe to call on a nil receiver.
func (ft *FileTracker) TrackRead(ctx context.Context, toolName, absPath, variant string) []string {
	if ft == nil {
		return nil
	}
	key := readKey{toolName: toolName, variant: variant}
	ft.readsMu.Lock()
	var staleIDs []string
	if byKey := ft.reads[absPath]; byKey != nil {
		staleIDs = byKey[key]
		delete(byKey, key)
	}
	if id := agent.ParentToolCallID(ctx); id != "" {
		if ft.reads[absPath] == nil {
			ft.reads[absPath] = make(map[readKey][]string)
		}
		ft.reads[absPath][key] = append(ft.reads[absPath][key], id)
	}
	ft.readsMu.Unlock()
	return staleIDs
}

// RecordRead appends a tool call ID for the given (toolName, variant, path).
func (ft *FileTracker) RecordRead(absPath, toolName, variant, toolCallID string) {
	ft.readsMu.Lock()
	if ft.reads[absPath] == nil {
		ft.reads[absPath] = make(map[readKey][]string)
	}
	key := readKey{toolName: toolName, variant: variant}
	ft.reads[absPath][key] = append(ft.reads[absPath][key], toolCallID)
	ft.readsMu.Unlock()
}

// StaleReads returns and clears all tracked read tool call IDs for the
// given path across all tools and variants. Used by file-modifying
// tools (apply_patch, format_file) to invalidate every previous read
// of a changed file. It is safe to call on a nil receiver.
func (ft *FileTracker) StaleReads(absPath string) []string {
	if ft == nil {
		return nil
	}
	ft.readsMu.Lock()
	byKey := ft.reads[absPath]
	delete(ft.reads, absPath)
	ft.readsMu.Unlock()
	var ids []string
	for _, keyIDs := range byKey {
		ids = append(ids, keyIDs...)
	}
	return ids
}

// Forget removes the recorded hash and all read IDs for the given path.
// It is safe to call on a nil receiver.
func (ft *FileTracker) Forget(absPath string) {
	if ft == nil {
		return
	}
	ft.hashes.Delete(absPath)
	ft.readsMu.Lock()
	delete(ft.reads, absPath)
	ft.readsMu.Unlock()
}

// TrackDiscovery records that the current tool call surfaced the given
// file paths. When any of these paths is later opened by a file-reading
// tool, ConsumeDiscoveries returns this tool call ID for dropping.
// It is safe to call on a nil receiver.
func (ft *FileTracker) TrackDiscovery(ctx context.Context, paths []string) {
	if ft == nil || len(paths) == 0 {
		return
	}
	id := agent.ParentToolCallID(ctx)
	if id == "" {
		return
	}
	ft.discoveriesMu.Lock()
	ft.discoveries[id] = paths
	ft.discoveriesMu.Unlock()
}

// ConsumeDiscoveries returns and removes all discovery tool call IDs
// that previously surfaced the given path. Called by file-reading and
// file-modifying tools to drop the discovery results that led to this
// file being opened. It is safe to call on a nil receiver.
func (ft *FileTracker) ConsumeDiscoveries(absPath string) []string {
	if ft == nil {
		return nil
	}
	ft.discoveriesMu.Lock()
	var consumed []string
	for id, paths := range ft.discoveries {
		for _, p := range paths {
			if p == absPath {
				consumed = append(consumed, id)
				delete(ft.discoveries, id)
				break
			}
		}
	}
	ft.discoveriesMu.Unlock()
	return consumed
}
