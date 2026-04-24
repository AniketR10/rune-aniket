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
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/go-tui/cmd/rune-agent/llm/llamacpp"
	"unstable.build/go-tui/cmd/rune-agent/llm/llamacpp/ociregistry"
)

// downloadUsage is the markdown help block returned when the user runs
// `local download` without arguments (or with too many). It is also reused by
// `help local download` so the normal manual path shows the same examples.
const downloadUsage = "## `local download <reference>`\n\n" +
	"Download a GGUF model from an OCI registry into the local cache.\n\n" +
	"When the reference has no host, `" + llamacpp.DefaultRegistryHost + "` is assumed.\n\n" +
	"### Synopsis\n\n" +
	"`local download <host/>owner/repo[:tag|@digest]`\n\n" +
	"### Examples\n\n" +
	"- `local download unsloth/gemma-3n-E2B-it-GGUF:Q4_K_M`  \n" +
	"  Pulls the `Q4_K_M` tag from Hugging Face.\n" +
	"- `local download hf.co/unsloth/gemma-3n-E2B-it-GGUF`  \n" +
	"  Same thing with an explicit (short) host and the default `latest` tag.\n" +
	"- `local download huggingface.co/foo/bar@sha256:abc…`  \n" +
	"  Pulls a specific digest instead of a tag.\n" +
	"- `local download docker.io/library/myrepo:v1`  \n" +
	"  Works against any OCI v2 registry, not just Hugging Face.\n\n" +
	"See `help local download` for this manual from the REPL."

// handleDownload pulls a model through llamacpp.Registry. Progress is
// streamed to the REPL ProgressWriter; a single markdown summary is
// yielded when the download completes. All OCI / cache details are
// owned by the registry — this handler is a pure UI adapter.
func (s *shell) handleDownload(
	ctx context.Context, args []string, pw repl.ProgressWriter,
) (iterator.Iterator[component.Responsive], error) {
	if len(args) != 1 {
		// downloadUsage is a markdown help block that intentionally
		// ends with punctuation; the REPL renders it as markdown.
		return nil, errors.New(downloadUsage) //nolint:staticcheck // markdown help block, not a sentence-style error
	}

	ref, err := llamacpp.ParseReference(args[0])
	if err != nil {
		return nil, fmt.Errorf("parse reference: %w", err)
	}

	// Show an indeterminate 0% progress bar immediately so the user sees
	// *something* while we wait for the manifest fetch to complete. Layer
	// downloads overwrite this with real byte counts as soon as they
	// start streaming.
	if pw != nil {
		pw.Progress(0, 0, "B")
	}

	// Start the pull on a goroutine. We bridge two channels back to the
	// iterator:
	//  - progressCh carries live rate updates (throttled)
	//  - resultCh carries the final outcome
	type progress struct {
		downloaded, total int64
	}
	progressCh := make(chan progress, 8)
	type result struct {
		res *llamacpp.DownloadResult
		err error
	}
	resultCh := make(chan result, 1)

	go func() {
		pr, err := s.localRegistry.Download(ctx, ref,
			func(downloaded, total int64) {
				select {
				case progressCh <- progress{downloaded, total}:
				default:
					// Channel full — drop this sample; the next one
					// will catch up. Progress is purely informational.
				}
			})
		// Signal done by closing progress first so the iterator's drain
		// loop observes it before the result.
		close(progressCh)
		resultCh <- result{res: pr, err: err}
	}()

	// Throttled progress -> ProgressWriter. We don't yield Responsives for
	// progress updates; the REPL's ProgressWriter renders them directly in
	// the prompt line.
	var lastPwUpdate time.Time
	pump := func(p progress) {
		if pw == nil {
			return
		}
		// Always emit terminal samples (downloaded >= total > 0) so
		// the bar lands at 100% even when the throttle would
		// otherwise drop them. Without this, fast or already-resumed
		// downloads leave the bar stuck at whatever the last
		// throttled tick reported.
		terminal := p.total > 0 && p.downloaded >= p.total
		now := time.Now()
		if !terminal && now.Sub(lastPwUpdate) < 200*time.Millisecond {
			return
		}
		lastPwUpdate = now
		scaled, scaledTotal, unit := scaleProgressBytes(p.downloaded, p.total)
		pw.Progress(scaled, scaledTotal, unit)
	}

	// done tracks whether the iterator has already emitted the final
	// summary (or error). FromFunc keeps calling Next until the returned
	// bool is false, so we must hard-stop the second time around.
	done := false
	return iterator.FromFunc(
		func(ctx context.Context) (component.Responsive, bool, error) {
			if done {
				return nil, false, nil
			}
			// Drain progress until closed, then read the final result and
			// return the summary as the iterator's only yielded item.
			for {
				select {
				case <-ctx.Done():
					done = true
					return nil, false, ctx.Err()
				case p, ok := <-progressCh:
					if !ok {
						// Progress channel closed — wait for result.
						select {
						case <-ctx.Done():
							done = true
							return nil, false, ctx.Err()
						case r := <-resultCh:
							done = true
							if r.err != nil {
								return nil, false, r.err
							}
							return downloadSummary(r.res, ref), true, nil
						}
					}
					pump(p)
				}
			}
		},
		func() error { return nil },
	), nil
}

