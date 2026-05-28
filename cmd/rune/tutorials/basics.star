# basics.star — the first-run tutorial.
#
# Walks the user through the three things they need to know to be
# productive in Rune: where to put work in the empty workspace,
# how to open a workspace (`:wopen`), and how to start editing
# files inside that workspace (`:edit`).
#
# The DSL is interpreted by ide/idetutorial/starlarktutorial. The
# entry function runs on its own Starlark goroutine; each blocking
# builtin (floating_window, wait_command, confirm) returns a real
# value so authors can branch, loop, and compose helpers with
# regular Starlark control flow.

ck = command_key()
if ck == "":
    ck = "<cmd-key>"

welcome_md = """\
# Welcome to Rune

This is the **home workspace**: a scratch workspace rooted at `~/` that
Rune shows when no project workspace is attached. Use it for files
outside any project, quick terminals, or to keep notes between
sessions.

## Switching workspaces

Rune has **nine workspace slots**. Press `<meta-1>` through
`<meta-9>` to jump between them. Every empty slot shows this same home
workspace; a slot only gets a project attached when you open one
inside it. So slot 1 may be the project you're working on while
slots 2-9 are still the home workspace, ready for whatever you
need.

This is handy when the current workspace is busy: jump to any
empty slot and press `<meta-enter>` to drop a terminal at `~/`
(that key runs the `terminalneworsplit` command). The slot fills
with a terminal without disturbing your project.

## Commands and key bindings

IDE-wide operations are exposed as **commands** that you invoke
from the command prompt. Keys like `<meta-1>` and `<meta-enter>`
are bound to those commands through your user configuration under
`command.key_bindings`, so every binding shown here is rebindable.

## Opening a workspace

1. Press `""" + ck + """` to open the command prompt.
2. Type `wopen` and use the auto-completer (Tab / arrow keys) to
   pick a workspace.
3. Press Enter to open it.

Press `<enter>` or `<space>` to continue.
"""

edit_md = """\
# Editing a file

You're in a real workspace now. To open a file:

1. Press `""" + ck + """` to open the command prompt.
2. Type `edit` followed by a partial filename.
3. Use the auto-completer to pick the file you want.
4. Press Enter.

Press `<enter>` or `<space>` to continue.
"""

def teach_edit():
    floating_window(title = "Open a file", text = edit_md)
    edit = wait_command(
        command  = "edit",
        on_error = ("`<cmd>edit` needs a `<file>` argument. Use the " +
                    "auto-completer (Tab / arrow keys) to pick a " +
                    "file, or type a path inside the workspace; " +
                    "relative paths resolve against the workspace root."),
    )
    notify(level = success, message = "You opened " + edit.args[0])


def run():
    floating_window(
        title = "Welcome",
        text = welcome_md,
        allow_keys = [
            "<meta-1>", "<meta-2>", "<meta-3>",
            "<meta-4>", "<meta-5>", "<meta-6>",
            "<meta-7>", "<meta-8>", "<meta-9>",
            "<meta-enter>",
        ],
        dismiss_keys = [ck],
    )

    ws = wait_command(
        command  = "wopen",
        on_error = ("`<cmd>wopen` needs a `<directory>` argument. " +
                    "Use the auto-completer (Tab / arrow keys) to " +
                    "pick a workspace, or type a directory path " +
                    "(it will be created if it doesn't exist)."),
    )
    notify(level = success, message = "Opened workspace: " + ws.args[0])

    if not confirm("Want to learn how to open a file next?"):
        notify(level = info,
               message = "Run `<cmd>tutorial run basics` any time to continue.")
        return

    teach_edit()


tutorial(id = "basics", title = "Rune basics", version = "2", entry = run)
