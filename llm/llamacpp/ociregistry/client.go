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


package ociregistry

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"oras.land/oras-go/v2/errdef"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/errcode"
	"oras.land/oras-go/v2/registry/remote/retry"
)

// ErrNotImplemented is returned when the registry does not expose an
// endpoint we asked about — most commonly tag listing on Hugging Face.
var ErrNotImplemented = errors.New("ociregistry: endpoint not implemented by registry")

// ErrNotGGUFRepo is returned when the caller points at a Hugging Face
// repository that does not expose the llama.cpp OCI endpoint (which HF only
// opens for GGUF repos). The wrapped error carries the original 400 response
// body for callers that want to surface it verbatim.
var ErrNotGGUFRepo = errors.New(
	"ociregistry: repository is not compatible with the llama.cpp OCI endpoint " +
		"(Hugging Face only exposes this endpoint for GGUF repos — look for a *-GGUF variant)")

// ErrNotFound is returned when the registry responds with a 404 for the
// requested reference. Kept separate from errdef.ErrNotFound so callers
// don't have to depend on oras-go's error types.
var ErrNotFound = errors.New("ociregistry: not found")

// ErrShardedGGUF is returned when the caller points at a Hugging Face
// repository whose GGUF weights are split into multiple shard files. HF's
// llama.cpp OCI endpoint refuses to serve a manifest for these repos and
// answers the manifest request with a 400. The message tells the user how
// to proceed since Rune cannot pull the model through this path.
var ErrShardedGGUF = errors.New(
	"ociregistry: repository contains a sharded (multi-file) GGUF, which " +
		"cannot be downloaded through this endpoint — pick a smaller " +
		"single-file quantization of the model, or download the shards " +
		"manually and merge them with `llama-gguf-split --merge` before " +
		"loading the resulting file")

// DefaultUserAgent is used when Client.UserAgent is empty.
const DefaultUserAgent = "unstable-rune-ociregistry/1.0"

// Client is an OCI v2 client backed by oras-go. The zero value is not
// usable; construct one with NewClient.
type Client struct {
	// HTTPClient overrides the transport. When nil, Client uses
	// retry.DefaultClient (oras-go's retry-wrapped transport).
	HTTPClient *http.Client
	// UserAgent is sent with every request.
	UserAgent string
	// Auth configures credentials. When nil, an anonymous client is used
	// (which still completes Bearer challenges against public registries).
	Auth *auth.Client
	// PlainHTTP forces http:// instead of https://. Intended for tests and
	// localhost registries.
	PlainHTTP bool
	// ManifestMediaTypes extends the Accept header for manifest requests.
	// Empty means "use the oras-go defaults" which cover Docker v2 and OCI.
	ManifestMediaTypes []string
}

// NewClient returns a Client with sensible defaults: TLS enabled, oras-go's
// retry-wrapped transport, and anonymous authentication.
func NewClient() *Client {
	return &Client{
		UserAgent: DefaultUserAgent,
	}
}

// WithHFToken installs a static Bearer token scoped to the given host, the
// way Hugging Face expects authenticated requests to be made. The token is
// sent preemptively on every request (HF's public endpoints do not issue a
// 401 challenge, so an on-demand credential would never fire) and is also
// registered as a host-scoped credential so that oras-go's Bearer-challenge
// flow still works if a redirect lands on a token-protected endpoint.
//
// host defaults to "hf.co" when empty; pass "huggingface.co" if you use
// that form in references.
func (c *Client) WithHFToken(host, token string) *Client {
	if host == "" {
		host = "hf.co"
	}
	header := c.defaultHeader()
	if token != "" {
		header.Set("Authorization", "Bearer "+token)
	}
	c.Auth = &auth.Client{
		Client:     c.httpClient(),
		Header:     header,
		Cache:      auth.NewCache(),
		Credential: auth.StaticCredential(host, auth.Credential{AccessToken: token}),
	}
	return c
}

