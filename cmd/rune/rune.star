# Rune default configuration, in Starlark form.
#
# The script's final top-level `config` dict is handed to the Rune config
# loader exactly as if it had been written in YAML.
#
# Use normal Starlark `if` / `for` / dict-merge idioms to customise.

# The Rune loader passes `mode` ("modal" | "modeless") and `tui` (bool) as
# predeclared globals; the script branches on them below.

# Box-drawing frame characters used by the terminal-specific overrides below.
TUI_FRAME_CHARSET = {
    "horizontaltop":    "─",
    "horizontalbottom": "─",
    "verticalright":    "│",
    "verticalleft":     "│",
    "topleft":          "┌",
    "topright":         "┐",
    "bottomleft":       "└",
    "bottomright":      "┘",
}

# GUI frame characters used by the default window-manager configuration.
GUI_FRAME_CHARSET = {
    "horizontaltop":    "▔",
    "horizontalbottom": "▁",
    "verticalright":    "▕",
    "verticalleft":     "▏",
    "topleft":          "🭽",
    "topright":         "🭾",
    "bottomleft":       "🭼",
    "bottomright":      "🭿",
}

def merge(base, overrides):
    """Recursive dict merge; `overrides` values win at every level."""
    out = dict(base)
    for k, v in overrides.items():
        if k in out and type(out[k]) == "dict" and type(v) == "dict":
            out[k] = merge(out[k], v)
        else:
            out[k] = v
    return out

def attr(fg = None, bg = None, flags = None, flag = None):
    """Build a text-attribute dict.

    All arguments are optional; only those explicitly passed show up in the
    result. `flags` may be a single string ("bold") or a list of strings
    (["bold", "italic"]), and `flag` is accepted as a legacy alias for the
    singular form. The rune config loader accepts both.
    """
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

