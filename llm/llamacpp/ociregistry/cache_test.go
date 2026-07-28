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

package ociregistry_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"unstable.build/go-tui/llm/llamacpp/ociregistry"
)

func mustOpenCache(t *testing.T) *ociregistry.Cache {
	t.Helper()
	c, err := ociregistry.OpenCache(t.TempDir())
	if err != nil {
		t.Fatalf("OpenCache: %v", err)
	}
	return c
}

func writePartialBlob(t *testing.T, c *ociregistry.Cache, digest string, data []byte) {
	t.Helper()
	path, err := c.BlobPath(digest)
	if err != nil {
		t.Fatalf("BlobPath: %v", err)
	}
	if err := os.WriteFile(path+".partial", data, 0o644); err != nil {
		t.Fatalf("write partial: %v", err)
	}
}

func digestOf(data []byte) string {
	h := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(h[:])
}

func TestCache_BlobPathUsesDashSeparator(t *testing.T) {
	c := mustOpenCache(t)
	digest := digestOf([]byte("hello"))
	path, err := c.BlobPath(digest)
	if err != nil {
		t.Fatalf("BlobPath: %v", err)
	}
	if !strings.Contains(path, "blobs"+string(filepath.Separator)+"sha256-") {
		t.Fatalf("expected dash separator in %s", path)
	}
}

func TestCache_HasBlobAndPartial(t *testing.T) {
	c := mustOpenCache(t)
	digest := digestOf([]byte("abc"))

	ok, sz, err := c.HasBlob(digest)
	if err != nil {
		t.Fatalf("HasBlob: %v", err)
	}
	if ok {
		t.Fatal("expected HasBlob=false on empty cache")
	}
	_ = sz

	writePartialBlob(t, c, digest, []byte("ab"))
	ok, n, err := c.HasPartialBlob(digest)
	if err != nil {
		t.Fatalf("HasPartialBlob: %v", err)
	}
	if !ok || n != 2 {
		t.Fatalf("HasPartialBlob: ok=%v size=%d, want ok=true size=2", ok, n)
	}
}

