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

func (linuxPlatformOps) ExtractTarGz(ctx context.Context, archivePath, destDir string) error {
	return extractTarGz(ctx, archivePath, destDir)
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