# Themes available to the `guitheme` command and the `gui.default_theme`
# property below. A theme consists of color-name overrides for Rune's GUI.
#
# There are three special keys: `foreground`, `background`, and `cursor`.
# The remaining keys override W3C color names. Run the `colorPalette`
# command for a full list of named colors.
GUI_THEMES = {
    "furman": {
        "foreground": "#a9b7c5", "background": "#202020", "black": "#1d1f21",
        "maroon": "sienna", "green": "#678455", "olive": "#C07C41",
        "navy": "#585858", "purple": "#8E73A2", "teal": "#498d8e",
        "silver": "#808080", "gray": "#3d3f41", "red": "#B95A53",
        "lime": "#649149", "yellow": "#c4742e", "blue": "#808080",
        "magenta": "#c683c4", "cyan": "#70dadb", "white": "#a9b7c5",
        "cursor": "magenta",
    },
    "romero": {
        "foreground": "#bbc2cf", "background": "#000000", "black": "#000000",
        "maroon": "#ff2400", "green": "#98be65", "olive": "#ecbe7b",
        "navy": "#5f0000", "purple": "#e50000", "teal": "#46d9ff",
        "silver": "#666666", "gray": "#282c34", "red": "#990000",
        "lime": "#98be65", "yellow": "#ecbe7b", "blue": "#ba0e2e",
        "magenta": "#ff6c6b", "cyan": "#46d9ff", "white": "#bbc2cf",
        "cursor": "#e50000",
    },
    "carmack": {
        "foreground": "#D0C6AB", "background": "#080808", "black": "#080808",
        "maroon": "#4d2a1c", "green": "#98be65", "olive": "darkorange",
        "navy": "#98971a", "teal": "#46d9ff", "purple": "#456e88",
        "silver": "#756958", "gray": "#231B0E", "red": "saddlebrown",
        "lime": "greenyellow", "yellow": "#ecbe7b", "blue": "#d79921",
        "magenta": "#83a58e", "cyan": "skyblue", "white": "#D0C6AB",
        "cursor": "#b5bd68",
    },
    "reynolds": {
        "foreground": "#ebdbb2", "background": "#121212", "black": "#121212",
        "maroon": "#e6c547", "green": "#00ff00", "olive": "darkolivegreen",
        "navy": "seagreen", "purple": "#458588", "teal": "darkturquoise",
        "silver": "#64697a", "gray": "#282c34", "red": "saddlebrown",
        "lime": "#d7ff00", "yellow": "goldenrod", "blue": "limegreen",
        "magenta": "#83a598", "cyan": "#46d9ff", "white": "#ebdbb2",
        "cursor": "#00ff00",
    },
    "brevik": {
        "foreground": "#e6e4dc", "background": "#080808", "black": "#080808",
        "maroon": "#501c1f", "green": "#31601e", "olive": "#b9984d",
        "purple": "brown", "navy": "indigo", "teal": "darkslategrey",
        "silver": "#5e6980", "gray": "#282c34", "red": "#cd2e20",
        "lime": "#31601e", "yellow": "#b9984d", "blue": "#8700ff",
        "magenta": "#d70000", "cyan": "#0000ff", "white": "#e6e4dc",
        "cursor": "#cd2e20",
    },
    "thompson": {
        "foreground": "#ffffff", "background": "#0f1114", "cursor": "#ba0e2e",
        "black": "#222936", "maroon": "#ba0e2e", "green": "#6ab0a3",
        "olive": "#52708b", "navy": "#508aaa", "purple": "#508aaa",
        "teal": "#6ab0a3", "silver": "#8ca2bf", "gray": "#3f4d64",
        "red": "#ba0e2e", "lime": "#6ab0a3", "yellow": "#52708b",
        "blue": "#508aaa", "magenta": "#508aaa", "cyan": "#6ab0a3",
        "white": "#ffffff",
    },
    "pike": {
        "foreground": "#000000", "background": "#ffffea", "black": "#000000",
        "maroon": "#c44646", "green": "#427840", "olive": "#775500",
        "navy": "#000000", "purple": "#003366", "teal": "#003366",
        "silver": "#000000", "gray": "#9d9dc2", "red": "#ca7c5e",
        "lime": "limegreen", "yellow": "#080877", "blue": "#003366",
        "cyan": "#6e6e9e", "magenta": "#6e6e9e", "white": "#ffffea",
        "cursor": "red",
    },
    "hopper": {
        "foreground": "#c1c1c1", "background": "#000000", "black": "#000000",
        "maroon": "#616161", "green": "#a8a8a8", "olive": "#c4c4c4",
        "navy": "#1c1c1c", "purple": "#444444", "teal": "#b1b1b1",
        "silver": "#c1c1c1", "gray": "#2c2c2c", "red": "#121212",
        "lime": "#a8a8a8", "yellow": "#c4c4c4", "blue": "#454545",
        "magenta": "#989898", "cyan": "#b1b1b1", "white": "#c1c1c1",
        "cursor": "#444444",
    },
    "kernighan": {
        "foreground": "#cccccc", "background": "#0c0c0c", "black": "#0c0c0c",
        "maroon": "#c50f1f", "green": "#13a10e", "olive": "#c19c00",
        "navy": "#0037da", "purple": "#881798", "teal": "#3a96dd",
        "silver": "#bdbdbd", "gray": "#767676", "red": "#e74856",
        "lime": "#16c60c", "yellow": "#f9f1a5", "blue": "#3b78ff",
        "magenta": "#b4009e", "cyan": "#61d6d6", "white": "#f2f2f2",
        "cursor": "#e74856",
    },
    "wozniak": {
        "foreground": "#D8D1E2", "background": "#120F1D", "gray": "#2f2c36",
        "black": "#1D192B", "maroon": "#F33E70", "green": "#2DA074",
        "olive": "#E4FB8A", "navy": "#3B2E58", "purple": "#8F46FF",
        "teal": "#00FFD1", "silver": "#665f75", "red": "#6346FF",
        "lime": "#23C07B", "yellow": "#FE45FF", "blue": "#6346FF",
        "magenta": "#D1E67D", "cyan": "#0F93f6", "white": "#D4D0D4",
        "cursor": "#23C07B",
    },
    "auge": {
        "foreground": "#d9d9d9", "background": "#270d06", "black": "#330000",
        "maroon": "darkred", "green": "#ff6600", "olive": "#ff9900",
        "navy": "#ffcc00", "purple": "#ff6600", "teal": "#ff9900",
        "silver": "#966f51", "gray": "#663300", "red": "#ff3300",
        "lime": "#ff9966", "yellow": "#ffcc99", "blue": "#ffcc33",
        "magenta": "#ff9966", "cyan": "#ffcc99", "white": "#d9d9d9",
        "cursor": "#ff9966",
    },
    "ritchie": {
        "foreground": "#fffbf6", "background": "#101421", "black": "#2e2e2e",
        "maroon": "#eb4129", "green": "#abe047", "olive": "#f6c744",
        "navy": "#47a0f3", "purple": "#7b5cb0", "teal": "#64dbed",
        "silver": "#aba9a9", "gray": "#565656", "red": "#ec5357",
        "lime": "#c0e17d", "yellow": "#f9da6a", "blue": "#49a4f8",
        "magenta": "#a47de9", "cyan": "#99faf2", "white": "#ffffff",
        "cursor": "#c0e17d",
    },
    "kelleher": {
        "foreground": "#ffffff", "background": "#000000", "black": "#1a1a1a",
        "maroon": "#f4005f", "green": "#98e024", "olive": "#fa8419",
        "navy": "#9d65ff", "purple": "#f4005f", "teal": "#58d1eb",
        "silver": "#ada582", "gray": "#625e4c", "red": "#f4005f",
        "lime": "#98e024", "yellow": "#e0d561", "blue": "#9d65ff",
        "magenta": "#f4005f", "cyan": "#58d1eb", "white": "#f6f6ef",
        "cursor": "#98e024",
    },
    "sanfilippo": {
        "foreground": "#c7c7c7", "background": "#000000", "black": "#616161",
        "maroon": "#ff8272", "green": "#b4fa72", "olive": "#fefdc2",
        "navy": "#a5d5fe", "purple": "#ff8ffd", "teal": "#d0d1fe",
        "silver": "#f1f1f1", "gray": "#8e8e8e", "red": "#ffc4bd",
        "lime": "#d6fcb9", "yellow": "#fefdd5", "blue": "#c1e3fe",
        "magenta": "#ffb1fe", "cyan": "#e5e6fe", "white": "#fffeff",
        "cursor": "#d6fcb9",
    },
    "greig": {
        "foreground": "#ffffff", "background": "#262335", "black": "#262335",
        "maroon": "#fe4450", "green": "#72f1b8", "olive": "#f3e70f",
        "navy": "#03edf9", "purple": "#ff7edb", "teal": "#03edf9",
        "silver": "#9c84bf", "gray": "#614d85", "red": "#fe4450",
        "lime": "#72f1b8", "yellow": "#fede5d", "blue": "#03edf9",
        "magenta": "#ff7edb", "cyan": "#03edf9", "white": "#ffffff",
        "cursor": "#fe4450",
    },
    "mullen": {
        "foreground": "#ccccce", "background": "#0d0d1b", "black": "#282828",
        "maroon": "#ca1444", "green": "#789aba", "olive": "#b3879f",
        "navy": "#95569b", "purple": "#cb6fa1", "teal": "#fb6e93",
        "silver": "#cf98c1", "gray": "#98218e", "red": "#cb515d",
        "lime": "#5a87b1", "yellow": "#f2a297", "blue": "#9a77b1",
        "magenta": "#9c61ab", "cyan": "#f4436f", "white": "#ebdbb2",
        "cursor": "#5a87b1",
    },
}