// httpClient returns the *http.Client used for transport, installing
// oras-go's retry-wrapped transport when the caller did not supply one.
func (c *Client) httpClient() *http.Client {
	base := c.HTTPClient
	if base == nil {
		base = retry.DefaultClient
	}
	// Wrap the transport so non-standard 4xx error bodies (Hugging
	// Face uses `{"error":"..."}` instead of OCI's `{"errors":[...]}`,
	// and ships the real reason in `x-error-message`) are normalised
	// into the OCI shape oras-go knows how to parse. Without this the
	// real failure reason is silently dropped.
	return &http.Client{
		Transport: &errorNormalizingTransport{
			base: transportOf(base),
		},
		CheckRedirect: base.CheckRedirect,
		Jar:           base.Jar,
		Timeout:       base.Timeout,
	}
}

func transportOf(c *http.Client) http.RoundTripper {
	if c.Transport != nil {
		return c.Transport
	}
	return http.DefaultTransport
}

func (c *Client) defaultHeader() http.Header {
	h := http.Header{}
	ua := c.UserAgent
	if ua == "" {
		ua = DefaultUserAgent
	}
	h.Set("User-Agent", ua)
	return h
}

// authClient returns an *auth.Client suitable for driving a repository.
// When the caller did not configure Auth, we still wrap the http client
// so oras-go's Bearer-challenge flow is available for public registries
// that happen to require a token exchange.
func (c *Client) authClient() *auth.Client {
	if c.Auth != nil {
		return c.Auth
	}
	return &auth.Client{
		Client: c.httpClient(),
		Header: c.defaultHeader(),
		Cache:  auth.NewCache(),
	}
}

// repository returns an *remote.Repository for ref. It bypasses
// registry.ParseReference so that Hugging Face's uppercase repo names
// (which violate oras-go's strict validator) are accepted.
func (c *Client) repository(ref Reference) *remote.Repository {
	repo := &remote.Repository{
		Client:             c.authClient(),
		Reference:          ref.orasReference(),
		PlainHTTP:          c.PlainHTTP,
		ManifestMediaTypes: c.ManifestMediaTypes,
	}
	return repo
}

// Ping validates that host speaks the OCI v2 protocol. A 200 means we can
// talk to the registry; a 401 (which Ping treats as success, see below)
// confirms the registry exists and expects auth; anything else is an error.
func (c *Client) Ping(ctx context.Context, host string) error {
	reg := &remote.Registry{
		RepositoryOptions: remote.RepositoryOptions{
			Client:    c.authClient(),
			Reference: registry.Reference{Registry: host},
			PlainHTTP: c.PlainHTTP,
		},
	}
	return reg.Ping(ctx)
}

// FetchManifest retrieves the manifest for ref and returns both the parsed
// form and the original JSON bytes (on Manifest.Raw).
func (c *Client) FetchManifest(ctx context.Context, ref Reference) (*Manifest, error) {
	if ref.Host == "" {
		return nil, errors.New("ociregistry: reference has no host")
	}
	repo := c.repository(ref)

	_, rc, err := repo.FetchReference(ctx, ref.Target())
	if err != nil {
		return nil, translateErr(err)
	}
	defer func() { _ = rc.Close() }()

	body, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("ociregistry: read manifest body: %w", err)
	}
	m, err := UnmarshalManifest(body)
	if err != nil {
		return nil, fmt.Errorf("ociregistry: parse manifest: %w", err)
	}
	return m, nil
}

// ListTags returns the tags defined for host/repository. Registries that
// do not implement /v2/{repo}/tags/list (notably Hugging Face for GGUF
// repos) cause this function to return ErrNotImplemented.
func (c *Client) ListTags(ctx context.Context, host, repository string) (*TagList, error) {
	ref := Reference{Host: host, Repository: repository}
	repo := c.repository(ref)

	var all []string
	err := repo.Tags(ctx, "", func(page []string) error {
		all = append(all, page...)
		return nil
	})
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotImplemented
		}
		return nil, translateErr(err)
	}
	return &TagList{Name: repository, Tags: all}, nil
}

// isNotFound reports whether err is a 404, accounting for oras-go's
// different ways of signalling it across endpoints (errdef.ErrNotFound vs.
// *errcode.ErrorResponse with StatusCode=404).
func isNotFound(err error) bool {
	if errors.Is(err, errdef.ErrNotFound) {
		return true
	}
	var er *errcode.ErrorResponse
	if errors.As(err, &er) && er.StatusCode == http.StatusNotFound {
		return true
	}
	return false
}

