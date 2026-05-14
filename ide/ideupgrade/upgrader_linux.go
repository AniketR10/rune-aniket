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
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", destDir, err)
	}
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}
		// Reject path traversal attempts.
		clean := filepath.Clean(hdr.Name)
		if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
			return fmt.Errorf("invalid tar entry %q", hdr.Name)
		}
		target := filepath.Join(destDir, clean)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode)|0o700); err != nil {
				return fmt.Errorf("mkdir %s: %w", target, err)
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("mkdir parent: %w", err)
			}
			out, err := os.OpenFile(target,
				os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return fmt.Errorf("create %s: %w", target, err)
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return fmt.Errorf("write %s: %w", target, err)
			}
			out.Close()
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("mkdir parent: %w", err)
			}
			_ = os.Remove(target)
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return fmt.Errorf("symlink %s: %w", target, err)
			}
		default:
			// Skip unsupported entry types (devices, fifos, etc.).
		}
	}
	return nil
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