config = {
    # Logging file path. This property is not reloaded on reloadWorkspace.
    "log_path":  "~/.rune/debug.log",
    # Log level. This property is not reloaded on reloadWorkspace.
    "log_level": "info",
    # Whether to use the system clipboard or Rune's in-memory clipboard.
    "clipboard": "system",
    # LSP configuration.
    "lsp": {
        # Icons used by the auxiliary bar when LSP diagnostics are displayed.
        "icons": {
            "error":       "✖",
            "warning":     "▲",
            "information": "◉",
            "hint":        "󰌵",
        },
    },
    "shell": {
        "max_history": 2000,
    },
    # Toggle workspace transition animations.
    #
    # Both default to True. Setting either to False suppresses the
    # corresponding animation; everything else (init/shutdown
    # shaders, the :shaderrun command, ...) is unaffected.
    #
    # The open animation is only ever played after a loading
    # animation, so disabling "loading_workspace" effectively
    # disables both.
    "animations": {
        # Plays while a workspace is being installed
        # (addWorkspace). The default gray-fade desaturates the
        # screen to signal the IDE is busy.
        "loading_workspace": True,
        # Plays when a workspace finishes installing. The default
        # is a quick burn sweep that reveals the new content.
        "open_workspace":    True,
    },
    # Self-upgrade configuration. Rune polls a public manifest endpoint
    # to discover new releases and prompts before installing them.
    "upgrade": {
        # When True, Rune periodically checks for a newer release and
        # prompts to install it. Set to False to opt out — the
        # `:upgrade` command continues to work either way.
        "auto_check_enabled": True,
        # How often the background check runs. Accepts any Go duration
        # string (e.g. "12h", "168h").
        "check_period":       "24h",
        # Release channel. Reserved for future use; only "stable" is
        # currently honoured.
        "channel":            "stable",
    },
    "gui": {
        # Add or remove pixels from the font's default column width.
        "column-width-offset": -1,
        # Add or remove pixels from the font's default line height.
        "line-height-offset":   0,
        # Pick a theme by name; see the `guitheme` command for the full list.
        "default_theme":        "romero",
        # Enables or disables transparent-window rendering, effectively
        # enabling or disabling the guiopacity command.
        "enable_transparent_window": True,
        # Initial window opacity when transparent rendering is enabled.
        "window_opacity":       {"fg": 1, "bg": 1},
        # Initial background blur when transparent rendering is enabled and
        # the background opacity is not 1.
        "window_blur_radius":   100,
        # Enable or disable font ligatures for sequences like => and ->.
        "ligatures":            False,
        # Themes available to `default_theme` and the `guitheme` command.
        "themes":               GUI_THEMES,
    },
    "editor": {
        # 'modal' simulates vi/vim, 'modeless' works like conventional text
        # editors, and 'byoe' (bring-your-own-editor) hands editing off to an
        # external TUI editor process running inside a Rune-managed vte. Rune
        # keeps owning the tab, the on-disk file, and IDE-wide commands; the
        # external editor owns the buffer contents and the cursor.
        "mode":       "modal",
        # Enable or disable syntax-driven indentation.
        "autoindent": True,
        # Auto-pair quotes, brackets, and braces while editing. Explicitly off
        # for modal mode by default; the modeless override below enables it.
        "auto_pair":  False,
        # Auto-save dirty buffers after a brief idle period. Off by default;
        # set to True to flush file tabs ~2s after the last edit.
        "auto_save":  False,
        "tabspaces": 4,
        "modal": {
            # Default text attributes.
            "attr":        attr(fg = "default", bg = "default"),
            # Search and message bar attributes.
            "bar_attr":    attr(fg = "default", bg = "default"),
            # Search result attributes.
            "search_attr": attr(fg = "grey", bg = "yellow"),
        },
        # External editor configuration, only consulted when mode == "byoe".
        #
        # `command` is the argv template Rune executes inside a vte to open a
        # file. Available substitutions:
        #   {file}  absolute path to the file to open (always required)
        #   {line}  1-based line number for the initial cursor
        #   {col}   1-based column number for the initial cursor
        #
        # `goto` is a Rune key sequence (handler.ParseSequence syntax) that,
        # when injected into the vte, makes the editor place its cursor at
        # {line}/{col}. The same {line}/{col} substitutions are recognised
        # inside the sequence string. Leave empty to disable :goto / mouse
        # click-to-line for this editor.
        #
        # Examples (uncomment one or write your own):
        #
        # Vim / Neovim
        #   "command": 'vim "+call cursor({line}, {col})" {file}',
        #   "goto":    "<esc>:{line}<enter>{col}|",
        #
        # Neovim
        #   "command": 'nvim "+call cursor({line}, {col})" {file}',
        #   "goto":    "<esc>:{line}<enter>{col}|",
        #
        # Helix
        #   "command": "hx {file}:{line}:{col}",
        #   "goto":    "<esc>:goto<space>{line}<enter>",
        #
        # Kakoune
        #   "command": "kak {file} +{line}:{col}",
        #   "goto":    "<esc>:edit<space>-existing<space>{file}<space>{line}<space>{col}<enter>",
        #
        # Micro
        #   "command": "micro {file}:{line}:{col}",
        #   "goto":    "<c-l>{line}:{col}<enter>",
        #
        # Emacs (no window system)
        #   "command": "emacs -nw +{line}:{col} {file}",
        #   "goto":    "<a-x>goto-line<enter>{line}<enter>",
        #
        # Nano
        #   "command": "nano +{line},{col} {file}",
        #   "goto":    "<c-_>{line},{col}<enter>",
        "byoe": {
            "command": 'vim "+call cursor({line}, {col})" {file}',
            "goto":    "<esc>:{line}<enter>{col}|",
            # Rune-native editor used to serve URIs the external editor
            # cannot meaningfully edit (memory:// pseudo-URIs such as
            # the file explorer's tab). Valid values are "modal" or
            # "modeless".
            "fallback": "modal",
        },
        "indents": {
            "chatito": "spaces",
            "nim": "spaces",
            "yaml": "spaces",
        },
        # Used to determine the max columns to allow in text wrapping operations
        "ruler": 90,
        # Syntax highlight overrides. Any omitted key inherits the default
        # highlight attributes provided by the active language/parser.
        "highlights": {
            "function":             attr(fg = "default"),
            "function.builtin":     attr(fg = "yellow"),
            "function.method":      attr(fg = "default"),
            "type":                 attr(fg = "default"),
            "property":             attr(fg = "default"),
            "variable":             attr(fg = "default"),
            "operator":             attr(fg = "default"),
            "keyword":              attr(fg = "yellow"),
            "string":               attr(fg = "magenta"),
            "escape":               attr(fg = "default"),
            "number":               attr(fg = "red"),
            "constant.builtin":     attr(fg = "default"),
            "comment":              attr(fg = "blue"),
            "markup.heading":       attr(fg = "yellow", flags = "bold"),
            "markup.raw":           attr(fg = "magenta"),
            "markup.raw.delimiter": attr(fg = "yellow"),
            "markup.link":          attr(fg = "cyan", flags = "underline"),
            "markup.link.url":      attr(fg = "cyan", flags = "underline"),
            "markup.link.label":    attr(fg = "yellow", flags = "underline"),
            "markup.link.text":     attr(fg = "cyan", flags = "underline"),
            "markup.list":          attr(fg = "yellow", flags = "bold"),
            "markup.quote":         attr(fg = "blue", flags = "italic"),
            "markup.strong":        attr(fg = "yellow", flags = "bold"),
            "markup.italic":        attr(fg = "yellow", flags = "italic"),
            "markup.strikethrough": attr(fg = "blue", flags = "strikethrough"),
            "markup.underline":     attr(fg = "yellow", flags = "underline"),
            "markup.math":          attr(fg = "magenta"),
            "text.title":           attr(fg = "yellow", flags = "bold"),
            "text.literal":         attr(fg = "magenta"),
            "text.uri":             attr(fg = "cyan", flags = "underline"),
            "text.reference":       attr(fg = "yellow", flags = "underline"),
            "text.emphasis":        attr(fg = "yellow", flags = "italic"),
            "text.strong":          attr(fg = "yellow", flags = "bold"),
        },
        # status_bar configures the editor's bottom status bar.
        "status_bar": {
            # Whether the editor renders the status bar at all.
            "enabled": True,
            # Layout of the status bar. Available components include:
            # Status, Filepath, GitShortRef, GitDiffAdd, GitDiffDel,
            # ShiftRight, CursorColumn, CursorLine, TotalLines, and Language.
            # Pipe operators support bg, fg, bold, italic, underline,
            # reverse, and dim.
            "layout": ' {{ .Status | bg "red" | fg "white" | bold }} █▓▒░  {{ .Filepath | fg "white" }}  {{ .GitShortRef | fg "white" }}   {{ .GitDiffAdd | fg "green" }}   {{ .GitDiffDel | fg "red" }} {{ .ShiftRight }} {{ .CursorColumn | fg "white" }}:{{ .CursorLine | fg "white" }}  {{ .TotalLines | fg "white" }} lines  ░▒▓█ {{ .Language | bold | fg "white" | bg "red"}} ',
            # Default background attributes when the layout does not override
            # them explicitly.
            "background_attr":         attr(bg = "gray"),
            # Error color used for status-bar messages.
            "foreground_error_attr":   attr(fg = "yellow"),
        },
        # aux_bar configures the vertical bar shown to the left of file tabs.
        "aux_bar": {
            # Whether the editor renders the auxiliary bar at all.
            "enabled":           True,
            # Enable or disable rendering fold marks.
            "folds":             True,
            # Git mode: bar | inline | all | disabled.
            "git":               "inline",
            "icons":             True,
            # Line-number mode: disabled | absolute | relative.
            "lines":             "absolute",
            # Whether to highlight the cursor position in the bar.
            "highlight_cursor":  True,
            "git_del_inline_attr":    attr(fg = "maroon", flags = "bold"),
            "git_add_inline_attr":    attr(fg = "green", flags = "bold"),
            "git_del_locations_attr": attr(bg = "maroon", flags = "dim"),
            "git_add_locations_attr": attr(bg = "green", flags = "dim"),
            "line_number_attr":       attr(fg = "gray", bg = "default"),
            "highlight_cursor_attr":  attr(fg = "white", bg = "gray", flags = "bold"),
        },
        # Whether certain language-specific folds start hidden when a file is
        # opened.
        "initial_folds": True,
        # file_explorer configures the :fexplorer tree view. Indent
        # and icon attributes default to gray so the guides and glyphs
        # recede visually behind file names.
        "file_explorer": {
            "indent_attr": attr(fg = "gray"),
            "icon_attr":   attr(fg = "gray"),
        },
    },
    # Built-in extension configuration. Keys are identifiers only.
    "extensions": {
        # Adds the `colorPalette` command, which shows the colors available
        # for use throughout this configuration.
        "color_palette": {},
        # Adds fuzzy file and line search to the editor.
        "fuzzy_search": {
            "path": "extension_fuzzy_search",
            "config": {
                "file": {
                    # Whether search is case sensitive.
                    "case_sensitive": True,
                    # Search algorithm: fuzzy | contains | equal.
                    "algo":           "fuzzy",
                    # Key binding used to cycle through history.
                    "history_key":    "<m-p>",
                    # Maximum history stored.
                    "history":        50000,
                    "element_attr":        attr(fg = "default", bg = "default", flags = ["dim"]),
                    "matched_text_attr":   attr(fg = "blue", bg = "default", flags = ["bold"]),
                    "count_attr":          attr(fg = "default", bg = "default"),
                    "focus_element_attr":  attr(fg = "purple", bg = "default"),
                },
                "line": {
                    # Whether search is case sensitive.
                    "case_sensitive": True,
                    # Search algorithm: fuzzy | contains | equal.
                    "algo":           "fuzzy",
                    "history_key":    "<m-\\\\>",
                    # Maximum history stored.
                    "history":        2000,
                    "element_attr":        attr(fg = "default", bg = "default", flags = ["dim"]),
                    "matched_text_attr":   attr(fg = "blue", bg = "default", flags = ["bold"]),
                    "count_attr":          attr(fg = "default", bg = "default"),
                    "focus_element_attr":  attr(fg = "purple", bg = "default"),
                },
                "syntax": {
                    "case_sensitive": True,
                    "algo":           "fuzzy",
                },
            },
        },
    },
    # Configuration for the command prompt.
    "command": {
        # Key that opens the command prompt.
        "key":          ":",
        # Key that toggles between command list and history in the prompt.
        "history_key":  "<m-r>",
        # Maximum number of commands stored in history.
        "max_history":  20000,
        # Aliases layer on top of the built-ins.
        "aliases":      {
            "e":              {"command": "edit", "completer": "files"},
            "w":              "write",
            "sed":            "!! gsed -i $1 %",
            "gitnextchange":  "jumptolocation next gitchange",
            "gitprevchange":  "jumptolocation previous gitchange",
            "lspnextdiagnostic": "jumptolocation next lsp-diagnostics",
            "lspprevdiagnostic": "jumptolocation previous lsp-diagnostics",
            # locationpicker re-shell-quotes each token before
            # handing the joined command to $SHELL -c, so alias
            # bodies do not need to add extra single/double
            # quoting for values that survive Layer 1 prompt
            # tokenisation as a single token (no embedded
            # whitespace).
            "grep":           "locationpicker grep -n -R $1",
            "todogrep":       "locationpicker grep -n -R -E (TODO|FIXME)",
            "conflicts":      "locationpicker git grep -n --column '^<<<<<<<\\|^=======$\\|^>>>>>>>'",
            "gitgrep":        "locationpicker git grep -n --column -- $1",
            # gitchanges: present every changed hunk (working tree
            # vs HEAD) as a `path:line:1:hunk` location. The awk
            # script reads `git diff -U0` from a sub-pipe and
            # extracts the post-image start line of each `@@` hunk
            # header. -U0 keeps each hunk anchored to a single
            # changed line so the picker entry points directly at
            # the edit. Single-quoting protects the awk body from
            # Layer 1 prompt tokenisation; locationpicker then
            # re-shell-quotes it so the whole script reaches awk
            # as one argv element.
            "gitchanges":     "locationpicker awk 'BEGIN { cmd = \"git diff HEAD --no-color -U0\"; while ((cmd | getline line) > 0) { if (substr(line, 1, 6) == \"+++ b/\") f = substr(line, 7); else if (substr(line, 1, 2) == \"@@\") { match(line, /[+][0-9]+/); print f \":\" substr(line, RSTART+1, RLENGTH-1) \":1:\" line } } }'",
            "gitblame":       "! git blame %",
            "gitdiff":        "! git diff",
            "shadercancel":   "shaderrun nop 500ms",
            "tabnew":         "echo {prompt}edit<space>",
            "tabsearch":      "echo {prompt}tabfocus<space>",
            "workspacesearch":"echo {prompt}workspacefocus<space>",
            "searchfunc":     "echo {prompt}searchast<space>locals.scm<space>local.definition.method|local.definition.function<enter>",
            "searchvar":      "echo {prompt}searchast<space>locals.scm<space>local.definition.var<enter>",
            "searchtype":     "echo {prompt}searchast<space>locals.scm<space>local.definition.type<enter>",
            "worktreenew": [
                '!! git worktree add "$RUNE_DATADIR/worktrees/$1" -b $1',
                'workspacenew $RUNE_DATADIR/worktrees/$1',
                'workspaceready workspacerename $1',
            ],
            "worktreeopen": {
                "command": [
                    "workspacenew $RUNE_DATADIR/worktrees/$1",
                    "workspaceready workspacerename $1",
                    "workspaceready shaderrun shine 600ms",
                ],
                "completer": "! $SHELL -c 'git worktree list | tail -n +2 | cut -d\" \" -f1 | xargs -n1 basename'",
            },
            "worktreeremove": {
                "command": [
                    "!! git worktree remove $1",
                    "shaderrun embers 600ms",
                ],
                "completer": "! $SHELL -c 'git worktree list | tail -n +2 | cut -d\" \" -f1 | xargs -n1 basename'",
            },
            # Open a workspace and play the shine shader as a visual
            # confirmation that the new workspace was opened.
            "wopen": {
                "command": "workspacenew $1",
                "completer": [
                    "{history}",
                    "{dirs}",
                ],
            },
        },
        # Key bindings merge with the built-ins; set a value to "" to unbind.
        "key_bindings": {
            "<shift-tab>":    "fexplorer",
            "<m-r>":          "history",
            "<m-,>":          "config",
            "<m-q>":          "quit",
            "<m-=>":          "guifontsize increase",
            "<m-->":          "guifontsize decrease",
            "<m-t>":          "tabnew",
            "<alt-enter>":    "echo {prompt}windowconverttab<space>",
            "<a-w>":          "tabclose",
            "<a-l>":          "tabnext",
            "<a-h>":          "tabprevious",
            "<m-n>":          "windownew",
            "<m-c>":          "clipboardcopy",
            "<m-v>":          "clipboardpaste",
            "<a-`>":          "tabsearch",
            "<a-s-l>":        "tabmove right",
            "<a-s-h>":        "tabmove left",
            "<a-s-f>":        "searchfunc",
            "<a-s-v>":        "searchvar",
            "<a-s-s>":        "searchtype",
            "<m-h>":          "windowfocus left",
            "<m-l>":          "windowfocus right",
            "<m-j>":          "windowfocus down",
            "<m-k>":          "windowfocus up",
            "<s-m-h>":        "windowmove left",
            "<s-m-l>":        "windowmove right",
            "<s-m-j>":        "windowmove down",
            "<s-m-k>":        "windowmove up",
            "<s-m-backspace>":"windowresize reset",
            "<s-m-+>":        ["windowresize max width", "windowresize max height"],
            "<s-m-->":        ["windowresize min width", "windowresize min height"],
            "<s-m-up>":       "windowresize increase height",
            "<s-m-down>":     "windowresize decrease height",
            "<s-m-left>":     "windowresize decrease width",
            "<s-m-right>":    "windowresize increase width",
            "<m-w>":          "windowclose",
            "<c-m-h>":        "windowdefaultsplit h",
            "<c-m-v>":        "windowdefaultsplit v",
            "<s-m-f>":        "windowtogglemaximize",
            "<a-f>":          "echo {prompt}jumptoast<space>locals.scm<space>local.definition.method|local.definition.function<space>",
            "<a-v>":          "echo {prompt}jumptoast<space>locals.scm<space>local.definition.var<space>",
            "<a-s>":          "echo {prompt}jumptoast<space>locals.scm<space>local.definition.type<space>",
            "<a-j>":          "lspnextdiagnostic",
            "<a-k>":          "lspprevdiagnostic",
            "<a-t>":          "lsp hover",
            "<a-s-t>":        "echo {prompt}lsp<space>hover<space>",
            "<a-s-e>":        "lsp diagnostics",
            "<m-f>":          "lsp rename",
            "<a-r>":          "lsp references",
            "<a-s-r>":        "echo {prompt}lsp<space>references<space>",
            "<a-d>":          "lsp definition",
            "<a-s-d>":        "echo {prompt}lsp<space>definition<space>",
            "<a-i>":          "lsp implementation",
            "<a-s-i>":        "echo {prompt}lsp<space>implementation<space>",
            "<c-o>":          "cursorhistory prev",
            "<c-i>":          "cursorhistory next",
            "<a-b>":          "lsp format",
            # on MacOS <c-space> is a shortcut. To disable it and enable this
            # key binding, go to System Settings → Keyboard → Keyboard Shortcuts
            # → Input Sources → uncheck both entries.
            "<c-space>":      "lsp complete",
            "<a-m>":          "go organize-imports",
            "<a-s-j>":        "gitnextchange",
            "<a-s-k>":        "gitprevchange",
            "<m-p>":          "searchfile",
            "<m-\\\\>":      "searchtext",
            "<m-enter>":      "terminalneworsplit",
            "<s-m-enter>":    "!",
            "<m-`>":          "workspacesearch",
            "<a-1>":          "tabfocus 1",
            "<a-2>":          "tabfocus 2",
            "<a-3>":          "tabfocus 3",
            "<a-4>":          "tabfocus 4",
            "<a-5>":          "tabfocus 5",
            "<a-6>":          "tabfocus 6",
            "<a-7>":          "tabfocus 7",
            "<a-8>":          "tabfocus 8",
            "<a-9>":          "tabfocus 9",
            "<a-s-1>":        "tabmove 1",
            "<a-s-2>":        "tabmove 2",
            "<a-s-3>":        "tabmove 3",
            "<a-s-4>":        "tabmove 4",
            "<a-s-5>":        "tabmove 5",
            "<a-s-6>":        "tabmove 6",
            "<a-s-7>":        "tabmove 7",
            "<a-s-8>":        "tabmove 8",
            "<a-s-9>":        "tabmove 9",
            "<m-1>":          "workspacefocus 1",
            "<m-2>":          "workspacefocus 2",
            "<m-3>":          "workspacefocus 3",
            "<m-4>":          "workspacefocus 4",
            "<m-5>":          "workspacefocus 5",
            "<m-6>":          "workspacefocus 6",
            "<m-7>":          "workspacefocus 7",
            "<m-8>":          "workspacefocus 8",
            "<m-9>":          "workspacefocus 9",
            "<s-m-1>":        "workspacemove 1",
            "<s-m-2>":        "workspacemove 2",
            "<s-m-3>":        "workspacemove 3",
            "<s-m-4>":        "workspacemove 4",
            "<s-m-5>":        "workspacemove 5",
            "<s-m-6>":        "workspacemove 6",
            "<s-m-7>":        "workspacemove 7",
            "<s-m-8>":        "workspacemove 8",
            "<s-m-9>":        "workspacemove 9",
        },
        # Prompt colors.
        "element_attr":       attr(fg = "default", bg = "default", flags = ["dim"]),
        "manual_attr":        attr(fg = "default", bg = "default"),
        "matched_text_attr":  attr(fg = "blue", bg = "default", flags = ["bold"]),
        "focus_element_attr": attr(fg = "purple", bg = "default"),
    },
    "browser": {
        # What the workspace bar shows: the simplified path ("path"), the
        # workspace number ("number"), or false to disable the bar entirely.
        "workspace_bar": "path",
        "window_manager": {
            "dim":                   True,
            "frame":                 True,
            "frame_attr":            attr(fg = "gray", bg = "default"),
            "focus_frame_attr":      attr(fg = "silver", bg = "default"),
            "frame_charset":         GUI_FRAME_CHARSET,
            "focus_frame_charset":   GUI_FRAME_CHARSET,
            "scroll_bar_attr":       attr(fg = "blue", bg = "default"),
            "scroll_bar_char":       "▐",
            "scroll_bar_hover_char": "█",
        },
        "union_frames":       False,
        "frameunion_charset": {
            "left":   "🭽",
            "right":  "🭾",
            "top":    "🭿",
            "bottom": "🭼",
        },
        # Separator characters used to space out tab names.
        "focus_tab_attr":          attr(fg = "silver", bg = "default"),
        "dirty_tab_attr":          attr(fg = "red", bg = "default", flags = ["italic"]),
        "non_focus_tab_attr":      attr(fg = "gray", bg = "default"),
        "focus_tab_icon_attr":     attr(fg = "yellow", bg = "default"),
        "non_focus_tab_icon_attr": attr(fg = "silver", bg = "default"),
        "focus_tab_highlight_attr": attr(fg = "red", bg = "default"),
        "focus_tab_highlight_char":  "\U00100006",
        # Icons for tabs; `terminal` is used for terminal tabs while
        # `default` is the fallback for files.
        "icons": {
            "default":  "",
            "terminal": "",
            "directory": "",
            "open_directory": "",
        },
        # If set, every tab icon rendered by the browser is forced
        # to this single rune, ignoring per-source icons (file
        # glyphs, `browser.icons`, terminal/shell icons, etc.).
        # Empty disables.
        "tab_override_icon": "",
        "tab_name_separator": "   ",
        # Prompt configuration used when Rune asks the user questions.
        "prompt": {
            "text_attr":       attr(bg = "gray", flag = "bold"),
            "highlight_attr":  attr(bg = "red", flag = "bold"),
            "background_attr": attr(fg = "default", bg = "default"),
        },
    },
    # Workspace configuration. This configuration is never reloaded.
    "workspace": {
        # Configuration for remote workspaces connected over SSH.
        "ssh":          {
            "timeout":         "5s",
            "skip_preflight":  True,
        },
        # Automatically restore the previous session's files, terminals, and
        # window layout.
        "auto_restore": True,
        # Attributes for custom ASCII or image wallpapers.
        "wallpaper_attr":            attr(fg = "blue", bg = "default"),
        "wallpaper_background_attr": attr(bg = "default"),
        # Character drawn on the bottom row of the workspace tab bar to
        # highlight the workspace currently in focus.
        "focus_tab_highlight_char":  "",
    },
    # Notification pop-up configuration.
    "notifications": {
        "auto_close":   "5s",
        "padding":      1,
        "progress_bar": True,
        "progress_format": {
            "start":       "\U00100000",
            "current":     "\U00100001",
            "current_tip": "\U00100002",
            "remain":      "\U00100003",
            "end":         "\U00100004",
        },
        "attr":            attr(fg = "default", bg = "default"),
        "background_attr": attr(fg = "default", bg = "default"),
        "frame_charset":   GUI_FRAME_CHARSET,
    },
    "terminal": {
        # Whether to respect the title set by the shell.
        "dynamic_tab_name":   True,
        # Enable terminal modal mode on <esc>.
        "modal":              True,
        # Maximum emulator lines. Increasing this slows resizing linearly.
        "max_lines":          1000,
        # Number of pre-initialized emulator instances kept in the reservoir.
        # A value of 0 disables the reservoir.
        "initial_reservoir":  0,
        "attr":               attr(fg = "default", bg = "default"),
        "selection_attr":     attr(flags = "reverse"),
        # Tab attributes used when a bell arrives while the window is unfocused.
        "needs_attention_attr": attr(fg = "red", flags = "blink"),
        # Configuration for programs executed via the `!` command.
        "plugin": {
            # Layout of the plugin status bar. Available components include:
            # Command, StatusIcon, ExitStatus, Elapsed, AlignRight, AlignCenter.
            # Pipe operators support bg, fg, bold, italic, underline, reverse,
            # and dim.
            "bar_layout": ' {{ .StatusIcon | bg "gray" | fg "white" }} █▓▒░{{ .AlignCenter}}{{ .Command | fg "white" | bold }}{{ .AlignRight }}  ░▒▓█ {{ .Elapsed | fg "white" | bg "gray" }} ',
            "status_error_icon":   "",
            "status_error_attr":   attr(fg = "red"),
            "status_success_icon": "",
            "status_success_attr": attr(fg = "green"),
            "animation": "⠃⠅⠆⠘⠨⠰⠉⠒⠤⠑⠡⠢⠊⠌⠔⠇⠸⠎⠱⠣⠜⠪⠕⠋⠙⠓⠚⠍⠩⠥⠬⠖⠲⠦⠴⠏⠹⠧⠼⠫⠝⠮⠵⠺⠗⠞⠳⠛⠭⠶⠟⠻⠷⠾⠯⠽⠿",
            "bar_background_attr": attr(bg = "default"),
            # Whether the plugin bar is rendered at the bottom instead of top.
            "bar_align_bottom":    False,
        },
    },
}