// HeadBlob returns the size of the given blob without downloading it.
func (c *Client) HeadBlob(ctx context.Context, ref Reference, digest string) (int64, error) {
	if err := validateDigest(digest); err != nil {
		return 0, err
	}
	repo := c.repository(ref)
	desc, err := repo.Blobs().Resolve(ctx, digest)
	if err != nil {
		return 0, translateErr(err)
	}
	return desc.Size, nil
}

// BlobReader returns a streaming reader for a blob, optionally starting at
// byte offset start. When start > 0, the returned reader is backed by an
// HTTP Range request; callers that cannot resume should always pass 0.
//
// The total content length is returned when known.
func (c *Client) BlobReader(
	ctx context.Context,
	ref Reference,
	digest string,
	start int64,
) (io.ReadCloser, int64, error) {
	if err := validateDigest(digest); err != nil {
		return nil, 0, err
	}
	repo := c.repository(ref)

	// Resolve the descriptor first so we have the total size regardless of
	// whether the server returns a Content-Length on the body itself (some
	// CDNs, including HF's, strip it on 307 redirects to signed URLs).
	desc, err := repo.Blobs().Resolve(ctx, digest)
	if err != nil {
		return nil, 0, translateErr(err)
	}

	rc, err := repo.Blobs().Fetch(ctx, desc)
	if err != nil {
		return nil, 0, translateErr(err)
	}
	if start == 0 {
		return rc, desc.Size, nil
	}

	// oras-go wraps the response body in an io.ReadSeekCloser whenever the
	// server advertised Accept-Ranges: bytes. Seeking re-issues the request
	// with a Range header.
	seeker, ok := rc.(io.Seeker)
	if !ok {
		_ = rc.Close()
		return nil, 0, errors.New("ociregistry: server does not support Range requests for this blob")
	}
	if _, err := seeker.Seek(start, io.SeekStart); err != nil {
		_ = rc.Close()
		return nil, 0, fmt.Errorf("ociregistry: seek blob to %d: %w", start, err)
	}
	return rc, desc.Size, nil
}

// ProgressFunc is invoked during blob downloads. Implementations must not
// return an error; use context cancellation to abort.
type ProgressFunc func(digest string, downloaded, total int64)

// DownloadBlob downloads the blob identified by desc into cache, verifying
// its sha256 as bytes stream. Interrupted downloads are resumed on the
// next call via a .partial file in the cache.
//
// If the blob is already fully present and sized correctly, DownloadBlob
// returns immediately after emitting a terminal progress event.
func (c *Client) DownloadBlob(
	ctx context.Context,
	cache *Cache,
	ref Reference,
	desc Descriptor,
	progress ProgressFunc,
) error {
	if cache == nil {
		return errors.New("ociregistry: nil cache")
	}
	if desc.Digest == "" {
		return errors.New("ociregistry: descriptor has no digest")
	}

	unlock := cache.LockBlob(desc.Digest)
	defer unlock()

	// Fast path: blob already on disk.
	if ok, size, err := cache.HasBlob(desc.Digest); err != nil {
		return err
	} else if ok && (desc.Size == 0 || size == desc.Size) {
		if progress != nil {
			progress(desc.Digest, size, size)
		}
		return nil
	}

	blobPath, err := cache.BlobPath(desc.Digest)
	if err != nil {
		return err
	}
	partial := blobPath + ".partial"

	start := int64(0)
	if ok, n, err := cache.HasPartialBlob(desc.Digest); err != nil {
		return err
	} else if ok {
		start = n
	}

	if err := c.streamToPartial(ctx, ref, desc, partial, start, progress); err != nil {
		return err
	}

	if err := cache.ImportBlob(desc.Digest, partial); err != nil {
		return err
	}
	if progress != nil {
		size := desc.Size
		if size == 0 {
			if info, statErr := os.Stat(blobPath); statErr == nil {
				size = info.Size()
			}
		}
		progress(desc.Digest, size, size)
	}
	return nil
}

