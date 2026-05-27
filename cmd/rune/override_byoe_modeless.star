# Rune user configuration (BYOE editor preset, modeless fallback).
#
# The block under "ACTIVE OVERRIDES" wires Rune to dispatch editing to
# an external TUI editor; Rune still owns tabs, files, and commands,
# while the external process owns the buffer and cursor. The
# `fallback` editor (modeless here) is used for URIs the external
# editor cannot serve, such as Rune's file explorer at memory:///fexplorer.
# Everything below the active block is commented out: uncomment what
# you want to change.

def merge(base, overrides):
    out = dict(base)
    for k, v in overrides.items():
        if k in out and type(out[k]) == "dict" and type(v) == "dict":
            out[k] = merge(out[k], v)
        else:
            out[k] = v
    return out

# ---------------------------------------------------------------------------
# ACTIVE OVERRIDES — required for the BYOE preset
# ---------------------------------------------------------------------------
config = merge(config, {
    "editor": {
        "mode": "byoe",
        "auto_pair": True,
        "byoe": {
            "command": '''<<.Command>>''',
            "goto": '''<<.Goto>>''',
            "fallback": "modeless",
        },
    },
})

# ---------------------------------------------------------------------------
# Switch BYOE editor
#
# `command` is the argv template Rune executes inside a vte to open a
# file. Available substitutions:
#   {file}  absolute path (always required)
#   {line}  1-based line for the initial cursor
#   {col}   1-based column for the initial cursor
# `goto` is a Rune key sequence (handler.ParseSequence syntax) that
# drives the running editor to {line}/{col}. Leave empty to disable
# :goto / click-to-line.
# ---------------------------------------------------------------------------
# # Vim
# config["editor"]["byoe"]["command"] = 'vim "+call cursor({line}, {col})" {file}'
# config["editor"]["byoe"]["goto"]    = '<esc>:{line}<enter>{col}|'
# # Neovim
# config["editor"]["byoe"]["command"] = 'nvim "+call cursor({line}, {col})" {file}'
# config["editor"]["byoe"]["goto"]    = '<esc>:{line}<enter>{col}|'
# # Helix
# config["editor"]["byoe"]["command"] = 'hx {file}:{line}:{col}'
# config["editor"]["byoe"]["goto"]    = '<esc>:goto<space>{line}<enter>'
# # Kakoune
# config["editor"]["byoe"]["command"] = 'kak {file} +{line}:{col}'
# config["editor"]["byoe"]["goto"]    = '<esc>:edit<space>-existing<space>{file}<space>{line}<space>{col}<enter>'
# # Emacs (no window system)
# config["editor"]["byoe"]["command"] = 'emacs -nw +{line}:{col} {file}'
# config["editor"]["byoe"]["goto"]    = '<a-x>goto-line<enter>{line}<enter>'

# config["log_level"] = "info"             # debug | info | warn | error
# config["log_path"]  = "~/.rune/debug.log"

# ---------------------------------------------------------------------------
# GUI theme
#
# Rune ships a catalog of themes:
#   furman, romero, carmack, reynolds, brevik, thompson, pike, hopper,
#   kernighan, wozniak, auge, ritchie, kelleher, sanfilippo, greig, mullen
# Run `:guitheme` to preview live and `:colorPalette` to see named colors.
# Try `mullen` for a deep-purple / hot-pink feel, or `kernighan` for the
# classic Windows-Terminal palette.
# ---------------------------------------------------------------------------
# config["gui"]["default_theme"]       = "romero"
# config["gui"]["font-size"]           = 13
# config["gui"]["line-height-offset"]  = 0
# config["gui"]["column-width-offset"] = -1
# config["gui"]["window_opacity"]      = {"fg": 1, "bg": 1}
# config["gui"]["window_blur_radius"]  = 100

# ---------------------------------------------------------------------------
# Editor behavior (applies to the fallback editor only)
# ---------------------------------------------------------------------------
# config["editor"]["tabspaces"]     = 4
# config["editor"]["ruler"]         = 90
# config["editor"]["autoindent"]    = True
# config["editor"]["auto_save"]     = False
# config["editor"]["initial_folds"] = True

