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

//go:build linux

package ideupgrade

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// Default install paths for linux.
func defaultInstallRoot() string      { return filepath.Join(homeDir(), ".local") }
func defaultAppName() string          { return "rune.app" }
func defaultCLIBinaryRelPath() string { return filepath.Join("bin", "rune") }

// linuxPlatformOps implements platformOps for linux. The tarball
// extraction uses archive/tar so we don't depend on a `tar` binary
// being on PATH.
type linuxPlatformOps struct {
	httpClient *http.Client
}

func newPlatformOps(client *http.Client) platformOps {
	return linuxPlatformOps{httpClient: client}
}

func (l linuxPlatformOps) Download(
	ctx context.Context, url, dest string, progress func(n, total int64),
) error {
	return downloadOver(ctx, l.httpClient, url, dest, progress)
}

func (linuxPlatformOps) VerifySHA256(path, want string) error {
	return verifySHA256(path, want)
}

func (linuxPlatformOps) MountDMG(
	_ context.Context, _ string,
) (string, func() error, error) {
	return "", nil, ErrUnsupported
}

func (linuxPlatformOps) AssessGatekeeper(_ context.Context, _ string) error {
	return ErrUnsupported
}

func (linuxPlatformOps) VerifyCodesign(_ context.Context, _ string) error {
	return ErrUnsupported
}

func (linuxPlatformOps) Ditto(ctx context.Context, src, dst string) error {
	// Use cp -a as the linux equivalent of `ditto`.
	cmd := exec.CommandContext(ctx, "cp", "-a", src, dst)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("cp -a %s -> %s: %w: %s",
			src, dst, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (linuxPlatformOps) ExtractTarGz(
	ctx context.Context, archivePath, destDir string,
	progress func(n, total int64),
) error {
	return extractTarGz(ctx, archivePath, destDir, progress)
}

func (linuxPlatformOps) Symlink(target, linkPath string) error {
	return replaceSymlink(target, linkPath)
}

func (linuxPlatformOps) RenameAtomic(src, dst string) error {
	return os.Rename(src, dst)
}

func (linuxPlatformOps) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

func (linuxPlatformOps) FreeSpace(path string) (uint64, error) {
	var s unix.Statfs_t
	if err := unix.Statfs(path, &s); err != nil {
		return 0, fmt.Errorf("statfs %s: %w", path, err)
	}
	return uint64(s.Bavail) * uint64(s.Bsize), nil
}
