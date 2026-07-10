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

// Package langext packages the language-agnostic machinery a Rune
// language extension needs to discover a project root for an opened file
// and initialize its language server rooted there, deduped per root.
//
// A language plugs in through ProjectConfig: the LSP language id, the
// project-root marker files to walk for, a predicate that recognizes the
// language's files, and a callback that performs the language-specific
// bring-up for a discovered root. Initializer wires those to editor open
// events and guarantees InitRoot runs at most once per project root.
package langext

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

// ProjectConfig is the per-language plug-in contract consumed by
// Initializer. Every field is required.
type ProjectConfig struct {
	// LanguageID is the LSP language id (e.g. "python", "go", "rust").
	LanguageID string

	// Markers are project-root marker files or directories, probed at
	// each candidate directory while walking up from an opened file
	// (e.g. "pyproject.toml", "go.mod", "Cargo.toml"). A directory
	// matches if any marker Stats successfully.
	Markers []string

	// FileMatch reports whether an opened file belongs to this language
	// (e.g. its extension is ".py"). Non-matching opens are ignored
	// cheaply before any filesystem walk.
	FileMatch func(uri workspaceapi.URI) bool

	// InitRoot performs the language-specific bring-up for a discovered
	// project root: environment setup plus building and calling
	// lsp.Initialize with the nested root URI. It runs in a background
	// goroutine; Initializer guarantees exactly one call per root.
	InitRoot func(ctx context.Context, root Root) error

	// WatchEvents lists the editor event types that drive discovery,
	// defaulting to {EventTypeOpen} when empty. Include EventTypeChange
	// and EventTypeCreate for a language whose files are written
	// out-of-band, so nested projects come up without an editor buffer.
	WatchEvents []textapi.EventType
}

// Root describes a discovered project root.
type Root struct {
	// Dir is the filesystem path of the project root.
	Dir string
	// URI is the file:// URI of the project root, used for RootURI.
	URI string
	// RelPath is the root path relative to the workspace root.
	RelPath string
}

// FindProjectRoot walks upward from fileURI looking for the nearest
// directory that carries one of cfg.Markers, stopping at the workspace
// root identified by workspaceRootURI. The opened file must live inside
// the workspace root; otherwise it returns found=false without probing.
//
// The nearest enclosing marked directory wins, so a file deep in a tree
// initializes against its own project rather than an ancestor. A file
// with no enclosing marker (within the workspace) yields found=false, so
// a stray source file does not spin up a language server.
func FindProjectRoot(
	fs workspaceapi.FileSystem, workspaceRootURI workspaceapi.URI, fileURI workspaceapi.URI,
	markers []string,
) (Root, bool) {
	wsDir := filepath.Clean(workspaceRootURI.Path())
	file := filepath.Clean(fileURI.Path())

	rel, err := filepath.Rel(wsDir, file)
	if err != nil || rel == ".." || hasParentPrefix(rel) {
		return Root{}, false
	}

	// The Rel check above guarantees file is within wsDir, so the walk
	// always terminates at wsDir.
	for dir := filepath.Dir(file); ; dir = filepath.Dir(dir) {
		if dirHasMarker(fs, dir, markers) {
			return rootForDir(fs, wsDir, dir), true
		}
		if dir == wsDir {
			return Root{}, false
		}
	}
}

// dirHasMarker reports whether dir contains any of the marker files or
// directories.
func dirHasMarker(fs workspaceapi.FileSystem, dir string, markers []string) bool {
	for _, m := range markers {
		if _, err := fs.Stat(filepath.Join(dir, m)); err == nil {
			return true
		}
	}
	return false
}

// rootForDir builds a Root for an absolute project directory.
func rootForDir(fs workspaceapi.FileSystem, wsDir, dir string) Root {
	rel, err := filepath.Rel(wsDir, dir)
	if err != nil {
		rel = ""
	}
	if rel == "." {
		rel = ""
	}
	return Root{Dir: dir, URI: uriForDir(fs, dir), RelPath: rel}
}

// uriForDir returns the file:// URI for an absolute directory. ty/ruff
// and other language servers run on the same host as the workspace
// files, so the path is rewritten to the file:// scheme they expect
// regardless of the workspace's own URI scheme.
func uriForDir(fs workspaceapi.FileSystem, dir string) string {
	if uri, err := fs.URI(dir); err == nil {
		return fmt.Sprintf("file://%s", uri.Path())
	}
	return fmt.Sprintf("file://%s", dir)
}

// hasParentPrefix reports whether a cleaned relative path escapes its
// base via a leading "../" segment.
func hasParentPrefix(rel string) bool {
	return len(rel) >= 3 && rel[0] == '.' && rel[1] == '.' && rel[2] == filepath.Separator
}