# ---------------------------------------------------------------------------
# Auxiliary bar (left of each editor pane; applies to the fallback editor)
# ---------------------------------------------------------------------------
# config["editor"]["aux_bar"]["enabled"]          = True
# config["editor"]["aux_bar"]["lines"]            = "absolute"  # disabled | absolute | relative
# config["editor"]["aux_bar"]["git"]              = "inline"    # bar | inline | all | disabled
# config["editor"]["aux_bar"]["folds"]            = True
# config["editor"]["aux_bar"]["icons"]            = True
# config["editor"]["aux_bar"]["highlight_cursor"] = True

# ---------------------------------------------------------------------------
# Editor status bar (bottom; applies to the fallback editor)
# ---------------------------------------------------------------------------
# config["editor"]["status_bar"]["enabled"] = True
# config["editor"]["status_bar"]["layout"]  = ' {{ .Status | bg "red" | fg "white" | bold }} █▓▒░  {{ .Filepath | fg "white" }}  {{ .GitShortRef | fg "white" }}   {{ .GitDiffAdd | fg "green" }}   {{ .GitDiffDel | fg "red" }} {{ .ShiftRight }} {{ .CursorColumn | fg "white" }}:{{ .CursorLine | fg "white" }}  {{ .TotalLines | fg "white" }} lines  ░▒▓█ {{ .Language | bold | fg "white" | bg "red" }} '
# config["editor"]["status_bar"]["background_attr"]       = {"bg": "gray"}
# config["editor"]["status_bar"]["foreground_error_attr"] = {"fg": "yellow"}

# ---------------------------------------------------------------------------
# Terminal
# ---------------------------------------------------------------------------
# config["terminal"]["max_lines"]         = 1000
# config["terminal"]["initial_reservoir"] = 0
# config["terminal"]["plugin"]["bar_layout"]        = ' {{ .StatusIcon | bg "gray" | fg "white" }} █▓▒░{{ .AlignCenter }}{{ .Command | fg "white" | bold }}{{ .AlignRight }}  ░▒▓█ {{ .Elapsed | fg "white" | bg "gray" }} '
# config["terminal"]["plugin"]["bar_align_bottom"]  = False
# config["terminal"]["plugin"]["status_error_icon"]   = ""
# config["terminal"]["plugin"]["status_success_icon"] = ""

# ---------------------------------------------------------------------------
# Animations between workspaces
# ---------------------------------------------------------------------------
# config["animations"]["loading_workspace"] = True
# config["animations"]["open_workspace"]    = {"enabled": True}

# ---------------------------------------------------------------------------
# Self-upgrade
# ---------------------------------------------------------------------------
# config["upgrade"]["auto_check_enabled"] = True
# config["upgrade"]["check_period"]       = "24h"

# ---------------------------------------------------------------------------
# Notifications
# ---------------------------------------------------------------------------
# config["notifications"]["auto_close"]   = "5s"
# config["notifications"]["padding"]      = 1
# config["notifications"]["progress_bar"] = True

# ---------------------------------------------------------------------------
# Command prompt
# ---------------------------------------------------------------------------
# config["command"]["key"]         = ":"
# config["command"]["history_key"] = "<m-r>"
# config["command"]["max_history"] = 20000
# config["command"]["aliases"] = {
#     "w": "write",
#     "e": {"command": "edit", "completer": "files"},
#     # Alias with a shell completer (here: git branch names).
#     "gitcheckout": {
#         "command":   "!! git -C $WORKSPACE_PATH checkout $1",
#         "completer": '! git -C $WORKSPACE_PATH branch --format="%(refname:short)"',
#     },
#     "conf": "config",
# }
# config["command"]["key_bindings"]["<m-,>"] = "config"
# config["command"]["key_bindings"]["<m-q>"] = "quit"
# config["command"]["key_bindings"]["<m-p>"] = "searchfile"

# ---------------------------------------------------------------------------
# LLM provider keys. The `models` block configures the router used by
# the agent and `:llm message`. Set `default` to any model your
# configured providers expose.
# ---------------------------------------------------------------------------
# config["models"]["default"]                       = "gpt-5.4"
# config["models"]["openai"]["api_key"]             = ""
# config["models"]["anthropic"]["api_key"]          = ""
# config["models"]["gemini"]["api_key"]             = ""
# config["models"]["codex"]["base_url"]             = ""
# config["models"]["custom"]["url"]                 = ""
# config["models"]["custom"]["api_key"]             = ""
# config["models"]["custom"]["available_models"]    = {}
# config["models"]["local"]["models_cache_dir"]     = ""
# config["models"]["local"]["n_gpu_layers"]         = -1
# config["models"]["local"]["flash_attention"]      = False
