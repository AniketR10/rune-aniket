#!/usr/bin/env bash
# capture.sh — produce a deterministic ANSI screen capture of a
# terminal editor opened on a sample file, plus the cursor position
# reported by tmux. Used by vteprobe tests to drive per-sample,
# per-editor fixtures.
#
# Usage:
#   ./capture.sh [--wrap] [--filetype <ft>] [--width N] [--height N] \
#       <sample> <editor> [line] [col] [post-keys]
#
# <sample> is the directory name under testdata/samples/ (e.g.
#   `go-basic`); the sample file is testdata/samples/<sample>/sample.txt.
# <editor> is the command used to open it (e.g. `vim`, `nvim`, `hx`,
#   `nano`, `kak`, `micro`, `emacs`).
# [line] [col] optionally position the cursor at the given 1-based
#   (line, col) before capturing — handy for capturing scrolled states
#   that exercise alignment code paths (e.g. cursor near EOF in a file
#   with many `}` lines). Defaults: line=1, col=1.
# [post-keys] is an optional literal key sequence fed to the editor
#   via `tmux send-keys` after the initial positioning and before
#   the screen is captured. Use it to reproduce bugs that only
#   manifest after a specific motion (e.g. `k` to move up from the
#   line the initial positioning landed on). The string is passed
#   verbatim, so use tmux's literal-key syntax (no Enter appended).
# --wrap forces soft-wrap on so long lines wrap inside the 80-column
#   pane instead of scrolling horizontally. The per-editor incantation
#   is: vim/nvim `+set wrap`; helix `:set soft-wrap.enable true`;
#   nano `-S`; emacs `(global-visual-line-mode 1)`. Other editors
#   ignore the flag.
# --filetype <ft> forces the editor's syntax/filetype to <ft> (e.g.
#   `go`, `python`). Useful when the sample is named `sample.txt` but
#   should be rendered with syntax highlighting. vim/nvim:
#   `+set filetype=<ft>`; helix: `:set-language <ft>`; emacs:
#   `+--eval '(<ft>-mode)'`. Other editors ignore the flag.
# --width N, --height N set the tmux pane geometry to N columns/rows
#   (defaults: 80x24). The chosen dimensions are recorded in
#   want.json so vte.Replay re-creates the same grid.
#
# Output lands in testdata/samples/<sample>/<editor>/:
#   screen.ansi   — raw ANSI byte stream from `tmux capture-pane -p -e -S -`
#   cursor.txt    — `<x>,<y>` from tmux display-message
#
# A want.json template is also written next to the capture if one does
# not already exist; edit it to declare the ground-truth (line, col).

set -euo pipefail

wrap=0
filetype=""
width=80
height=24
while [[ $# -gt 0 ]]; do
    case "$1" in
        --wrap) wrap=1; shift;;
        --filetype) filetype="$2"; shift 2;;
        --width) width="$2"; shift 2;;
        --height) height="$2"; shift 2;;
        --) shift; break;;
        -*)
            echo "unknown flag: $1" >&2
            exit 2
            ;;
        *) break;;
    esac
done

if [[ $# -lt 2 || $# -gt 5 ]]; then
    echo "usage: $0 [--wrap] [--filetype <ft>] [--width N] [--height N] <sample> <editor> [line] [col] [post-keys]" >&2
    exit 2
fi

sample="$1"
editor="$2"
line="${3:-1}"
col="${4:-1}"
postkeys="${5:-}"
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

tmux new-session -d -s "$session" -x "$width" -y "$height"

# Build the editor command line for the requested (line, col).
# Different editors use different positioning syntaxes; keep the
# mapping editor-specific but compact and centralised here so test
# authors only need one entry point.
case "$editor" in
    vim|nvim)
        # `+call cursor(L, C)` positions to byte-column C on line L.
        # Byte-col (vs. screen-col `|`) so leading tabs/multibyte runes
        # do not silently shift the cursor.
        wrap_opts=""
        if [[ "$wrap" == "1" ]]; then
            wrap_opts="'+set wrap'"
        fi
        ft_opts=""
        if [[ -n "$filetype" ]]; then
            ft_opts="'+set filetype=${filetype}'"
        fi
        cmd="$editor ${wrap_opts} ${ft_opts} '+call cursor(${line}, ${col})' $sample_file"
        ;;
    hx|helix)
        # Helix accepts `<file>:<line>:<col>` directly.
        cmd="$editor $sample_file:${line}:${col}"
        if [[ "$wrap" == "1" ]]; then
            # Soft-wrap is toggled via the `:set` ex-command after open.
            postkeys=":set soft-wrap.enable true Enter ${postkeys}"
        fi
        if [[ -n "$filetype" ]]; then
            postkeys=":set-language ${filetype} Enter ${postkeys}"
        fi
        ;;
    nano)
        # nano uses `+LINE,COL`; `-S` enables soft-wrap.
        wrap_opts=""
        if [[ "$wrap" == "1" ]]; then
            wrap_opts="-S"
        fi
        cmd="$editor ${wrap_opts} +${line},${col} $sample_file"
        ;;
    emacs)
        # `emacsclient -t` etc. would also work, but plain `emacs -nw`
        # with `+LINE:COL` is enough for capture fixtures.
        wrap_opts=""
        if [[ "$wrap" == "1" ]]; then
            wrap_opts="--eval '(global-visual-line-mode 1)'"
        fi
        ft_opts=""
        if [[ -n "$filetype" ]]; then
            ft_opts="--eval '(${filetype}-mode)'"
        fi
        cmd="$editor -nw ${wrap_opts} ${ft_opts} +${line}:${col} $sample_file"
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

if [[ -n "$postkeys" ]]; then
    # Word-split so each whitespace-separated token in $postkeys
    # becomes a distinct tmux key (e.g. `G k k k` => four key sends).
    # shellcheck disable=SC2086
    tmux send-keys -t "$session" $postkeys
    # Give the editor a beat to repaint after the motion.
    sleep 0.3
fi

tmux capture-pane -t "$session" -p -e -S - >"$outdir/screen.ansi"
tmux display-message -t "$session" -p '#{cursor_x},#{cursor_y}' \
    >"$outdir/cursor.txt"

if [[ ! -f "$outdir/want.json" ]]; then
    cat >"$outdir/want.json" <<JSON
{
  "width": $width,
  "height": $height,
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
