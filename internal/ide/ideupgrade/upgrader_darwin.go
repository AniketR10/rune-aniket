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

//go:build darwin

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

// Default install paths for darwin.
func defaultInstallRoot() string      { return "/Applications" }
func defaultAppName() string          { return "Rune.app" }
func defaultCLIBinaryRelPath() string { return filepath.Join("Contents", "MacOS", "rune") }

// darwinPlatformOps implements platformOps using the macOS native
// tools `hdiutil`, `spctl` and `ditto` plus stdlib download/symlink
// helpers.
type darwinPlatformOps struct {
	httpClient *http.Client
}

// newPlatformOps returns the production platformOps for the current
// OS. Tests in this package replace it with a fake.
func newPlatformOps(client *http.Client) platformOps {
	return darwinPlatformOps{httpClient: client}
}

func (d darwinPlatformOps) Download(
	ctx context.Context, url, dest string, progress func(n, total int64),
) error {
	return downloadOver(ctx, d.httpClient, url, dest, progress)
}

func (darwinPlatformOps) VerifySHA256(path, want string) error {
	return verifySHA256(path, want)
}

func (darwinPlatformOps) MountDMG(
	ctx context.Context, dmgPath string,
) (string, func() error, error) {
	mount, err := os.MkdirTemp("", "rune-upgrade-mount-")
	if err != nil {
		return "", nil, fmt.Errorf("mkdir mount: %w", err)
	}
	cmd := exec.CommandContext(ctx, "hdiutil", "attach",
		"-nobrowse", "-quiet", "-mountpoint", mount, dmgPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.Remove(mount)
		return "", nil, fmt.Errorf("hdiutil attach: %w: %s", err, strings.TrimSpace(string(out)))
	}
	detach := func() error {
		c := exec.Command("hdiutil", "detach", "-quiet", mount)
		out, derr := c.CombinedOutput()
		_ = os.Remove(mount)
		if derr != nil {
			return fmt.Errorf("hdiutil detach: %w: %s", derr, strings.TrimSpace(string(out)))
		}
		return nil
	}
	return mount, detach, nil
}

func (darwinPlatformOps) AssessGatekeeper(ctx context.Context, appPath string) error {
	cmd := exec.CommandContext(ctx, "spctl", "--assess", "--type", "execute", appPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("spctl assess %s: %w: %s",
			appPath, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (darwinPlatformOps) VerifyCodesign(ctx context.Context, appPath string) error {
	cmd := exec.CommandContext(ctx, "codesign",
		"--verify", "--deep", "--strict", "--verbose=2", appPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("codesign verify %s: %w: %s",
			appPath, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (darwinPlatformOps) Ditto(ctx context.Context, src, dst string) error {
	cmd := exec.CommandContext(ctx, "ditto", src, dst)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ditto %s -> %s: %w: %s",
			src, dst, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (darwinPlatformOps) ExtractTarGz(
	_ context.Context, _, _ string, _ func(n, total int64),
) error {
	return ErrUnsupported
}

func (darwinPlatformOps) Symlink(target, linkPath string) error {
	return replaceSymlink(target, linkPath)
}

func (darwinPlatformOps) RenameAtomic(src, dst string) error {
	return os.Rename(src, dst)
}

func (darwinPlatformOps) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

func (darwinPlatformOps) FreeSpace(path string) (uint64, error) {
	return statfsFreeBytes(path)
}

// statfsFreeBytes returns the bytes available to a non-root caller
// on the filesystem containing path. Shared by darwin and linux —
// the struct is the same shape on both, only the field types differ
// across platforms.
func statfsFreeBytes(path string) (uint64, error) {
	var s unix.Statfs_t
	if err := unix.Statfs(path, &s); err != nil {
		return 0, fmt.Errorf("statfs %s: %w", path, err)
	}
	return uint64(s.Bavail) * uint64(s.Bsize), nil
}