if mode == "modeless":
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
                "layout": '██▓▒░  {{ .Filepath }}  {{ .GitShortRef }}   {{ .GitDiffAdd | fg "green" }}   {{ .GitDiffDel | fg "red" }} {{ .ShiftRight }} {{ .CursorColumn }}:{{ .CursorLine }}  {{ .TotalLines }} lines  {{ .Language | bold }}  ░▒▓██',
            },
        },
        "command": {
            "key":          "<s-m-p>",
            "key_bindings": {
                # These blank commands intentionally shadow the modal defaults to
                # preserve the legacy runerc.modeless key map exactly.
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

if tui:
    tui_cmd_bindings = {
        "<c-w>":          "tabclose",
        "<c-l>":          "tabnext",
        "<c-h>":          "tabprevious",
        "<c-x><c-v>":     "clipboardpaste",
        "<c-x><c-c>":     "clipboardcopy",
        "<c-x><c-h>":     "windowfocus left",
        "<c-x><c-l>":     "windowfocus right",
        "<c-x><c-j>":     "windowfocus down",
        "<c-x><c-k>":     "windowfocus up",
        "<c-x><c-w>":     "windowclose",
        "<c-x>h":         "windowdefaultsplit h",
        "<c-x>v":         "windowdefaultsplit v",
        "<c-j>":          "lspnextdiagnostic",
        "<c-k>":          "lspprefdiagnostic",
        "<c-x><c-f>":     "windowtogglemaximize",
        "<c-x><c-t>":     "lsphover",
        "<c-x><c-e>":     "lspref",
        "<c-x><c-g>":     "lspgotodef",
        "<c-x><c-b>":     "lspformat",
        "<c-x><c-p>":     "searchfile",
        "<c-x><c-\\>":    "searchtext",
        "<c-x><enter>":   "terminalneworsplit",
        "gf":             "editfileoncursor",
        "<c-x>1":         "workspacefocus 1",
        "<c-x>2":         "workspacefocus 2",
        "<c-x>3":         "workspacefocus 3",
        "<c-x>4":         "workspacefocus 4",
        "<c-x>5":         "workspacefocus 5",
        "<c-x>6":         "workspacefocus 6",
        "<c-x>7":         "workspacefocus 7",
        "<c-x>8":         "workspacefocus 8",
        "<c-x>9":         "workspacefocus 9",
    }

    config = merge(config, {
        "default_attr": attr(fg = "default", bg = "#1e1e1e"),
            "log_path":  "~/.rune/debug.log",
        "log_level": "info",
        "input_mode": ["esc", "mouse"],
        "editor": {
            "modal": {
                "attr":        attr(fg = "default", bg = "#1e1e1e"),
                "bar_attr":    attr(fg = "default", bg = "#1e1e1e"),
                "search_attr": attr(fg = "default", bg = "#1e1e1e", flags = "reverse"),
            },
            "modeless": {
                "attr":        attr(fg = "default", bg = "#1e1e1e"),
                "bar_attr":    attr(fg = "default", bg = "#1e1e1e"),
                "search_attr": attr(fg = "default", bg = "#1e1e1e", flags = "reverse"),
            },
            "aux_bar": {
                "git_del_inline_attr":    attr(fg = "maroon", flags = "bold"),
                "git_add_inline_attr":    attr(fg = "green", flags = "bold"),
                "git_del_locations_attr": attr(bg = "maroon", flags = "dim"),
                "git_add_locations_attr": attr(bg = "green", flags = "dim"),
                "line_number_attr":       attr(fg = "gray", bg = "default"),
                "highlight_cursor_attr":  attr(fg = "white", bg = "gray", flags = "bold"),
            },
            "status_bar": {"background_attr": attr(bg = "gray")},
        },
        "extensions": {
            "fuzzy_search": {
                "path": "extension_fuzzy_search",
                "config": {
                    "file": {
                        "history_key":         "<c-p>",
                        "element_attr":        attr(fg = "default", bg = "#1e1e1e"),
                        "matched_text_attr":   attr(fg = "#D34728", bg = "#1e1e1e", flags = ["bold"]),
                        "count_attr":          attr(fg = "default", bg = "#1e1e1e"),
                        "focus_element_attr":  attr(fg = "#c6c6c6", bg = "#1e1e1e", flags = ["bold"]),
                    },
                    "line": {
                        "history_key":         "<c-\\\\>",
                        "element_attr":        attr(fg = "default", bg = "#1e1e1e"),
                        "matched_text_attr":   attr(fg = "#D34728", bg = "#1e1e1e", flags = ["bold", "italic"]),
                        "count_attr":          attr(fg = "default", bg = "#1e1e1e"),
                        "focus_element_attr":  attr(fg = "#c6c6c6", bg = "#1e1e1e", flags = ["bold"]),
                    },
                },
            },
        },
        "command": {
            "key_bindings": {
                "<c-w>":          "tabclose",
                "<c-l>":          "tabnext",
                "<c-h>":          "tabprevious",
                "<c-x><c-v>":     "clipboardpaste",
                "<c-x><c-c>":     "clipboardcopy",
                "<c-x><c-h>":     "windowfocus left",
                "<c-x><c-l>":     "windowfocus right",
                "<c-x><c-j>":     "windowfocus down",
                "<c-x><c-k>":     "windowfocus up",
                "<c-x><c-w>":     "windowclose",
                "<c-x>h":         "windowdefaultsplit h",
                "<c-x>v":         "windowdefaultsplit v",
                "<c-j>":          "lspnextdiagnostic",
                "<c-k>":          "lspprefdiagnostic",
                "<c-x><c-f>":     "windowtogglemaximize",
                "<c-x><c-t>":     "lsphover",
                "<c-x><c-e>":     "lspref",
                "<c-x><c-g>":     "lspgotodef",
                "<c-x><c-b>":     "lspformat",
                "<c-x><c-p>":     "searchfile",
                "<c-x><c-\\\\>": "searchtext",
                "<c-x><enter>":   "terminalneworsplit",
                "gf":             "editfileoncursor",
                "<c-x>1":         "workspacefocus 1",
                "<c-x>2":         "workspacefocus 2",
                "<c-x>3":         "workspacefocus 3",
                "<c-x>4":         "workspacefocus 4",
                "<c-x>5":         "workspacefocus 5",
                "<c-x>6":         "workspacefocus 6",
                "<c-x>7":         "workspacefocus 7",
                "<c-x>8":         "workspacefocus 8",
                "<c-x>9":         "workspacefocus 9",
            },
            "element_attr":      attr(fg = "default", bg = "#1e1e1e"),
            "manual_attr":       attr(fg = "default", bg = "#1e1e1e"),
            "matched_text_attr": attr(fg = "#D34728", bg = "#1e1e1e", flags = ["bold", "italic"]),
            "focus_element_attr":attr(fg = "#c6c6c6", bg = "#1e1e1e", flags = ["bold"]),
        },
        "browser": {
            "union_frames": True,
            "window_manager": {
                "dim":                   True,
                "frame":                 True,
                "frame_attr":            attr(fg = "#3a3a3a", bg = "#1e1e1e"),
                "focus_frame_attr":      attr(fg = "#c6c6c6", bg = "#1e1e1e", flags = "bold"),
                "frame_charset":         TUI_FRAME_CHARSET,
                "focus_frame_charset":   TUI_FRAME_CHARSET,
                "scroll_bar_attr":       attr(fg = "#c6c6c6", bg = "#1e1e1e"),
                "scroll_bar_char":       "┃",
                "scroll_bar_hover_char": "║",
            },
            "tab_name_separator":  "  ",
            "frameunion_charset":  {
                "left":   "├",
                "right":  "┤",
                "top":    "┬",
                "bottom": "┴",
            },
            "focus_tab_attr":          attr(fg = "#c6c6c6", bg = "#1e1e1e", flags = ["bold"]),
            "dirty_tab_attr":          attr(fg = "#D34728", bg = "#1e1e1e", flags = ["italic"]),
            "non_focus_tab_attr":      attr(fg = "gray", bg = "#1e1e1e"),
            "focus_tab_icon_attr":     attr(fg = "#c6c6c6", bg = "#1e1e1e", flags = ["bold"]),
            "non_focus_tab_icon_attr": attr(fg = "gray", bg = "#1e1e1e"),
            "focus_tab_highlight_attr": attr(fg = "yellow", bg = "#1e1e1e"),
            "focus_tab_highlight_char": "━",
            "prompt": {
                "text_attr":       attr(fg = "default", bg = "#1e1e1e"),
                "highlight_attr":  attr(fg = "#c6c6c6", bg = "default", flag = "reverse"),
                "background_attr": attr(fg = "default", bg = "#1e1e1e"),
            },
        },
        "workspace": {
            "wallpaper_attr":            attr(fg = "#D34728", bg = "#1e1e1e"),
            "wallpaper_background_attr": attr(bg = "#1e1e1e"),
            "focus_tab_highlight_char":  "━",
        },
        "notifications": {
            "attr":            attr(fg = "default", bg = "#1e1e1e"),
            "background_attr": attr(fg = "default", bg = "#1e1e1e"),
            "frame_charset":   TUI_FRAME_CHARSET,
        },
        "terminal": {
            "attr":                  attr(fg = "default", bg = "#1e1e1e"),
            "selection_attr":        attr(flags = "reverse"),
            "needs_attention_attr":  attr(fg = "red", flags = "blink"),
        },
    })
