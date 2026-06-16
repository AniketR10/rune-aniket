# Rune user configuration.
#
# `config` is predeclared with Rune's merged default tree; mutate it in
# place to override a setting. Everything below is commented out;
# uncomment what you want to change. See the embedded rune.star for the
# full schema and run `:config` from inside Rune to re-open this file.

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
# config["gui"]["font_size"]           = 13
# config["gui"]["line_height_offset"]  = 0
# config["gui"]["column_width_offset"] = -1
# config["gui"]["window_opacity"]      = {"fg": 1, "bg": 1}
# config["gui"]["window_blur_radius"]  = 100
# Remap physical keys before they reach the editor (see the key-mapping docs).
# Both sides use the command key-binding syntax. <capslock>, <numlock>,
# <scrolllock> and <menu> only act when remapped here.
# config["gui"]["key_mapping"]         = {"<capslock>": "<esc>"}

# ---------------------------------------------------------------------------
# Editor behavior
# ---------------------------------------------------------------------------
# config["editor"]["tabspaces"]     = 4
# config["editor"]["ruler"]         = 90
# config["editor"]["autoindent"]    = True   # syntax-driven indentation
# config["editor"]["auto_pair"]     = False  # auto-pair quotes/brackets/braces
# config["editor"]["auto_save"]     = False  # flush dirty buffers after idle
# config["editor"]["initial_folds"] = True   # collapse language folds on open

# ---------------------------------------------------------------------------
# Editor status bar (bottom)
#
# `layout` is a Go text/template string. Components: Status, Filepath,
# GitShortRef, GitDiffAdd, GitDiffDel, ShiftRight, CursorColumn,
# CursorLine, TotalLines, Language. Pipe operators: bg, fg, bold,
# italic, underline, reverse, dim.
# ---------------------------------------------------------------------------
# config["editor"]["status_bar"]["enabled"] = True
# config["editor"]["status_bar"]["layout"]  = ' {{ .Status | bg "red" | fg "white" | bold }} █▓▒░  {{ .Filepath | fg "white" }}  {{ .GitShortRef | fg "white" }}   {{ .GitDiffAdd | fg "green" }}   {{ .GitDiffDel | fg "red" }} {{ .ShiftRight }} {{ .CursorColumn | fg "white" }}:{{ .CursorLine | fg "white" }}  {{ .TotalLines | fg "white" }} lines  ░▒▓█ {{ .Language | bold | fg "white" | bg "red" }} '
# config["editor"]["status_bar"]["background_attr"]       = {"bg": "gray"}
# config["editor"]["status_bar"]["foreground_error_attr"] = {"fg": "yellow"}

# ---------------------------------------------------------------------------
# Auxiliary bar (left of each editor pane)
# ---------------------------------------------------------------------------
# config["editor"]["aux_bar"]["enabled"]          = True
# config["editor"]["aux_bar"]["lines"]            = "absolute"  # disabled | absolute | relative
# config["editor"]["aux_bar"]["git"]              = "inline"    # bar | inline | all | disabled
# config["editor"]["aux_bar"]["folds"]            = True
# config["editor"]["aux_bar"]["icons"]            = True
# config["editor"]["aux_bar"]["highlight_cursor"] = True

# ---------------------------------------------------------------------------
# Terminal
# ---------------------------------------------------------------------------
# config["terminal"]["modal"]             = True
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
# config["animations"]["open_workspace"] = {
#     "enabled":  True,
#     # "shader":   "burn",
#     # "duration": "1s",
# }

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
#
# `key` opens the command prompt; `history_key` toggles between command
# list and history. Aliases layer on top of built-ins. A string value is
# the substitution; a dict with `command` + `completer` adds completion.
# Set a key_binding value to "" to unbind a built-in.
# ---------------------------------------------------------------------------
# config["command"]["key"]         = ":"
# config["command"]["history_key"] = "<m-r>"
# config["command"]["max_history"] = 20000
# config["command"]["aliases"] = {
#     # Simple alias: `:w` runs `:write`.
#     "w": "write",
#     # Alias with a completer: typing `:e <tab>` suggests files.
#     "e": {"command": "edit", "completer": "files"},
#     # Alias with a shell completer (here: git branch names). The
#     # `! …` template runs a shell line whose stdout becomes one
#     # completion candidate per line.
#     "gitcheckout": {
#         "command":   "!! git -C $WORKSPACE_PATH checkout $1",
#         "completer": '! git -C $WORKSPACE_PATH branch --format="%(refname:short)"',
#     },
#     # Open this configuration file (already bound to <m-,>).
#     "conf": "config",
# }
# config["command"]["key_bindings"]["<m-,>"]     = "config"
# config["command"]["key_bindings"]["<m-q>"]     = "quit"
# config["command"]["key_bindings"]["<m-p>"]     = "searchfile"
# config["command"]["key_bindings"]["<m-\\\\>"] = "searchtext"
# config["command"]["key_bindings"]["<m-enter>"] = "terminalneworsplit"
# config["command"]["key_bindings"]["<a-j>"]     = "lspnextdiagnostic"
# config["command"]["key_bindings"]["<a-k>"]     = "lspprevdiagnostic"

# ---------------------------------------------------------------------------
# LLM provider keys. The `models` block configures the router used by
# the agent and `:llm message`. Set `default` to any model your
# configured providers expose.
# ---------------------------------------------------------------------------
# config["models"]["default"]                       = "gpt-5.4"
# config["models"]["openai"]["api_key"]             = ""
# config["models"]["openai"]["reasoning_effort"]    = ""    # none | minimal | low | medium | high | xhigh | max
# config["models"]["anthropic"]["api_key"]          = ""
# config["models"]["anthropic"]["cache_control"]    = ""    # ephemeral | 5m | 1h | "" (disabled)
# config["models"]["gemini"]["api_key"]             = ""
# config["models"]["codex"]["base_url"]             = ""
# config["models"]["custom"]["url"]                 = ""
# config["models"]["custom"]["api_key"]             = ""
# config["models"]["custom"]["available_models"]    = {}    # name -> context window in tokens
# config["models"]["local"]["models_cache_dir"]     = ""    # empty: $RUNE_DATADIR/models
# config["models"]["local"]["n_gpu_layers"]         = -1    # negative offloads every supported layer
# config["models"]["local"]["flash_attention"]      = False