// downloadSummary renders a short markdown block describing what was
// pulled, including the on-disk path of every layer that ended up in the
// cache. The summary is intentionally small — the full manifest is
// available via the model registry.
func downloadSummary(pr *llamacpp.DownloadResult, requested llamacpp.Reference) component.Responsive {
	if pr == nil {
		return responsiveString("download: no result")
	}
	ref := pr.Reference
	// Pull may canonicalise the reference (e.g. filling in a default
	// tag); fall back to what the user actually typed when the result's
	// copy is empty.
	if ref.Host == "" && ref.Repository == "" {
		ref = requested
	}
	var b strings.Builder
	fmt.Fprintf(&b, "## Downloaded `%s`\n\n", ref.String())
	if pr.ModelPath != "" {
		fmt.Fprintf(&b, "- **Model**: `%s`\n", pr.ModelPath)
	}
	for _, layer := range pr.LayerSizes {
		fmt.Fprintf(&b, "- %s: `%s` (%s)\n",
			shortMediaType(layer.MediaType), layer.Path, humanBytes(layer.Size))
	}
	b.WriteByte('\n')
	b.WriteString("Use `/agent models` to see it in the list.\n")
	return markdownResponsive(b.String())
}

// markdownResponsive wraps a string in a markdown component, falling back
// to a plain responsive string when parsing fails. Mirrors markdownOutput
// but returns a single Responsive instead of an iterator.
func markdownResponsive(content string) component.Responsive {
	it := markdownOutput(content)
	defer func() { _ = it.Close() }()
	if r, ok := it.Next(context.Background()); ok {
		return r
	}
	return component.NewResponsiveString(content, component.StringResponsiveConfig{})
}

// shortMediaType returns a user-friendly label for known Ollama layer
// media types, and the media type itself as a fallback.
func shortMediaType(mt string) string {
	switch mt {
	case ociregistry.MediaTypeOllamaModel:
		return "model"
	case ociregistry.MediaTypeOllamaTemplate:
		return "template"
	case ociregistry.MediaTypeOllamaParams:
		return "params"
	case ociregistry.MediaTypeOllamaSystem:
		return "system"
	case ociregistry.MediaTypeOllamaAdapter:
		return "adapter"
	case ociregistry.MediaTypeOllamaLicense:
		return "license"
	}
	return mt
}

// humanBytes formats a byte count as KiB/MiB/GiB, matching the scale used
// by most HF download UIs.
func humanBytes(n int64) string {
	const (
		k = 1024
		m = 1024 * 1024
		g = 1024 * 1024 * 1024
	)
	switch {
	case n >= g:
		return fmt.Sprintf("%.1f GiB", float64(n)/float64(g))
	case n >= m:
		return fmt.Sprintf("%.1f MiB", float64(n)/float64(m))
	case n >= k:
		return fmt.Sprintf("%.1f KiB", float64(n)/float64(k))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

// wrapPullError was moved into llamacpp.Registry so error-shape
// translation happens next to the transport. The shell just surfaces
// whatever Registry.Download returns.

// scaleProgressBytes maps a (downloaded, total) byte pair into a shared,
// human-readable unit suitable for a repl.ProgressWriter. The unit is
// chosen from `total` so the denominator stays stable across updates as
// the download grows. Both values are returned in the chosen unit with
// integer truncation — the progress bar only renders integers, so the
// small loss of precision is acceptable and the label remains legible
// even for multi-GiB downloads.
//
// The unit is chosen so the scaled total has at least ~100 ticks, giving
// the REPL progress bar roughly 1% granularity. Using the largest
// fitting unit instead (e.g. GiB for a 21 GiB download) would pin total
// at 21 and make the bar jump ~5% per tick.
//
// We keep this in the agent-side code (rather than the SDK's
// ProgressBar) so the component stays unit-agnostic: "B" is not a
// meaning the UI framework should understand.
func scaleProgressBytes(downloaded, total int64) (int64, int64, string) {
	const (
		kib = int64(1024)
		mib = 1024 * kib
		gib = 1024 * mib
		tib = 1024 * gib

		// minTicks is the smallest total-in-unit value for which we
		// consider a given unit "fine-grained enough" for the progress
		// bar. 100 yields ~1% resolution.
		minTicks = int64(100)
	)
	// Clamp progress above total so misbehaving sources can't push the
	// bar past 100%.
	if total > 0 && downloaded > total {
		downloaded = total
	}
	// Pick the largest unit where the scaled total still has at least
	// `minTicks` increments. This keeps the label compact while
	// preserving ~1% granularity on the bar.
	units := []struct {
		div  int64
		name string
	}{
		{tib, "TiB"},
		{gib, "GiB"},
		{mib, "MiB"},
		{kib, "KiB"},
	}
	for _, u := range units {
		if total/u.div >= minTicks {
			return downloaded / u.div, total / u.div, u.name
		}
	}
	return downloaded, total, "B"
}
