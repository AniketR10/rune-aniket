// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY.

package agentshell

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"unstable.build/go-tui/cmd/rune-agent/llm/llamacpp"
	"unstable.build/go-tui/cmd/rune-agent/llm/llamacpp/ociregistry"
)

// ---- helpers --------------------------------------------------------------

// digestOf returns the sha256: digest format used by the OCI cache layout.
func digestOf(b []byte) string {
	h := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(h[:])
}

// newTestLocalRegistry wraps client as the transport of a fresh
// llamacpp.Registry rooted at a unique temp dir. Tests pass the
// resulting registry to WithLocalRegistry so the shell exercises the
// same code path as production while the underlying HTTP requests are
// served by the caller's httptest server.
func newTestLocalRegistry(t *testing.T, client *ociregistry.Client) *llamacpp.Registry {
	t.Helper()
	reg, err := llamacpp.NewRegistry(
		t.TempDir(),
		llamacpp.WithDownloader(client),
	)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	return reg
}

// seedLocalModel writes a manifest + weight blob into the registry's
// on-disk cache so Models()/CachedReferences() treat the entry as
// fully pulled. The layout must stay in sync with ociregistry.Cache.
func seedLocalModel(
	t *testing.T, reg *llamacpp.Registry, host, repo, tag string, body []byte,
) string {
	t.Helper()
	digest := digestOf(body)
	root := reg.Root()
	blobDir := filepath.Join(root, "blobs")
	if err := os.MkdirAll(blobDir, 0o755); err != nil {
		t.Fatalf("mkdir blobs: %v", err)
	}
	blobName := "sha256-" + digest[len("sha256:"):]
	if err := os.WriteFile(filepath.Join(blobDir, blobName), body, 0o644); err != nil {
		t.Fatalf("write blob: %v", err)
	}
	mDir := filepath.Join(root, "manifests", host, repo)
	if err := os.MkdirAll(mDir, 0o755); err != nil {
		t.Fatalf("mkdir manifests: %v", err)
	}
	manifest := []byte(`{"schemaVersion":2,"mediaType":"` +
		ociregistry.MediaTypeDockerManifestV2 +
		`","layers":[{"mediaType":"` + ociregistry.MediaTypeOllamaModel +
		`","digest":"` + digest +
		`","size":` + strconv.Itoa(len(body)) + `}]}`)
	if err := os.WriteFile(filepath.Join(mDir, tag), manifest, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return digest
}

// blobPresent reports whether the weight blob for digest is fully
// materialised under root (i.e. completed, not the partial sidecar).
func blobPresent(root, digest string) bool {
	name := "sha256-" + digest[len("sha256:"):]
	_, err := os.Stat(filepath.Join(root, "blobs", name))
	return err == nil
}

// ---- tiny in-process OCI registry fake -----------------------------------

type fakeOCI struct {
	manifest []byte
	blobs    map[string][]byte
}

func (f *fakeOCI) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/manifests/"):
			w.Header().Set("Content-Type", ociregistry.MediaTypeDockerManifestV2)
			_, _ = w.Write(f.manifest)
		case strings.Contains(r.URL.Path, "/blobs/"):
			parts := strings.Split(r.URL.Path, "/")
			digest := parts[len(parts)-1]
			data, ok := f.blobs[digest]
			if !ok {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
			w.Header().Set("Accept-Ranges", "bytes")
			if r.Method == http.MethodHead {
				w.WriteHeader(http.StatusOK)
				return
			}
			_, _ = w.Write(data)
		case r.URL.Path == "/v2/":
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	})
	return mux
}

func buildFake(t *testing.T) (*httptest.Server, string, string) {
	t.Helper()
	modelContent := []byte("GGUF-fake-model-weights")
	modelDigest := digestOf(modelContent)
	tplContent := []byte("chat template placeholder")
	tplDigest := digestOf(tplContent)

	m := ociregistry.Manifest{
		SchemaVersion: 2,
		MediaType:     ociregistry.MediaTypeDockerManifestV2,
		Config: ociregistry.Descriptor{
			MediaType: "application/vnd.docker.container.image.v1+json",
			Digest:    digestOf([]byte("cfg")),
			Size:      3,
		},
		Layers: []ociregistry.Descriptor{
			{MediaType: ociregistry.MediaTypeOllamaModel, Digest: modelDigest, Size: int64(len(modelContent))},
			{MediaType: ociregistry.MediaTypeOllamaTemplate, Digest: tplDigest, Size: int64(len(tplContent))},
		},
	}
	mdata, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	fake := &fakeOCI{
		manifest: mdata,
		blobs: map[string][]byte{
			modelDigest: modelContent,
			tplDigest:   tplContent,
		},
	}
	srv := httptest.NewServer(fake.handler())
	t.Cleanup(srv.Close)
	return srv, modelDigest, "foo/bar:latest"
}

