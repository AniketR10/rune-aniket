// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.
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