func TestCache_ImportBlob_VerifiesDigest(t *testing.T) {
	c := mustOpenCache(t)
	want := []byte("content")
	digest := digestOf(want)

	// Write the content to a temp file and import.
	src := filepath.Join(t.TempDir(), "part")
	if err := os.WriteFile(src, want, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := c.ImportBlob(digest, src); err != nil {
		t.Fatalf("ImportBlob: %v", err)
	}

	rc, err := c.OpenBlob(digest)
	if err != nil {
		t.Fatalf("OpenBlob: %v", err)
	}
	defer func() { _ = rc.Close() }()
	got, _ := io.ReadAll(rc)
	if string(got) != string(want) {
		t.Fatalf("content mismatch: got %q want %q", got, want)
	}
}

func TestCache_ImportBlob_RejectsBadDigest(t *testing.T) {
	c := mustOpenCache(t)
	src := filepath.Join(t.TempDir(), "part")
	if err := os.WriteFile(src, []byte("content"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	badDigest := digestOf([]byte("different"))
	if err := c.ImportBlob(badDigest, src); err == nil {
		t.Fatal("expected error on digest mismatch")
	}
}

func TestCache_PutGetManifest(t *testing.T) {
	c := mustOpenCache(t)
	want := []byte(`{"schemaVersion":2}`)
	if err := c.PutManifest("hf.co", "bartowski/repo", "Q4_K_M", want); err != nil {
		t.Fatalf("PutManifest: %v", err)
	}
	got, err := c.GetManifest("hf.co", "bartowski/repo", "Q4_K_M")
	if err != nil {
		t.Fatalf("GetManifest: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestCache_RemovePartial(t *testing.T) {
	c := mustOpenCache(t)
	digest := digestOf([]byte("x"))
	writePartialBlob(t, c, digest, []byte{'x'})

	if err := c.RemovePartialBlob(digest); err != nil {
		t.Fatalf("RemovePartialBlob: %v", err)
	}
	if ok, _, _ := c.HasPartialBlob(digest); ok {
		t.Fatal("partial should have been removed")
	}
	// Removing again must be a no-op.
	if err := c.RemovePartialBlob(digest); err != nil {
		t.Fatalf("RemovePartialBlob (idempotent): %v", err)
	}
}

func TestCache_DeleteManifestAndPruneUnreferencedBlobs(t *testing.T) {
	c := mustOpenCache(t)
	shared := []byte("shared")
	onlyA := []byte("only-a")
	onlyB := []byte("only-b")
	sharedDig := digestOf(shared)
	onlyADig := digestOf(onlyA)
	onlyBDig := digestOf(onlyB)

	// Seed 3 blobs.
	for _, tc := range []struct {
		digest string
		data   []byte
	}{{sharedDig, shared}, {onlyADig, onlyA}, {onlyBDig, onlyB}} {
		src := filepath.Join(t.TempDir(), strings.ReplaceAll(tc.digest, ":", "-"))
		if err := os.WriteFile(src, tc.data, 0o644); err != nil {
			t.Fatalf("write seed blob: %v", err)
		}
		if err := c.ImportBlob(tc.digest, src); err != nil {
			t.Fatalf("ImportBlob(%s): %v", tc.digest, err)
		}
	}

	manifestA := []byte(`{"schemaVersion":2,"layers":[` +
		`{"mediaType":"` + ociregistry.MediaTypeOllamaModel + `","digest":"` + onlyADig + `","size":6},` +
		`{"mediaType":"` + ociregistry.MediaTypeOllamaTemplate + `","digest":"` + sharedDig + `","size":6}` +
		`]}`)
	manifestB := []byte(`{"schemaVersion":2,"layers":[` +
		`{"mediaType":"` + ociregistry.MediaTypeOllamaModel + `","digest":"` + onlyBDig + `","size":6},` +
		`{"mediaType":"` + ociregistry.MediaTypeOllamaTemplate + `","digest":"` + sharedDig + `","size":6}` +
		`]}`)

	if err := c.PutManifest("huggingface.co", "foo/a-GGUF", "latest", manifestA); err != nil {
		t.Fatalf("PutManifest A: %v", err)
	}
	if err := c.PutManifest("huggingface.co", "foo/b-GGUF", "latest", manifestB); err != nil {
		t.Fatalf("PutManifest B: %v", err)
	}

	// Delete A. Manifest A and its private blob should go away, but the
	// shared blob must stay because manifest B still references it.
	if err := c.DeleteReference("huggingface.co", "foo/a-GGUF", "latest"); err != nil {
		t.Fatalf("DeleteReference A: %v", err)
	}
	if _, err := c.GetManifest("huggingface.co", "foo/a-GGUF", "latest"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("manifest A still exists or wrong error: %v", err)
	}
	if ok, _, _ := c.HasBlob(onlyADig); ok {
		t.Fatalf("private blob %s should have been pruned", onlyADig)
	}
	if ok, _, _ := c.HasBlob(sharedDig); !ok {
		t.Fatalf("shared blob %s should still exist", sharedDig)
	}
	if ok, _, _ := c.HasBlob(onlyBDig); !ok {
		t.Fatalf("manifest B blob %s should still exist", onlyBDig)
	}

	// Delete B. Now the shared blob becomes unreferenced and should be pruned.
	if err := c.DeleteReference("huggingface.co", "foo/b-GGUF", "latest"); err != nil {
		t.Fatalf("DeleteReference B: %v", err)
	}
	if ok, _, _ := c.HasBlob(sharedDig); ok {
		t.Fatalf("shared blob %s should have been pruned after deleting last reference", sharedDig)
	}
	if ok, _, _ := c.HasBlob(onlyBDig); ok {
		t.Fatalf("private blob %s should have been pruned", onlyBDig)
	}
}
