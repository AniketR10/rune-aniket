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

package ociregistry

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

// Cache is a content-addressed on-disk store that mirrors the Ollama layout:
//
//	<root>/blobs/sha256-<hex>
//	<root>/manifests/<host>/<repository...>/<tag or digest>
//
// The layout is deliberately compatible so that models pulled here can be
// consumed by an Ollama daemon pointed at the same models directory, and
// vice versa.
type Cache struct {
	root string

	mu       sync.Mutex
	blobLock map[string]*sync.Mutex
}

// OpenCache creates root if it does not exist and returns a Cache rooted there.
func OpenCache(root string) (*Cache, error) {
	if root == "" {
		return nil, errors.New("ociregistry: cache root path is empty")
	}
	if err := os.MkdirAll(filepath.Join(root, "blobs"), 0o755); err != nil {
		return nil, fmt.Errorf("ociregistry: create blobs dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "manifests"), 0o755); err != nil {
		return nil, fmt.Errorf("ociregistry: create manifests dir: %w", err)
	}
	return &Cache{root: root, blobLock: make(map[string]*sync.Mutex)}, nil
}

// Root returns the cache root directory.
func (c *Cache) Root() string { return c.root }

// BlobPath returns the on-disk path for the given digest. The file need not
// exist; callers typically pair BlobPath with HasBlob to check availability.
func (c *Cache) BlobPath(digest string) (string, error) {
	if err := validateDigest(digest); err != nil {
		return "", err
	}
	// Docker convention: the colon in the digest becomes a dash on disk so
	// the filename is FAT32/Windows-safe. Ollama follows the same rule.
	alg, hex, _ := strings.Cut(digest, ":")
	return filepath.Join(c.root, "blobs", alg+"-"+hex), nil
}

// HasBlob reports whether a blob with the given digest is present and its
// size (or 0 when absent).
func (c *Cache) HasBlob(digest string) (bool, int64, error) {
	path, err := c.BlobPath(digest)
	if err != nil {
		return false, 0, err
	}
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, 0, nil
	}
	if err != nil {
		return false, 0, err
	}
	return true, info.Size(), nil
}

// HasPartialBlob reports whether a partially-downloaded blob is present and
// returns its current size. Used to drive HTTP Range resumes.
func (c *Cache) HasPartialBlob(digest string) (bool, int64, error) {
	path, err := c.BlobPath(digest)
	if err != nil {
		return false, 0, err
	}
	info, err := os.Stat(path + ".partial")
	if errors.Is(err, os.ErrNotExist) {
		return false, 0, nil
	}
	if err != nil {
		return false, 0, err
	}
	return true, info.Size(), nil
}

