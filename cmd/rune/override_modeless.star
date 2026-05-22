# Rune user configuration (modeless editor preset).
#
# The block under "ACTIVE OVERRIDES" switches Rune into modeless editor
# mode and remaps command bindings to the legacy runerc.modeless
# layout. Everything below that block is commented out: uncomment what
# you want to change.
#
# `config` is predeclared with Rune's merged default tree; this script
# merges the active overrides on top, then leaves the rest for you.

def merge(base, overrides):
    out = dict(base)
    for k, v in overrides.items():
        if k in out and type(out[k]) == "dict" and type(v) == "dict":
            out[k] = merge(out[k], v)
        else:
            out[k] = v
    return out

def attr(fg = None, bg = None, flags = None, flag = None):
    out = {}
    if fg != None:
        out["fg"] = fg
    if bg != None:
        out["bg"] = bg
    if flag != None:
        out["flag"] = flag
    if flags != None:
        out["flags"] = flags
    return out

# ---------------------------------------------------------------------------
# ACTIVE OVERRIDES — required for the modeless preset
# ---------------------------------------------------------------------------
config = merge(config, {
    "editor": {
        "mode":      "modeless",
        "auto_pair": True,
        "modeless": {
            "attr":        attr(fg = "default", bg = "default"),
            "bar_attr":    attr(fg = "default", bg = "default"),
            "search_attr": attr(fg = "grey", bg = "yellow"),
        },
        "status_bar": {
            "layout": '██▓▒░  {{ .Filepath }}  {{ .GitShortRef }}   {{ .GitDiffAdd | fg "green" }}   {{ .GitDiffDel | fg "red" }} {{ .ShiftRight }} {{ .CursorColumn }}:{{ .CursorLine }}  {{ .TotalLines }} lines  {{ .Language | bold }}  ░▒▓██',
        },
    },
    "command": {
        "key":          "<s-m-p>",
        "key_bindings": {
            # Blank values intentionally shadow modal defaults so this
            # map matches the legacy runerc.modeless layout exactly.
            "<m-j>":       "",
            "<m-k>":       "",
            "<m-l>":       "",
            "<m-h>":       "",
            "<m-s>":       "write",
            "<a-m-s>":     "writeall",
            "<m-q>":       "quit",
            "<m-s-n>":     "windownew",
            "<m-s-w>":     "windowclose",
            "<a-g>":       "searchtext",
            "<m-r>":       "echo {prompt}jumptoast<space>locals.scm<space>local.definition.type<space>",
            "<s-m-r>":     "searchtype",
            "<m-;>":       "searchtext",
            "<c-m-p>":     "echo <s-m-p>workspacefocus{wait}<space>",
            "<m-f2>":      "locationtoggle bookmark",
            "<f2>":        "jumptolocation next bookmark",
            "<s-f2>":      "jumptolocation previous bookmark",
            "<a-f2>":      "locationhighlight bookmark",
            "<s-m-f2>":    "locationdeleteall bookmark",
            "<a-m-right>": "tabnext",
            "<a-m-left>":  "tabprevious",
            "<m-f>":       "searchtext",
            "<m-g>":       "jumptolocation next search",
            "<s-m-g>":     "jumptolocation prev search",
            "<ctrl-->":    "cursorhistory prev",
            "<ctrl-shift-->": "cursorhistory next",
            "<m-u>":       "cursorhistory prev",
            "<s-m-u>":     "cursorhistory next",
            "<m-,>":       "config",
            "<s-m-]>":     "tabnext",
            "<s-m-[>":     "tabprevious",
            "<c-g>":       "echo <esc>:",
            "<ctrl-meta-p>":       "echo <esc>:workspacefocus<space>",
            "<alt-meta-down>":     "lspgotodef",
            "<f12>":               "lspgotodef",
            "<alt-shift-meta-down>":"lspref",
            "<m-=>":       "guifontsize increase",
            "<m-->":       "guifontsize decrease",
            "<m-t>":       "tabnew",
            "<m-w>":       "tabclose",
            "<a-m-l>":     "tabmove right",
            "<a-m-h>":     "tabmove left",
            "<m-n>":       "windownew",
            "<m-c>":       "clipboardcopy",
            "<m-v>":       "clipboardpaste",
            "<m-y>":       "echolastcmd",
            "<s-m-h>":     "windowfocus left",
            "<s-m-l>":     "windowfocus right",
            "<s-m-j>":     "windowfocus down",
            "<s-m-k>":     "windowfocus up",
            "<a-s-m-h>":   "windowmove left",
            "<a-s-m-l>":   "windowmove right",
            "<a-s-m-j>":   "windowmove down",
            "<a-s-m-k>":   "windowmove up",
            "<s-m-backspace>":"windowresize reset",
            "<s-m-+>":     ["windowresize max width", "windowresize max height"],
            "<s-m-->":     ["windowresize min width", "windowresize min height"],
            "<s-m-up>":    "windowresize increase height",
            "<s-m-down>":  "windowresize decrease height",
            "<s-m-left>":  "windowresize decrease width",
            "<s-m-right>": "windowresize increase width",
            "<s-m-w>":     "windowclose",
            "<c-m-h>":     "windowdefaultsplit h",
            "<c-m-v>":     "windowdefaultsplit v",
            "<s-m-f>":     "windowtogglemaximize",
            "<m-o>":       "lsphover",
            "<m-b>":       "lspformat",
            "<m-m>":       "lspformatimports",
            "<a-j>":       "gitnextchange",
            "<a-k>":       "gitprevchange",
            "<m-p>":       "searchfile",
            "<m-\\\\>":   "searchtext",
            "<m-enter>":   "terminalneworsplit",
            "<s-m-enter>": "!",
            "gf":          "editfileoncursor",
            "<m-1>":       "workspacefocus 1",
            "<m-2>":       "workspacefocus 2",
            "<m-3>":       "workspacefocus 3",
            "<m-4>":       "workspacefocus 4",
            "<m-5>":       "workspacefocus 5",
            "<m-6>":       "workspacefocus 6",
            "<m-7>":       "workspacefocus 7",
            "<m-8>":       "workspacefocus 8",
            "<m-9>":       "workspacefocus 9",
        },
    },
    "terminal": {"modal": False},
})

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
# Editor behavior
# ---------------------------------------------------------------------------
# config["editor"]["tabspaces"]     = 4
# config["editor"]["ruler"]         = 90
# config["editor"]["autoindent"]    = True
# config["editor"]["auto_save"]     = False
# config["editor"]["initial_folds"] = True

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
# Editor status bar (bottom)
# ---------------------------------------------------------------------------
# config["editor"]["status_bar"]["enabled"]               = True
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
# Command prompt — additions to the active modeless overrides above
#
# The active block has already remapped `command.key` to <s-m-p>; you
# can override that here. Aliases layer on top of built-ins. A string
# value is the substitution; a dict with `command` + `completer` adds
# completion. Set a key_binding value to "" to unbind a built-in.
# ---------------------------------------------------------------------------
# config["command"]["history_key"] = "<m-r>"
# config["command"]["max_history"] = 20000
# config["command"]["aliases"] = {
#     # Simple alias: `:w` runs `:write`.
#     "w": "write",
#     # Alias with a completer: typing `:e <tab>` suggests files.
#     "e": {"command": "edit", "completer": "files"},
#     # Alias with a shell completer (here: git worktree names).
#     "wtopen": {
#         "command":   "workspacenew $RUNE_DATADIR/worktrees/$WORKSPACE-$WORKSPACE_HASH/$1",
#         "completer": "! $SHELL -c 'git worktree list | tail -n +2 | cut -d\" \" -f1 | xargs -n1 basename'",
#     },
#     "conf": "config",
# }

# ---------------------------------------------------------------------------
# LLM provider keys. The `models` block configures the router used by
# the agent and `:llm message`. Set `default` to any model your
# configured providers expose.
# ---------------------------------------------------------------------------
# config["models"]["default"]                       = "gpt-5.4"
# config["models"]["openai"]["api_key"]             = ""
# config["models"]["openai"]["reasoning_effort"]    = ""
# config["models"]["anthropic"]["api_key"]          = ""
# config["models"]["anthropic"]["cache_control"]    = ""
# config["models"]["gemini"]["api_key"]             = ""
# config["models"]["codex"]["base_url"]             = ""
# config["models"]["custom"]["url"]                 = ""
# config["models"]["custom"]["api_key"]             = ""
# config["models"]["custom"]["available_models"]    = {}
# config["models"]["local"]["models_cache_dir"]     = ""
# config["models"]["local"]["n_gpu_layers"]         = -1
# config["models"]["local"]["flash_attention"]      = False
