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

package agent

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

// DefaultAgentsFile is the default filename to search for when loading
// workspace-level agent instructions.
const DefaultAgentsFile = "AGENTS.md"

// DiscoverAgentsFiles walks up from cwd to the filesystem root, collecting
// every file named filename it finds. Files are returned ordered from the
// workspace root (closest) outward (furthest ancestor), so the most
// specific instructions come first.
func DiscoverAgentsFiles(fs workspaceapi.FileSystem, cwd string, filename string) []string {
	if filename == "" {
		return nil
	}
	var paths []string
	dir := filepath.Clean(cwd)
	for {
		candidate := filepath.Join(dir, filename)
		if _, err := fs.Stat(candidate); err == nil {
			paths = append(paths, candidate)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return paths
}

// LoadAgentsFiles reads the discovered files and returns a prompt section
// suitable for appending to the system prompt. Returns an empty string
// when no files are found or all reads fail.
func LoadAgentsFiles(fs workspaceapi.FileSystem, paths []string) string {
	if len(paths) == 0 {
		return ""
	}

	var b strings.Builder
	for _, p := range paths {
		data, err := readFile(fs, p)
		if err != nil {
			slog.Warn("failed to read agents file", "path", p, "error", err)
			continue
		}
		content := strings.TrimSpace(string(data))
		if content == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		fmt.Fprintf(&b, "Contents of %s (project instructions):\n\n%s", p, content)
	}

	return b.String()
}

// readFile reads an entire file via the workspace FileSystem.
func readFile(fs workspaceapi.FileSystem, path string) ([]byte, error) {
	f, err := fs.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return io.ReadAll(f)
}
