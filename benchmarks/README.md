# GUI renderer benchmark battery

`BenchmarkGUI` (in `cmd/rune`) is an end-to-end benchmark battery for the
ebiten GUI renderer (`internal/term/gui`). Each benchmark drives the **production**
GUI stack — a real `ide.IDE` built through the same `bootstrapHandler`
construction path `runGUI` uses, dispatched through the real
`gui.GUI.Update` / `gui.GUI.Draw`, with GPU commands flushed by the
fork's `benchdraw` package — under a scripted workload.

The goal is to measure the numbers that the production wiring actually
produces (draw calls, glyphs, repainted rows, cells drawn, allocations,
ns/frame), not an isolated micro path, so renderer optimizations can be
gated on real deltas.

## Running

```bash
make bench-gui                       # count=10, benchtime=50x, writes results/<sha>.txt
make bench-gui BENCH_COUNT=6 BENCHTIME=30x
```

Run a single scenario / resolution / variant directly:

```bash
go test -run '^$' -bench 'BenchmarkGUI/scroll/HD/opaque' \
    -benchmem -benchtime=20x ./cmd/rune/
```

Sub-benchmark names are stable — `BenchmarkGUI/<scenario>/<HD|4K>/<opaque|transparent>` —
so runs are comparable across commits.

## Comparing runs

`make bench-gui` writes `benchmarks/results/<git-sha>.txt`. Compare two
commits with [`benchstat`](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat):

```bash
benchstat benchmarks/results/<old-sha>.txt benchmarks/results/<new-sha>.txt
```

The **baseline is captured on `main` before any renderer optimization**;
every later per-phase commit records its measured before/after numbers.

## Reported metrics

Each benchmark reports `ns/op` (nanoseconds per frame) plus `allocs/op`
and `B/op` via `b.ReportAllocs`. These are the honest, hardware-portable
signals for renderer regressions and are what `benchstat` compares.

Draw-call batching (background rects collapsing into one `DrawTriangles`,
glyph runs collapsing to one `DrawTriangles` per atlas page per blend)
is verified structurally in the `internal/term/gui/drawrect` and
`internal/term/gui/drawtext` unit tests rather than via a per-frame GPU-command
metric, so the benchmark battery needs no engine-level counter.

## Scenarios

| Scenario | Stresses |
|---|---|
| `idle` | Update overhead with an early-out Draw (no repaint). |
| `cursor-move` | Single-cell damage: cursor nudged each frame. |
| `typing` | Keystrokes into the editor in insert mode + status-bar churn. |
| `scroll` | Full-viewport damage over a syntax-highlighted Go fixture. |
| `splits` | Three panes of mixed content, one scrolled. |
| `selection-drag` | Mouse sweep producing reverse-attribute churn. |
| `invalidate-burst` | Theme switch every frame — the font/theme reload path. |
| `unicode-stress` | CJK / emoji / box-drawing / braille wide cells scrolled. |
| `worst-case` | Every cell a distinct glyph, scrolled — regression guard. |

## Fidelity caveats

The harness reuses the production option builders and construction path,
but a headless benchmark diverges from a running windowed app in a few
documented ways:

- **No real GPU.** `benchdraw` flushes the command queue through a no-op
  graphics driver, so the draw commands are issued and merged but no
  shader runs and nothing is presented to a window. `ReadPixels` between
  `benchdraw.BeginFrame`/`EndFrame` still returns the CPU-rasterized
  frame (glyph masks are rendered on the CPU), which the differential
  correctness harness hashes; end-to-end GPU pixel output is out of
  scope here.
- **Device scale pinned to 1.** The grid geometry is deterministic across
  machines; a hi-DPI display would resolve a different grid at runtime.
- **Builtin font.** `gui.font_family` is left unset so the built-in font
  is used; no system-font discovery, keeping goldens and grids stable.
- **Network disabled.** The apiclient points at a local `404` HTTP server
  and an unreachable gRPC endpoint; the upgrade auto-check is disabled in
  the config fixture. The weekly nag and crash-report checks resolve to
  their signed-out / no-op behavior.
- **Scenario scripting.** Workloads are injected through the production
  publish channel (`gui.WithPublishChannel`), the same seam the running
  app feeds OS input into. Scenarios that depend on asynchronous
  subsystems whose output is not yet scripted deterministically
  (embedded-terminal byte streams, agent chat streaming, init/wallpaper
  shader animation, completion popups) are intentionally not part of the
  battery yet.