// streamToPartial performs the actual HTTP streaming loop, verifying the
// sha256 as bytes are written. On range-not-supported, it falls back to
// a full download (truncating any existing .partial).
func (c *Client) streamToPartial(
	ctx context.Context,
	ref Reference,
	desc Descriptor,
	partial string,
	start int64,
	progress ProgressFunc,
) error {
	rc, total, err := c.BlobReader(ctx, ref, desc.Digest, start)
	if err != nil {
		// Range not supported on this blob — restart from offset 0.
		if start > 0 && strings.Contains(err.Error(), "does not support Range") {
			_ = os.Remove(partial)
			return c.streamToPartial(ctx, ref, desc, partial, 0, progress)
		}
		return err
	}
	defer func() { _ = rc.Close() }()

	if total == 0 {
		total = desc.Size
	}

	flags := os.O_WRONLY | os.O_CREATE
	if start == 0 {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_APPEND
	}
	f, err := os.OpenFile(partial, flags, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if start > 0 {
		if err := replayHash(partial, h, start); err != nil {
			return fmt.Errorf("ociregistry: re-seed hash from partial: %w", err)
		}
	}

	written := start
	buf := make([]byte, 256*1024)
	// Emit an initial "start" progress sample immediately so callers
	// (progress bars) have a real total to latch onto before any bytes
	// flow. Without this, small resumes or fast connections can end
	// up with a single terminal 100% sample, making the bar look like
	// it completed instantly.
	if progress != nil {
		progress(desc.Digest, written, total)
	}
	lastReport := time.Now()
	for {
		n, rerr := rc.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return werr
			}
			h.Write(buf[:n])
			written += int64(n)

			if progress != nil && time.Since(lastReport) > 100*time.Millisecond {
				progress(desc.Digest, written, total)
				lastReport = time.Now()
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if progress != nil {
		progress(desc.Digest, written, total)
	}

	want := strings.TrimPrefix(desc.Digest, "sha256:")
	got := hex.EncodeToString(h.Sum(nil))
	if got != want {
		return fmt.Errorf("ociregistry: digest mismatch: want sha256:%s got sha256:%s", want, got)
	}
	return nil
}

// replayHash streams the first n bytes of path into h. Used to reseed the
// running sha256 on resume.
func replayHash(path string, h io.Writer, n int64) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = io.CopyN(h, f, n)
	return err
}

// PullResult is returned by Client.Pull.
type PullResult struct {
	Reference Reference
	Manifest  *Manifest
	// ModelPath is the absolute path to the GGUF weight blob on disk, or
	// empty when the manifest has no MediaTypeOllamaModel layer.
	ModelPath string
	// MMProjPath is the absolute path to the multimodal projector GGUF on disk,
	// or empty when the manifest has no MediaTypeOllamaProjector layer.
	MMProjPath string
	// Cache is the cache the model was pulled into.
	Cache *Cache
}

// LayerPath returns the on-disk path of the given layer, or "".
func (r *PullResult) LayerPath(mediaType string) string {
	if r == nil || r.Manifest == nil || r.Cache == nil {
		return ""
	}
	layer := r.Manifest.FindLayer(mediaType)
	if layer == nil {
		return ""
	}
	path, err := r.Cache.BlobPath(layer.Digest)
	if err != nil {
		return ""
	}
	return path
}

// Pull downloads the manifest and every referenced blob for ref into cache.
// progress is invoked repeatedly as each layer streams.
func (c *Client) Pull(
	ctx context.Context,
	cache *Cache,
	ref Reference,
	progress ProgressFunc,
) (*PullResult, error) {
	if cache == nil {
		return nil, errors.New("ociregistry: Pull requires a cache")
	}
	if ref.Host == "" {
		return nil, errors.New("ociregistry: Pull requires a host on the reference")
	}

	manifest, err := c.FetchManifest(ctx, ref)
	if err != nil {
		return nil, err
	}

	// Layers include the weight blob plus optional metadata (template,
	// params, system). They're pulled sequentially so progress reports
	// stay monotonic — the weight blob dominates total transfer time, so
	// parallelism gives no meaningful speedup.
	//
	// Report progress as bytes accumulated across the entire manifest so
	// the UI bar reflects overall pull progress. Per-layer callbacks are
	// translated into (completedBefore + layerProgress, manifestTotal)
	// samples.
	var manifestTotal int64
	for _, layer := range manifest.Layers {
		manifestTotal += layer.Size
	}
	var completed int64
	for _, layer := range manifest.Layers {
		layerSize := layer.Size
		var layerProgress ProgressFunc
		if progress != nil {
			layerProgress = func(digest string, downloaded, _ int64) {
				// Clamp against the layer size so a misbehaving
				// server reporting more bytes than declared can't
				// push the bar past 100%.
				if downloaded > layerSize && layerSize > 0 {
					downloaded = layerSize
				}
				progress(digest, completed+downloaded, manifestTotal)
			}
		}
		if err := c.DownloadBlob(ctx, cache, ref, layer, layerProgress); err != nil {
			return nil, fmt.Errorf("ociregistry: pull layer %s: %w", layer.Digest, err)
		}
		completed += layerSize
	}
	if progress != nil && manifestTotal > 0 {
		// Emit a final cumulative 100% tick so the bar lands at a
		// clean terminal state even if the last layer was tiny.
		progress("", manifestTotal, manifestTotal)
	}

	// Persist the manifest last, so an interrupted pull leaves no stale
	// manifest pointing at a missing blob.
	if err := cache.PutManifest(ref.Host, ref.Repository, ref.Target(), manifest.Raw); err != nil {
		return nil, fmt.Errorf("ociregistry: persist manifest: %w", err)
	}

	res := &PullResult{
		Reference: ref,
		Manifest:  manifest,
		Cache:     cache,
	}
	if model := manifest.FindLayer(MediaTypeOllamaModel); model != nil {
		if path, err := cache.BlobPath(model.Digest); err == nil {
			res.ModelPath = path
		}
	}
	if mmproj := manifest.FindLayer(MediaTypeOllamaProjector); mmproj != nil {
		if path, err := cache.BlobPath(mmproj.Digest); err == nil {
			res.MMProjPath = path
		}
	}
	return res, nil
}

// Resolve reads a previously-pulled manifest from the cache.
func Resolve(cache *Cache, ref Reference) (*PullResult, error) {
	data, err := cache.GetManifest(ref.Host, ref.Repository, ref.Target())
	if err != nil {
		return nil, err
	}
	m, err := UnmarshalManifest(data)
	if err != nil {
		return nil, err
	}
	r := &PullResult{
		Reference: ref,
		Manifest:  m,
		Cache:     cache,
	}
	if model := m.FindLayer(MediaTypeOllamaModel); model != nil {
		if path, err := cache.BlobPath(model.Digest); err == nil {
			r.ModelPath = path
		}
	}
	if mmproj := m.FindLayer(MediaTypeOllamaProjector); mmproj != nil {
		if path, err := cache.BlobPath(mmproj.Digest); err == nil {
			r.MMProjPath = path
		}
	}
	return r, nil
}

// translateErr rewrites oras-go/errdef errors into ones that carry our
// package name, so call-site error messages stay uniform. Specifically:
//
//   - HTTP 404 → wrapped as ErrNotFound.
//   - HTTP 400 whose error message indicates the repo is not a GGUF
//     endpoint → wrapped as ErrNotGGUFRepo.
//   - HTTP 400 for a sharded (multi-file) GGUF repo → wrapped as
//     ErrShardedGGUF with actionable guidance.
//   - Other 400s → returned with the registry-provided message
//     preserved so the REPL can show the real reason.
//   - Anything else → returned as-is.
func translateErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, errdef.ErrNotFound) {
		return fmt.Errorf("%w: %s", ErrNotFound, err.Error())
	}
	var er *errcode.ErrorResponse
	if errors.As(err, &er) {
		switch er.StatusCode {
		case http.StatusNotFound:
			return fmt.Errorf("%w: %s", ErrNotFound, er.Error())
		case http.StatusBadRequest:
			msg := errorResponseMessage(er)
			if isNotGGUFRepoMessage(msg) {
				return fmt.Errorf("%w", ErrNotGGUFRepo)
			}
			if isShardedGGUFMessage(msg) {
				return fmt.Errorf("%w", ErrShardedGGUF)
			}
			if msg != "" {
				return fmt.Errorf(
					"ociregistry: registry rejected request: %s", msg)
			}
			// Empty body: fall back to the legacy mapping so older
			// HF behaviour stays covered.
			return fmt.Errorf("%w: %s", ErrNotGGUFRepo, er.Error())
		}
	}
	return err
}

