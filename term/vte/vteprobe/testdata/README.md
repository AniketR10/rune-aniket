# vteprobe testdata

This directory holds editor screen captures used to verify that
`vteprobe` correctly maps a terminal cursor to a file `(line, col)`
across multiple editors AND multiple file contents.

## Layout

```
testdata/
├── README.md
├── capture.sh
└── samples/
    └── <sample>/
        ├── sample.txt        # the file content under test
        └── <editor>/
            ├── screen.ansi   # raw ANSI from `tmux capture-pane -p -e -S -`
            ├── cursor.txt    # `<x>,<y>` from tmux display-message
            └── want.json     # ground-truth expectations (see below)
```

Each `<sample>` is independent: it bundles a sample file with one
capture per editor that has been recorded against it. Samples
without any editor captures are tolerated; they simply cause the
matching subtest to skip with a clear message.

## Adding a sample

To exercise a new content scenario — for example to reproduce a bug
against a specific kind of file — drop a new sample under
`samples/`:

1. Create the file: `samples/my-bug/sample.txt`
2. Record one or more editor screens against it:

   ```bash
   ./capture.sh my-bug vim
   ./capture.sh my-bug nvim
   ./capture.sh my-bug hx
   ./capture.sh my-bug nano
   ```

   `capture.sh` writes `screen.ansi`, `cursor.txt`, and a stub
   `want.json` into `samples/my-bug/<editor>/`.

   To capture a scrolled state (for example a cursor near EOF that
   exercises alignment heuristics), pass the 1-based `(line, col)`
   the editor should jump to before capturing:

   ```bash
   ./capture.sh my-bug vim 325 6
   ./capture.sh my-bug hx  325 6
   ```

   The script knows the positioning syntax for vim, nvim, hx,
   nano, emacs, micro, and kak; for other editors it ignores the
   line/col and just opens the file.

3. Edit each `want.json` so it declares the file position the cursor
   *should* resolve to, plus the minimum confidence the inferrer
   must report. Coordinates are 0-based (matching `term.Coordinates`):

   ```json
   {
     "width": 80,
     "height": 24,
     "want": {
       "cursorAtScroll": {"x": 2, "y": 4},
       "scroll":         {"x": 0, "y": 0},
       "tabstop": 4,
       "folded": false
     },
     "minConfidence": 0.7
   }
   ```

   - `cursorAtScroll` is the 0-based `(rune column, file line)` the
     cursor should map to.
   - `scroll` is the 0-based file position of the top-left of the
     content band; for a buffer that displays the top of the file
     leave both fields at 0. (Horizontal-scroll detection is not yet
     implemented; reserve `scroll.x` for future use.)
   - `tabstop` is checked only when non-zero; set it to assert which
     candidate tabstop the inferrer should pick.
   - `folded` is checked verbatim against `Result.Folded`.

   `width` and `height` must match the terminal dimensions that
   produced the capture; `vteprobe` will Replay the byte stream at
   exactly those dimensions.

4. Run `go test ./term/vteprobe/`. The new case appears as
   `TestInferEditorCaptures/my-bug/<editor>` and activates
   automatically because the test discovers samples and editors at
   runtime.

## Existing samples

- **go-basic** — a 12-line Go program with one `\t`-indented line,
  shared across vim, nvim, helix, and nano captures. The cursor sits
  on line 1 in every editor; this is the entry-level fixture.
- **go-complex** — a 60+ line Go program exercising soft-wrap,
  gutter-lookalike content (`" 1 "` as a literal), and fold-marker
  lookalike content (`+-- 3 lines` as a literal). No captures yet —
  generate them with `./capture.sh go-complex <editor>`.

## How `screen.ansi` is replayed

`tmux capture-pane -p -e -S -` prints one rendered terminal row per
line separated by `\n`, with embedded SGR escapes but **no**
cursor-position escapes between rows. To turn that into a Replay-able
byte stream `editor_test.go` wraps every row in a CUP escape
(`\x1b[<row>;1H`) plus an SGR reset, padding or truncating to the
recorded height, and appends a final CUP to the cursor reported by
tmux. This re-anchors each row so the VTE replay reconstructs the
exact screen tmux captured.

## Known editor quirks the suite already exercises

- **vim** / **nvim** — no gutter; tab characters render as 8 spaces
  by default (`tabstop=8`).
- **helix** — left gutter with right-aligned line numbers padded by
  two spaces before the body; trailing one-shot message rows below
  the status bar.
- **nano** — italic "File: <path>" title bar at the top; two
  shortcut-help rows at the bottom.