All comparisons must be made on the **same machine** (GPU/driver variance
otherwise dominates), which is why results are keyed by git SHA rather
than committed as absolute targets.

The per-run outputs under `results/` are machine-specific and git-ignored
(only `.gitkeep` is tracked); the durable record is the table below,
refreshed when the renderer changes materially.

## Results: row-damage tracking (Phase 2)

`BenchmarkGUI` pairs the production damage-tracked renderer (`optimized`)
against the full-repaint reference (`reference`, `gui.WithForceFullRepaint`)
in the same binary on the same GPU, so the delta isolates row-damage
tracking. Reproduce the table with:

```bash
make bench-gui BENCH_COUNT=6 BENCHTIME=30x
out=benchmarks/results/$(git rev-parse --short HEAD)
awk '/\/reference-/{sub(/\/reference-/,"-");print}'  "$out".txt > /tmp/ref.txt
awk '/\/optimized-/{sub(/\/optimized-/,"-");print}'  "$out".txt > /tmp/opt.txt
benchstat /tmp/ref.txt /tmp/opt.txt
```

`sec/op`, reference → optimized (Apple M-series, `-count=6 -benchtime=30x`):

| Scenario (HD / 4K) | reference | optimized | Δ |
|---|---|---|---|
| `idle` | ~0.35–0.40 µs | ~0.30–0.34 µs | ~ (noise) |
| `cursor-move` HD | 799 µs | 286 µs | **−64%** |
| `cursor-move` 4K | 2765 µs | 617 µs | **−78%** |
| `typing` HD | 2.53 ms | 1.57 ms | **−38%** |
| `typing` 4K | 4.60 ms | 2.16 ms | **−53%** |
| `scroll` HD | 848 µs | 278 µs | **−67%** |
| `scroll` 4K | 2779 µs | 602 µs | **−78%** |
| `splits` HD | 998 µs | 160 µs | **−84%** |
| `splits` 4K | 4184 µs | 466 µs | **−89%** |
| `selection-drag` HD | 828 µs | 258 µs | **−69%** |
| `selection-drag` 4K | 2825 µs | 933 µs | **−67%** |
| `unicode-stress` HD | 875 µs | 248 µs | **−72%** |
| `unicode-stress` 4K | 2901 µs | 538 µs | **−81%** |
| `invalidate-burst` HD | 21.98 ms | 12.65 ms | **−42%** |
| `worst-case` HD | 2.03 ms | 1.25 ms | **−39%** |
| `worst-case` 4K | 4.73 ms | 1.13 ms | **−76%** |
| **geomean** | 1.016 ms | 367 µs | **−64%** |

Acceptance highlights:

- `cursor-move` / `splits` repaint only the damaged rows (`splits` skips
  its two unchanged panes entirely), so the per-frame work collapses
  toward the cost of the changed strip.
- `worst-case` — every cell distinct, defeating both bg-skip and damage
  tracking — still improves and never regresses: the diff cost is
  bounded, satisfying the regression-guard criterion.
- `idle` is statistically flat: neither path repaints, so damage
  tracking adds no measurable overhead.

## Results: pass separation + glyph atlas (Phases 3–4)

Phase 3 (one `DrawTriangles` per background/underline batch, glyphs
drawn consecutively) and Phase 4 (shelf-packed glyph atlas, one
`DrawTriangles` per atlas page per blend) collapse the per-cell draw
calls the renderer issues — one background rect plus one glyph per cell,
e.g. ~1560 on a dense `worst-case/HD/opaque` grid — into a handful of
`DrawTriangles`: one per background batch plus one per atlas page per
blend, regardless of how many cells changed.

Because this batching applies to both repaint modes it does not show up
in the reference/optimized `ns/op` delta above; it is instead verified
**structurally** in the `internal/term/gui/drawrect` and `internal/term/gui/drawtext` unit
tests — one `Batch.Flush` emits a single `DrawTriangles`, and an open
glyph run of N same-page/same-blend quads stays a single run (4N
vertices, one flush). That is a deterministic, hardware-independent
check, unlike a per-frame GPU-command count.
