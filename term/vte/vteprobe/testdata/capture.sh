#!/usr/bin/env bash
# capture.sh — produce a deterministic 80x24 ANSI screen capture of a
# terminal editor opened on a sample file, plus the cursor position
# reported by tmux. Used by vteprobe tests to drive per-sample,
# per-editor fixtures.
#
# Usage:
#   ./capture.sh <sample> <editor> [line] [col]
#
# <sample> is the directory name under testdata/samples/ (e.g.
#   `go-basic`); the sample file is testdata/samples/<sample>/sample.txt.
# <editor> is the command used to open it (e.g. `vim`, `nvim`, `hx`,
#   `nano`, `kak`, `micro`, `emacs`).
# [line] [col] optionally position the cursor at the given 1-based
#   (line, col) before capturing — handy for capturing scrolled states
#   that exercise alignment code paths (e.g. cursor near EOF in a file
#   with many `}` lines). Defaults: line=1, col=1.
#
# Output lands in testdata/samples/<sample>/<editor>/:
#   screen.ansi   — raw ANSI byte stream from `tmux capture-pane -p -e -S -`
#   cursor.txt    — `<x>,<y>` from tmux display-message
#
# A want.json template is also written next to the capture if one does
# not already exist; edit it to declare the ground-truth (line, col).

set -euo pipefail

if [[ $# -lt 2 || $# -gt 4 ]]; then
    echo "usage: $0 <sample> <editor> [line] [col]" >&2
    exit 2
fi

sample="$1"
editor="$2"
line="${3:-1}"
col="${4:-1}"
root="$(cd "$(dirname "$0")" && pwd)"
sample_file="$root/samples/$sample/sample.txt"
outdir="$root/samples/$sample/$editor"

if [[ ! -f "$sample_file" ]]; then
    echo "no such sample: $sample_file" >&2
    exit 2
fi

mkdir -p "$outdir"

session="capinfer-$$"
trap 'tmux kill-session -t "$session" 2>/dev/null || true' EXIT

tmux new-session -d -s "$session" -x 80 -y 24

# Build the editor command line for the requested (line, col).
# Different editors use different positioning syntaxes; keep the
# mapping editor-specific but compact and centralised here so test
# authors only need one entry point.
case "$editor" in
    vim|nvim)
        # `+N` jumps to line N at load; `+normal! M|` then runs the
        # column motion `M|` (1-based) in normal mode.
        cmd="$editor '+${line}' '+normal! ${col}|' $sample_file"
        ;;
    hx|helix)
        # Helix accepts `<file>:<line>:<col>` directly.
        cmd="$editor $sample_file:${line}:${col}"
        ;;
    nano)
        # nano uses `+LINE,COL`.
        cmd="$editor +${line},${col} $sample_file"
        ;;
    emacs)
        # `emacsclient -t` etc. would also work, but plain `emacs -nw`
        # with `+LINE:COL` is enough for capture fixtures.
        cmd="$editor +${line}:${col} $sample_file"
        ;;
    micro|kak)
        cmd="$editor +${line}:${col} $sample_file"
        ;;
    *)
        # Unknown editor: just open the file; ignore line/col. The
        # captured cursor will land wherever the editor defaults to.
        if [[ "$line" != "1" || "$col" != "1" ]]; then
            echo "warning: $editor has no known positioning syntax; " \
                 "ignoring line=$line col=$col" >&2
        fi
        cmd="$editor $sample_file"
        ;;
esac

tmux send-keys -t "$session" "$cmd" Enter
# Allow time for the editor to paint its initial screen.
sleep 1.5

tmux capture-pane -t "$session" -p -e -S - >"$outdir/screen.ansi"
tmux display-message -t "$session" -p '#{cursor_x},#{cursor_y}' \
    >"$outdir/cursor.txt"

if [[ ! -f "$outdir/want.json" ]]; then
    cat >"$outdir/want.json" <<JSON
{
  "width": 80,
  "height": 24,
  "want": {
    "cursorAtScroll": {"x": $((col - 1)), "y": $((line - 1))},
    "scroll": {"x": 0, "y": 0},
    "tabstop": 8,
    "folded": false
  },
  "minConfidence": 0.6
}
JSON
    echo "wrote $outdir/{screen.ansi,cursor.txt,want.json}"
else
    echo "wrote $outdir/{screen.ansi,cursor.txt}"
fi
