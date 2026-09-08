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

package ideupgrade

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// platformOps is the abstraction over OS-specific upgrade primitives.
// One concrete implementation lives per OS (upgrader_darwin.go,
// upgrader_linux.go); tests inject a fake to drive runUpgrade
// deterministically. The type is unexported because the caller is
// not expected to swap implementations — production builds always
// use newPlatformOps(cfg.HTTPClient).
type platformOps interface {
	// Download retrieves url into dest (overwriting any existing file).
	// progress, when non-nil, is called periodically with the number
	// of bytes received and the expected total (-1 if unknown).
	Download(ctx context.Context, url, dest string, progress func(n, total int64)) error

	// VerifySHA256 computes the SHA256 of path and returns an error if
	// it does not match want (case-insensitive hex).
	VerifySHA256(path, want string) error

	// MountDMG (darwin only) attaches the dmg at dmgPath and returns
	// the mountpoint plus a callable that detaches it. Implementations
	// on non-darwin must return ErrUnsupported.
	MountDMG(ctx context.Context, dmgPath string) (mountpoint string, detach func() error, err error)

	// AssessGatekeeper (darwin only) runs `spctl --assess --type
	// execute` on the given app bundle path. Implementations on
	// non-darwin must return ErrUnsupported.
	AssessGatekeeper(ctx context.Context, appPath string) error

	// VerifyCodesign (darwin only) runs `codesign --verify --deep
	// --strict` on the given app bundle path. It validates the seal
	// integrity of every signed component, catching cases where the
	// bundle is structurally valid but has been mutated after
	// install (e.g. an AV scanner rewriting xattrs). Implementations
	// on non-darwin must return ErrUnsupported.
	VerifyCodesign(ctx context.Context, appPath string) error

	// Ditto recursively copies src to dst preserving extended
	// attributes (darwin: `ditto`; linux: cp -a equivalent).
	Ditto(ctx context.Context, src, dst string) error

	// ExtractTarGz (linux only) extracts archivePath into destDir.
	// progress, when non-nil, receives the number of compressed bytes
	// consumed so far and the total archive size.
	// Implementations on non-linux must return ErrUnsupported.
	ExtractTarGz(
		ctx context.Context, archivePath, destDir string,
		progress func(n, total int64),
	) error

	// Symlink creates a symlink at linkPath pointing to target,
	// replacing any existing symlink at linkPath.
	Symlink(target, linkPath string) error

	// RenameAtomic renames src to dst. Caller guarantees both paths
	// are on the same filesystem.
	RenameAtomic(src, dst string) error

	// RemoveAll removes path. Equivalent to os.RemoveAll, but exposed
	// so tests can record/inspect cleanups.
	RemoveAll(path string) error

	// FreeSpace returns the number of bytes available to the calling
	// user on the filesystem containing path. Used by the pre-flight
	// check so we fail before downloading a release that won't fit.
	FreeSpace(path string) (uint64, error)
}

// ErrUnsupported is returned by platformOps methods that are not
// applicable on the running OS (e.g. MountDMG on linux).
var ErrUnsupported = errors.New("operation not supported on this platform")

// downloadOver downloads url into dest using client. It is shared by
// the per-OS implementations because the logic is identical.
func downloadOver(
	ctx context.Context, client *http.Client, url, dest string,
	progress func(n, total int64),
) error {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: status %d", url, resp.StatusCode)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create %s: %w", dest, err)
	}
	defer f.Close()

	total := resp.ContentLength
	var written int64
	buf := make([]byte, 64*1024)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return fmt.Errorf("write: %w", werr)
			}
			written += int64(n)
			if progress != nil {
				progress(written, total)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return fmt.Errorf("read body: %w", rerr)
		}
	}
	return nil
}

// verifySHA256 computes the SHA256 of path and compares against want
// (case-insensitive hex). It is shared by the per-OS platformOps
// implementations.
func verifySHA256(path, want string) error {
	if want == "" {
		return errors.New("manifest sha256 is empty")
	}
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("hash %s: %w", path, err)
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("sha256 mismatch: want %s, got %s", want, got)
	}
	return nil
}

// replaceSymlink atomically replaces the symlink at linkPath with one
// pointing to target. Used by both darwin and linux implementations.
func replaceSymlink(target, linkPath string) error {
	if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
		return fmt.Errorf("mkdir symlink parent: %w", err)
	}
	tmp := linkPath + ".new"
	_ = os.Remove(tmp)
	if err := os.Symlink(target, tmp); err != nil {
		return fmt.Errorf("create symlink %s -> %s: %w", tmp, target, err)
	}
	if err := os.Rename(tmp, linkPath); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename symlink %s -> %s: %w", tmp, linkPath, err)
	}
	return nil
}
