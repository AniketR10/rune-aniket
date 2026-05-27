# Minimal Starlark config used by ide tests.

aliases = {
    "w": "write",
    "gitblame": "! git blame $FILE",
    "worktreenew": [
        '!! ROOT=$(cd "$(git rev-parse --git-common-dir)/.." && pwd)',
        '!! ROOT_HASH=$(printf %s "$ROOT" | (sha256sum 2>/dev/null || shasum -a 256) | cut -c1-4)',
        '!! ROOT_NAME=$(basename "$ROOT" | tr -c "A-Za-z0-9.\\-_" "_")',
        '!! WORKTREE=$RUNE_DATADIR/worktrees/$ROOT_NAME-$ROOT_HASH/$1',
        '!! git worktree add "$WORKTREE" -b $1',
        "workspacenew $WORKTREE",
        "workspaceready workspacerename $1",
    ],
}

keybindings = {
    "<m-q>": "quit",
    "<m-t>": "tabnew",
}

# Demonstrates top-level control: a for-loop materialising per-workspace bindings.
for i in range(1, 10):
    keybindings["<a-%d>" % i] = "tabfocus %d" % i

config = {
    "log_path": "/tmp/debug.log",
    "log_level": "info",
    "clipboard": "system",
    "editor": {
        "mode": "modal",
        "autoindent": True,
        "highlights": {
            "keyword": {"fg": "yellow"},
            "string": {"fg": "magenta"},
        },
    },
    "browser": {
        "icons": {
            "default": "X",
            "terminal": "T",
        },
    },
    "gui": {
        "ligatures": False,
        "default_theme": "carmack",
        "themes": {
            "romero": {
                "red": "#990000",
                "background": "#000000",
            },
        },
    },
    "command": {
        "key": ":",
        "max_history": 20000,
        "aliases": aliases,
        "key_bindings": keybindings,
    },
}