// ---- progress writer capture --------------------------------------------

type recordingPW struct {
	mu    sync.Mutex
	calls int32
}

func (r *recordingPW) Progress(_, _ int64, _ string) {
	atomic.AddInt32(&r.calls, 1)
}

// captureProgress records every Progress call so tests can inspect the
// exact byte-count samples flowing into the REPL progress bar.
type progressCall struct {
	progress, total int64
	units           string
}

type captureProgress struct {
	mu      sync.Mutex
	samples []progressCall
}

func (c *captureProgress) Progress(progress, total int64, units string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.samples = append(c.samples, progressCall{progress, total, units})
}

func (c *captureProgress) snapshot() []progressCall {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]progressCall(nil), c.samples...)
}

// newShellFor builds a shell wired to reg. reg must be non-nil — the
// local llamacpp.Registry is a hard dependency of the shell now.
func newShellFor(t *testing.T, deps *testDeps, reg *llamacpp.Registry) *shell {
	t.Helper()
	if reg == nil {
		t.Fatal("newShellFor: reg must not be nil")
	}
	opts := append([]Option(nil), deps.opts...)
	return New(
		deps.wm, deps.svc, deps.modelRegistry, deps.defaultModel,
		deps.store, deps.registry, deps.agentsConfig, deps.cfg,
		deps.skillRegistry, deps.workspaceRoot, deps.fs,
		deps.storage, nil, nil, nil,
		deps.notifications, deps.dataPath,
		reg,
		opts...,
	).(*shell)
}

// TestScaleProgressBytes pins down the byte-scaling contract used when
// feeding download progress into the REPL ProgressWriter.
func TestScaleProgressBytes(t *testing.T) {
	tests := []struct {
		name              string
		progress, total   int64
		wantProg, wantTot int64
		wantUnit          string
	}{
		{"zero total keeps B", 0, 0, 0, 0, "B"},
		{"sub-KiB stays in B", 500, 900, 500, 900, "B"},
		{"small KiB stays in B for enough ticks", 2048, 10 * 1024, 2048, 10240, "B"},
		{"large KiB uses KiB", 50 * 1024, 200 * 1024, 50, 200, "KiB"},
		{"small MiB drops to KiB", 3 * 1024 * 1024, 12 * 1024 * 1024, 3 * 1024, 12 * 1024, "KiB"},
		{"large MiB uses MiB", 50 * 1024 * 1024, 200 * 1024 * 1024, 50, 200, "MiB"},
		{"21 GiB uses MiB for 1% ticks", 1024 * 1024 * 1024, 21 * 1024 * 1024 * 1024, 1024, 21 * 1024, "MiB"},
		{"small GiB drops to MiB", 3*1024*1024*1024 + 512*1024*1024, 8 * 1024 * 1024 * 1024, 3*1024 + 512, 8 * 1024, "MiB"},
		{"large GiB uses GiB", 50 * 1024 * 1024 * 1024, 200 * 1024 * 1024 * 1024, 50, 200, "GiB"},
		{"small TiB drops to GiB", 2 * 1024 * 1024 * 1024 * 1024, 10 * 1024 * 1024 * 1024 * 1024, 2 * 1024, 10 * 1024, "GiB"},
		{"large TiB uses TiB", 50 * 1024 * 1024 * 1024 * 1024, 200 * 1024 * 1024 * 1024 * 1024, 50, 200, "TiB"},
		{"progress clamps above total", 9999, 1024, 1024, 1024, "B"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotProg, gotTot, gotUnit := scaleProgressBytes(tc.progress, tc.total)
			if gotProg != tc.wantProg || gotTot != tc.wantTot || gotUnit != tc.wantUnit {
				t.Fatalf("scaleProgressBytes(%d,%d) = (%d,%d,%q), want (%d,%d,%q)",
					tc.progress, tc.total, gotProg, gotTot, gotUnit,
					tc.wantProg, tc.wantTot, tc.wantUnit)
			}
		})
	}
}

