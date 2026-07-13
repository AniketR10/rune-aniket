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

package llamaserver

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/unstablebuild/rune-go-sdk/iterator"
)

// serverPackageID is the package identifier under which the llama-server
// binary is installed (`pkg install llama-server`).
const serverPackageID = "llama-server"

// serverBinaryName is the executable name shipped by the llama-server
// package.
const serverBinaryName = "llama-server"

// ErrServerNotInstalled is returned by a BinaryLocator when the
// llama-server binary cannot be resolved via the package manager. The
// Service translates it into the user-facing install instruction.
var ErrServerNotInstalled = errors.New("llamaserver: llama-server binary not installed")

// BinaryLocator resolves the absolute path to the llama-server executable.
// Locate returns ErrServerNotInstalled when the binary is unavailable so the
// Service can distinguish "not installed" from real failures.
type BinaryLocator interface {
	Locate(ctx context.Context) (string, error)
}

// pkgLibDirLister is the subset of the package manager the locator needs. It
// mirrors ide's pkgManager.LibDir so the production locator takes a narrow,
// mockable dependency. LibDir resolves paths on the workspace host, which may
// be remote, so the locator must never touch the local filesystem or $PATH.
type pkgLibDirLister interface {
	LibDir(ctx context.Context, pkgID string) (iterator.Iterator[string], error)
}

// fixedLocator returns a preconfigured path; used when the config carries an
// explicit ServerBinPath override.
type fixedLocator struct{ path string }

// NewFixedLocator returns a BinaryLocator that always resolves to path.
func NewFixedLocator(path string) BinaryLocator { return fixedLocator{path: path} }

func (l fixedLocator) Locate(context.Context) (string, error) {
	if l.path == "" {
		return "", ErrServerNotInstalled
	}
	return l.path, nil
}

// pkgLocator resolves llama-server through the package manager. It never
// consults $PATH or the local filesystem because the workspace, and hence the
// server, may run on a remote host.
type pkgLocator struct {
	pkg pkgLibDirLister
}

// NewPkgLocator returns a BinaryLocator that resolves llama-server via the
// llama-server package's LibDir. It panics if pkg is nil.
func NewPkgLocator(pkg pkgLibDirLister) BinaryLocator {
	if pkg == nil {
		panic("llamaserver: NewPkgLocator: pkg must not be nil")
	}
	return &pkgLocator{pkg: pkg}
}

func (l *pkgLocator) Locate(ctx context.Context) (string, error) {
	it, err := l.pkg.LibDir(ctx, serverPackageID)
	if err != nil {
		return "", ErrServerNotInstalled
	}
	defer func() { _ = it.Close() }()
	paths, err := iterator.ToSlice(ctx, it)
	if err != nil {
		return "", ErrServerNotInstalled
	}
	// LibDir enumerates the files the package shipped on the workspace host,
	// so a base-name match is proof the binary exists there; the process is
	// launched through the (possibly remote) executor, never locally.
	for _, path := range paths {
		if filepath.Base(path) == serverBinaryName {
			return path, nil
		}
	}
	return "", ErrServerNotInstalled
}
