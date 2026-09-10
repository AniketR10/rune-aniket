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

package ideupgrade

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// extractTarGz extracts a gzip-compressed tar archive into destDir.
// It is pure Go (archive/tar + compress/gzip) with no platform-specific
// calls, so it is deliberately kept free of build tags: the Linux
// upgrade path uses it in production, and cross-platform tests exercise
// it directly against real tarball bytes.
//
// progress, when non-nil, is called after every archive entry with the
// number of compressed bytes consumed so far and the total archive
// size. Compressed bytes are used rather than extracted bytes because
// the uncompressed size is not known until the archive has been fully
// read.
func extractTarGz(
	ctx context.Context, archivePath, destDir string,
	progress func(n, total int64),
) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", destDir, err)
	}
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer f.Close()
	var total int64
	if info, err := f.Stat(); err == nil {
		total = info.Size()
	}
	counted := &countingReader{r: f}
	gz, err := gzip.NewReader(counted)
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
		if progress != nil {
			progress(counted.n, total)
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

// countingReader tallies the bytes read from the underlying reader so
// extractTarGz can report how much of the compressed archive it has
// consumed.
type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}