// TestDownload_ProgressWriterUsesScaledUnits guards the real call site
// in handleDownload against regressing to raw-byte Progress calls.
func TestDownload_ProgressWriterUsesScaledUnits(t *testing.T) {
	body := bytes.Repeat([]byte("Z"), 512*1024)
	modelDigest := digestOf(body)
	m := ociregistry.Manifest{
		SchemaVersion: 2,
		MediaType:     ociregistry.MediaTypeDockerManifestV2,
		Config: ociregistry.Descriptor{
			MediaType: "application/vnd.docker.container.image.v1+json",
			Digest:    digestOf([]byte("cfg")),
			Size:      3,
		},
		Layers: []ociregistry.Descriptor{
			{MediaType: ociregistry.MediaTypeOllamaModel, Digest: modelDigest, Size: int64(len(body))},
		},
	}
	mdata, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	fr := &fakeOCI{manifest: mdata, blobs: map[string][]byte{modelDigest: body}}
	srv := httptest.NewServer(fr.handler())
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}

	client := ociregistry.NewClient()
	client.PlainHTTP = true
	client.HTTPClient = srv.Client()
	sh := newShellFor(t, newTestDeps(), newTestLocalRegistry(t, client))

	pw := &captureProgress{}
	it, err := sh.HandleCommand(context.Background(),
		repl.Command{Name: "local", Args: []string{"download", u.Host + "/foo/bar:latest"}},
		pw,
	)
	if err != nil {
		t.Fatalf("HandleCommand: %v", err)
	}
	for {
		if _, ok := it.Next(context.Background()); !ok {
			break
		}
	}
	_ = it.Close()

	var sawScaled bool
	for _, s := range pw.snapshot() {
		if s.units != "" && s.units != "B" {
			sawScaled = true
			break
		}
	}
	if !sawScaled {
		t.Fatalf("expected at least one scaled progress sample, got %+v", pw.snapshot())
	}
}

// ---- tests ---------------------------------------------------------------

// TestNew_PanicsWithoutLocalRegistry pins the invariant that the
// shell's local llamacpp.Registry is a mandatory positional dependency.
// Handlers that require it (download / list / delete) are always
// available — the feature is never silently disabled.
func TestNew_PanicsWithoutLocalRegistry(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected New to panic when local registry is nil")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "local llamacpp.Registry") {
			t.Fatalf("unexpected panic value: %v", r)
		}
	}()
	deps := newTestDeps()
	_ = New(
		deps.wm, deps.svc, deps.modelRegistry, deps.defaultModel,
		deps.store, deps.registry, deps.agentsConfig, deps.cfg,
		deps.skillRegistry, deps.workspaceRoot, deps.fs,
		deps.storage, nil, nil, nil,
		deps.notifications, deps.dataPath,
		nil, // local registry intentionally nil
	)
}

func TestDownload_MissingReferenceReturnsError(t *testing.T) {
	client := ociregistry.NewClient()
	client.PlainHTTP = true
	sh := newShellFor(t, newTestDeps(), newTestLocalRegistry(t, client))
	_, err := sh.HandleCommand(context.Background(),
		repl.Command{Name: "local", Args: []string{"download"}}, repl.NopProgressWriter())
	if err == nil {
		t.Fatal("expected usage error")
	}
	msg := err.Error()
	for _, want := range []string{"local download <reference>", "Examples", "unsloth/", "hf.co/", "help local download"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("usage message missing %q:\n%s", want, msg)
		}
	}
}