// RemovePartialBlob deletes any .partial file for the digest. Used when the
// server rejects a resume (e.g. object moved) and we have to start over.
func (c *Cache) RemovePartialBlob(digest string) error {
	path, err := c.BlobPath(digest)
	if err != nil {
		return err
	}
	err = os.Remove(path + ".partial")
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// OpenBlob returns an io.ReadCloser over a fully-downloaded blob. Returns
// an error when the blob is absent.
func (c *Cache) OpenBlob(digest string) (io.ReadCloser, error) {
	path, err := c.BlobPath(digest)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}

// LockBlob acquires a per-digest lock so concurrent downloads of the same
// blob are serialised within a process. The returned function releases the
// lock.
func (c *Cache) LockBlob(digest string) func() {
	c.mu.Lock()
	m, ok := c.blobLock[digest]
	if !ok {
		m = &sync.Mutex{}
		c.blobLock[digest] = m
	}
	c.mu.Unlock()
	m.Lock()
	return m.Unlock
}

// ImportBlob verifies the sha256 of src and, on success, atomically renames
// it to its final blob path. Callers use this to promote a .partial file
// after a successful download.
func (c *Cache) ImportBlob(digest, src string) error {
	dst, err := c.BlobPath(digest)
	if err != nil {
		return err
	}
	if err := verifyFileDigest(src, digest); err != nil {
		return err
	}
	return os.Rename(src, dst)
}

// verifyFileDigest computes sha256 (or sha512) of src and returns an error
// if it does not match the expected digest.
func verifyFileDigest(src, digest string) error {
	alg, want, _ := strings.Cut(digest, ":")
	if alg != "sha256" {
		// Only sha256 is used in practice — keep a clean failure mode for
		// sha512 or anything else until we actually see it.
		return fmt.Errorf("ociregistry: digest verification for %q not implemented", alg)
	}
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if got != want {
		return fmt.Errorf("ociregistry: digest mismatch on %s: got sha256:%s, want %s", src, got, digest)
	}
	return nil
}

// ManifestPath returns the path at which we store a manifest pointer for
// (host, repository, target). The target is either a tag name or a digest.
func (c *Cache) ManifestPath(host, repository, target string) string {
	// Sanitise `:` in digests so it is safe on Windows.
	target = strings.ReplaceAll(target, ":", "-")
	return filepath.Join(append(
		[]string{c.root, "manifests", host},
		append(strings.Split(repository, "/"), target)...,
	)...)
}

// PutManifest writes the raw manifest bytes under manifests/<host>/<repo>/<target>.
// The file content is the manifest JSON itself — the same layout that Ollama
// uses so its daemon can serve models pulled here.
func (c *Cache) PutManifest(host, repository, target string, data []byte) error {
	path := c.ManifestPath(host, repository, target)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// GetManifest reads a previously-stored manifest. Returns os.ErrNotExist
// when the manifest is absent.
func (c *Cache) GetManifest(host, repository, target string) ([]byte, error) {
	return os.ReadFile(c.ManifestPath(host, repository, target))
}

// DeleteReference removes the stored manifest for (host, repository, target)
// and prunes any blobs that become unreferenced as a result. Shared blobs are
// kept when another manifest in the cache still points at them.
func (c *Cache) DeleteReference(host, repository, target string) error {
	data, err := c.GetManifest(host, repository, target)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return os.ErrNotExist
		}
		return err
	}
	m, err := UnmarshalManifest(data)
	if err != nil {
		return fmt.Errorf("ociregistry: parse manifest for delete: %w", err)
	}
	path := c.ManifestPath(host, repository, target)
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, layer := range m.Layers {
		_ = c.pruneBlobIfUnreferenced(layer.Digest)
	}
	return nil
}

func (c *Cache) pruneBlobIfUnreferenced(digest string) error {
	referenced, err := c.isBlobReferenced(digest)
	if err != nil {
		return err
	}
	if referenced {
		return nil
	}
	path, err := c.BlobPath(digest)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (c *Cache) isBlobReferenced(digest string) (bool, error) {
	root := filepath.Join(c.root, "manifests")
	info, err := os.Stat(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if !info.IsDir() {
		return false, nil
	}
	var found bool
	walkErr := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() || found {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		m, err := UnmarshalManifest(data)
		if err != nil {
			return nil
		}
		for _, layer := range m.Layers {
			if layer.Digest == digest {
				found = true
				break
			}
		}
		return nil
	})
	if walkErr != nil {
		return false, walkErr
	}
	return found, nil
}

// CachedReferences returns every cached model reference currently present in
// the manifests tree, using the same canonical name format as CacheRegistry.
// Only manifests with a model layer and an existing weight blob are returned.
func (c *Cache) CachedReferences() ([]Reference, error) {
	root := filepath.Join(c.root, "manifests")
	info, err := os.Stat(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, nil
	}
	var out []Reference
	walkErr := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return nil
		}
		host, repo, target, ok := manifestCoords(root, path)
		if !ok {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		m, err := UnmarshalManifest(data)
		if err != nil {
			return nil
		}
		model := m.FindLayer(MediaTypeOllamaModel)
		if model == nil {
			return nil
		}
		blobPath, err := c.BlobPath(model.Digest)
		if err != nil {
			return nil
		}
		if st, err := os.Stat(blobPath); err != nil || st.IsDir() {
			return nil
		}
		ref := Reference{Host: host, Repository: repo}
		if strings.HasPrefix(target, "sha256:") || strings.HasPrefix(target, "sha512:") {
			ref.Digest = target
		} else {
			ref.Tag = target
		}
		out = append(out, ref)
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	slices.SortFunc(out, func(a, b Reference) int {
		return strings.Compare(a.String(), b.String())
	})
	return out, nil
}