// errorResponseMessage returns the concatenated Message fields from
// er.Errors, or "" when none were decoded.
func errorResponseMessage(er *errcode.ErrorResponse) string {
	var msgs []string
	for _, e := range er.Errors {
		if e.Message != "" {
			msgs = append(msgs, e.Message)
		}
	}
	return strings.Join(msgs, "; ")
}

// isNotGGUFRepoMessage reports whether msg matches one of the
// "repository is not a GGUF endpoint" failure modes that HF surfaces
// with a 400 status code. Matching is substring-based because HF
// occasionally tweaks the exact wording.
func isNotGGUFRepoMessage(msg string) bool {
	low := strings.ToLower(msg)
	switch {
	case strings.Contains(low, "not a gguf"),
		strings.Contains(low, "is not gguf"),
		strings.Contains(low, "no gguf"),
		strings.Contains(low, "repository does not contain gguf"):
		return true
	}
	return false
}

// isShardedGGUFMessage reports whether msg is Hugging Face's 400 response
// for a repository whose GGUF is split across multiple shard files. HF
// phrases this in terms of Ollama not supporting sharded GGUF; match on
// "sharded gguf" so wording tweaks around the Ollama reference don't
// break detection.
func isShardedGGUFMessage(msg string) bool {
	return strings.Contains(strings.ToLower(msg), "sharded gguf")
}