func TestDownload_PullsFromFakeRegistry(t *testing.T) {
	srv, modelDigest, ref := buildFake(t)
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}

	client := ociregistry.NewClient()
	client.PlainHTTP = true
	client.HTTPClient = srv.Client()
	reg := newTestLocalRegistry(t, client)
	sh := newShellFor(t, newTestDeps(), reg)

	pw := &recordingPW{}
	it, err := sh.HandleCommand(context.Background(),
		repl.Command{Name: "local", Args: []string{"download", u.Host + "/" + ref}},
		pw,
	)
	if err != nil {
		t.Fatalf("HandleCommand: %v", err)
	}
	text := renderOutput(t, context.Background(), it)

	if !strings.Contains(text, "Downloaded") {
		t.Errorf("expected 'Downloaded' in output, got:\n%s", text)
	}
	if !strings.Contains(text, "template") {
		t.Errorf("expected template layer in summary, got:\n%s", text)
	}

	if !blobPresent(reg.Root(), modelDigest) {
		t.Fatalf("expected blob %s in cache at %s", modelDigest, reg.Root())
	}

	if n := atomic.LoadInt32(&pw.calls); n == 0 {
		t.Fatalf("expected ProgressWriter to receive progress updates, got 0")
	}
}

// TestDownload_EmitsInitialProgress ensures the REPL progress bar is
// shown immediately when the command starts, before any manifest is
// fetched.
func TestDownload_EmitsInitialProgress(t *testing.T) {
	client := ociregistry.NewClient()
	client.HTTPClient = deadHTTPClient
	sh := newShellFor(t, newTestDeps(), newTestLocalRegistry(t, client))

	pw := &recordingPW{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := sh.HandleCommand(ctx,
		repl.Command{Name: "local", Args: []string{"download", "foo/bar"}},
		pw,
	)
	if err != nil {
		t.Fatalf("HandleCommand: %v", err)
	}
	if n := atomic.LoadInt32(&pw.calls); n < 1 {
		t.Fatalf("expected initial progress tick before network I/O, got %d", n)
	}
}

// TestDownload_HostlessReferenceDefaultsToHF verifies that a bare
// `owner/repo` argument parses against huggingface.co.
func TestDownload_HostlessReferenceDefaultsToHF(t *testing.T) {
	client := ociregistry.NewClient()
	client.HTTPClient = deadHTTPClient
	sh := newShellFor(t, newTestDeps(), newTestLocalRegistry(t, client))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	it, err := sh.HandleCommand(ctx,
		repl.Command{Name: "local", Args: []string{"download", "unsloth/gemma-3n-E2B-it-GGUF"}},
		repl.NopProgressWriter(),
	)
	if err != nil {
		t.Fatalf("HandleCommand: %v", err)
	}
	for {
		_, ok := it.Next(ctx)
		if !ok {
			break
		}
	}
	_ = it.Close()
}

// TestDownload_400WithXErrorMessageSurfacedToUser verifies HF-style 400
// bodies are surfaced unchanged instead of being hidden behind the
// generic "-GGUF variant" hint.
func TestDownload_400WithXErrorMessageSurfacedToUser(t *testing.T) {
	const wantMsg = "The specified repository contains sharded GGUF. " +
		"Ollama does not support this yet."

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v2/":
			w.WriteHeader(http.StatusOK)
		case strings.Contains(r.URL.Path, "/manifests/"):
			w.Header().Set("x-error-message", wantMsg)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"` + wantMsg + `"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	client := ociregistry.NewClient()
	client.PlainHTTP = true
	client.HTTPClient = srv.Client()
	sh := newShellFor(t, newTestDeps(), newTestLocalRegistry(t, client))

	it, err := sh.HandleCommand(context.Background(),
		repl.Command{Name: "local",
			Args: []string{"download", u.Host + "/ggml-org/gpt-oss-120b-GGUF"}},
		repl.NopProgressWriter(),
	)
	if err != nil {
		t.Fatalf("HandleCommand: %v", err)
	}
	for {
		if _, ok := it.Next(context.Background()); !ok {
			break
		}
	}
	iterErr := it.Err()
	_ = it.Close()
	if iterErr == nil {
		t.Fatal("expected an error")
	}
	msg := iterErr.Error()
	if !strings.Contains(msg, "sharded GGUF") {
		t.Fatalf("expected registry message to be surfaced, got: %s", msg)
	}
	if strings.Contains(msg, "look for") && strings.Contains(msg, "-GGUF") {
		t.Fatalf("expected no spurious '-GGUF variant' hint, got: %s", msg)
	}
}

// TestDownload_CancelThenResumeReportsRealProgress reproduces the
// user-visible behaviour: cancel mid-stream, then re-run — the UI must
// still show real progress for the bytes still missing.
func TestDownload_CancelThenResumeReportsRealProgress(t *testing.T) {
	srv, modelDigest, ref := buildSlowFake(t)
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}

	client := ociregistry.NewClient()
	client.PlainHTTP = true
	client.HTTPClient = srv.Client()
	reg := newTestLocalRegistry(t, client)
	sh := newShellFor(t, newTestDeps(), reg)

	// --- Run 1: cancel mid-stream.
	ctx1, cancel1 := context.WithCancel(context.Background())
	t.Cleanup(cancel1)
	it1, err := sh.HandleCommand(ctx1,
		repl.Command{Name: "local", Args: []string{"download", u.Host + "/" + ref}},
		&recordingPW{},
	)
	if err != nil {
		t.Fatalf("run 1 HandleCommand: %v", err)
	}

	run1Done := make(chan struct{})
	go func() {
		defer close(run1Done)
		for {
			_, ok := it1.Next(ctx1)
			if !ok {
				return
			}
		}
	}()
	<-srv.midStreamReached()
	cancel1()
	<-run1Done
	_ = it1.Close()

	if blobPresent(reg.Root(), modelDigest) {
		t.Fatal("weight blob already fully cached after ctrl-c mid-stream: " +
			"resume would bogusly appear to complete immediately")
	}

	// --- Run 2: let it complete.
	srv.release()
	pw2 := &captureProgress{}
	it2, err := sh.HandleCommand(context.Background(),
		repl.Command{Name: "local", Args: []string{"download", u.Host + "/" + ref}},
		pw2,
	)
	if err != nil {
		t.Fatalf("run 2 HandleCommand: %v", err)
	}
	for {
		_, ok := it2.Next(context.Background())
		if !ok {
			break
		}
	}
	_ = it2.Close()

	if !blobPresent(reg.Root(), modelDigest) {
		t.Fatalf("expected blob %s in cache after successful retry", modelDigest)
	}

	samples := pw2.snapshot()
	if len(samples) == 0 {
		t.Fatal("resume emitted no progress samples at all")
	}
	sawPartial := false
	var finalSample progressCall
	for _, s := range samples {
		if s.total > 0 && s.progress < s.total {
			sawPartial = true
		}
		if s.total > 0 {
			finalSample = s
		}
	}
	if !sawPartial {
		t.Fatalf("resume never reported partial progress — every sample "+
			"was already at 100%%, which is the 'completed immediately' "+
			"bug. Samples: %+v", samples)
	}
	if finalSample.progress != finalSample.total {
		t.Fatalf("resume did not reach 100%%: final=%d/%d; all samples: %+v",
			finalSample.progress, finalSample.total, samples)
	}
}

func TestLocalDownload_RoutesToExistingRegistry(t *testing.T) {
	srv, modelDigest, ref := buildFake(t)
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}

	client := ociregistry.NewClient()
	client.PlainHTTP = true
	client.HTTPClient = srv.Client()
	reg := newTestLocalRegistry(t, client)
	sh := newShellFor(t, newTestDeps(), reg)

	it, err := sh.HandleCommand(context.Background(),
		repl.Command{Name: "local", Args: []string{"download", u.Host + "/" + ref}},
		repl.NopProgressWriter(),
	)
	if err != nil {
		t.Fatalf("HandleCommand: %v", err)
	}
	_ = renderOutput(t, context.Background(), it)
	if !blobPresent(reg.Root(), modelDigest) {
		t.Fatalf("expected blob %s to be downloaded via local download", modelDigest)
	}
}

func TestLocalList_ListsDownloadedLocalModels(t *testing.T) {
	reg, err := llamacpp.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	seedLocalModel(t, reg, "huggingface.co", "foo/bar-GGUF", "latest", []byte("gguf-bytes"))

	sh := newShellFor(t, newTestDeps(), reg)
	it, err := sh.HandleCommand(context.Background(),
		repl.Command{Name: "local", Args: []string{"list"}},
		repl.NopProgressWriter(),
	)
	if err != nil {
		t.Fatalf("HandleCommand: %v", err)
	}
	text := renderOutput(t, context.Background(), it)
	if !strings.Contains(text, "huggingface.co/foo/bar-GGUF:latest") {
		t.Fatalf("expected local model in output, got:\n%s", text)
	}
}