// errorNormalizingTransport rewrites 4xx response bodies that use
// non-standard OCI error shapes (notably Hugging Face's
// `{"error":"..."}` plus `x-error-message` header) into the
// `{"errors":[{"code":..,"message":..}]}` layout oras-go expects, so
// the real failure reason reaches translateErr and the user.
type errorNormalizingTransport struct {
	base http.RoundTripper
}

func (t *errorNormalizingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil || resp == nil {
		return resp, err
	}
	if resp.StatusCode < 400 || resp.StatusCode >= 500 {
		return resp, nil
	}
	msg := extractHTTPErrorMessage(resp)
	if msg == "" {
		return resp, nil
	}
	payload, mErr := json.Marshal(struct {
		Errors []map[string]string `json:"errors"`
	}{
		Errors: []map[string]string{
			{"code": "REGISTRY_ERROR", "message": msg},
		},
	})
	if mErr != nil {
		return resp, nil
	}
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(payload))
	resp.ContentLength = int64(len(payload))
	resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(payload)))
	resp.Header.Set("Content-Type", "application/json")
	return resp, nil
}

// extractHTTPErrorMessage returns the most descriptive text it can
// find for a 4xx response, preferring the `x-error-message` header,
// then a non-standard `{"error":"..."}` body. Returns "" when the body
// already uses the OCI `errors` shape (oras-go handles those) or when
// nothing useful is available.
func extractHTTPErrorMessage(resp *http.Response) string {
	if h := resp.Header.Get("X-Error-Message"); h != "" {
		return h
	}
	if resp.Body == nil {
		return ""
	}
	const maxErrorBody = 8 * 1024
	buf := &bytes.Buffer{}
	_, _ = io.Copy(buf, io.LimitReader(resp.Body, maxErrorBody))
	_ = resp.Body.Close()
	body := buf.Bytes()
	// Restore the body so unrelated callers can still read it.
	resp.Body = io.NopCloser(bytes.NewReader(body))
	if len(body) == 0 {
		return ""
	}
	var peek struct {
		Error  string `json:"error"`
		Errors []struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &peek); err != nil {
		return ""
	}
	if len(peek.Errors) > 0 {
		return ""
	}
	return peek.Error
}