func TestLocalDelete_RemovesCachedModel(t *testing.T) {
	reg, err := llamacpp.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	digest := seedLocalModel(t, reg, "huggingface.co", "foo/delete-GGUF", "latest",
		[]byte("gguf-delete"))

	sh := newShellFor(t, newTestDeps(), reg)
	it, err := sh.HandleCommand(context.Background(),
		repl.Command{Name: "local", Args: []string{"delete", "foo/delete-GGUF"}},
		repl.NopProgressWriter(),
	)
	if err != nil {
		t.Fatalf("HandleCommand: %v", err)
	}
	_ = renderOutput(t, context.Background(), it)
	if blobPresent(reg.Root(), digest) {
		t.Fatalf("expected blob %s to be removed by local delete", digest)
	}
	if refs, _ := reg.CachedReferences(); len(refs) != 0 {
		t.Fatalf("expected no cached references after delete, got %+v", refs)
	}
}

// ---- slowFake / deadHTTPClient used by tests above ----------------------

type slowFake struct {
	*httptest.Server
	midStreamCh chan struct{}
	midOnce     sync.Once
	releaseCh   chan struct{}
}

func (s *slowFake) midStreamReached() <-chan struct{} { return s.midStreamCh }
func (s *slowFake) release()                          { close(s.releaseCh) }

func buildSlowFake(t *testing.T) (*slowFake, string, string) {
	t.Helper()
	body := make([]byte, 64*1024)
	for i := range body {
		body[i] = byte('A' + i%26)
	}
	modelDigest := digestOf(body)

	m := ociregistry.Manifest{
		SchemaVersion: 2,
		MediaType:     ociregistry.MediaTypeDockerManifestV2,
		Config: ociregistry.Descriptor{
			MediaType: "application/vnd.docker.container.image.v1+json",
			Digest:    digestOf([]byte("cfg")),
			Size:      3,
		},
		Layers: []ociregistry.Descriptor{
			{MediaType: ociregistry.MediaTypeOllamaModel, Digest: modelDigest, Size: int64(len(body))},
		},
	}
	mdata, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	sf := &slowFake{
		midStreamCh: make(chan struct{}),
		releaseCh:   make(chan struct{}),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/manifests/"):
			w.Header().Set("Content-Type", ociregistry.MediaTypeDockerManifestV2)
			_, _ = w.Write(mdata)
		case strings.Contains(r.URL.Path, "/blobs/"):
			if r.Method == http.MethodHead {
				w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
				w.Header().Set("Accept-Ranges", "bytes")
				w.WriteHeader(http.StatusOK)
				return
			}
			start := int64(0)
			if rh := r.Header.Get("Range"); strings.HasPrefix(rh, "bytes=") {
				spec := strings.TrimPrefix(rh, "bytes=")
				parts := strings.SplitN(spec, "-", 2)
				if n, err := strconv.ParseInt(parts[0], 10, 64); err == nil {
					start = n
				}
				w.Header().Set("Content-Range",
					fmt.Sprintf("bytes %d-%d/%d", start, len(body)-1, len(body)))
				w.Header().Set("Content-Length",
					fmt.Sprintf("%d", int64(len(body))-start))
				w.WriteHeader(http.StatusPartialContent)
			} else {
				w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
				w.Header().Set("Accept-Ranges", "bytes")
			}
			flusher, _ := w.(http.Flusher)

			remaining := body[start:]
			half := len(remaining) / 2
			_, _ = w.Write(remaining[:half])
			if flusher != nil {
				flusher.Flush()
			}
			sf.midOnce.Do(func() { close(sf.midStreamCh) })
			select {
			case <-sf.releaseCh:
				_, _ = w.Write(remaining[half:])
			case <-r.Context().Done():
			}
		case r.URL.Path == "/v2/":
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	})
	sf.Server = httptest.NewServer(mux)
	t.Cleanup(sf.Server.Close)
	return sf, modelDigest, "foo/bar:latest"
}

// deadHTTPClient refuses every connection.
var deadHTTPClient = &http.Client{
	Transport: deadRoundTripper{},
	Timeout:   1,
}

type deadRoundTripper struct{}

func (deadRoundTripper) RoundTrip(_ *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("test: network disabled")
}
