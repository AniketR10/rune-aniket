# navigation.star — the code-navigation tutorial.
#
# Teaches how to move around code in Rune: finding a file by name,
# finding where text lives, and the language-server verbs that jump to
# a definition, list references and implementations, show hover docs,
# and step back and forth through the cursor history (jumplist).
#
# The DSL is interpreted by ide/idetutorial/starlarktutorial. The entry
# function runs on its own Starlark goroutine; each blocking builtin
# (floating_window, wait_command, wait_event) returns a real value so
# authors can branch, loop, and compose helpers with regular Starlark
# control flow.

ck = command_key()

# Cursor-movement keys the teaching windows must let through so the user
# can position the cursor on a symbol before pressing the LSP binding.
# Arrow keys work in every mode; modal adds vim's home row and emacs adds
# the GNU-Emacs motion keys. Without these the overlay swallows the
# keypress and pulses its hint instead of moving the cursor.
arrow_keys = ["<up>", "<down>", "<left>", "<right>"]
if editor_mode() == "modal":
    move_keys = arrow_keys + ["h", "j", "k", "l", "w", "b", "e", "0", "$"]
elif editor_mode() == "emacs":
    move_keys = arrow_keys + ["<ctrl-p>", "<ctrl-n>", "<ctrl-f>", "<ctrl-b>",
                              "<ctrl-a>", "<ctrl-e>"]
else:
    move_keys = arrow_keys

# How to move through a location picker / finder list, phrased per mode.
# Modal points with `<ctrl-j>` / `<ctrl-k>`; emacs and standard use the
# arrow keys.
if editor_mode() == "modal":
    move_phrase = "`<ctrl-j>` / `<ctrl-k>` (or the arrow keys)"
else:
    move_phrase = "the arrow keys `<up>` / `<down>`"

# `<alt-shift-d>` opens the command prompt prefilled with `lsp definition `
# so the user only types the symbol name. That prefill binding ships in
# modal and standard mode; emacs has no equivalent, so fall back to the
# command-prompt wording there.
if editor_mode() == "emacs":
    def_by_name_cta = ("open the command prompt (`" + ck +
                       "`) and type `lsp definition `")
    def_by_name_dismiss = [ck]
else:
    def_by_name_cta = "press `<alt-shift-d>`"
    def_by_name_dismiss = [ck, "<alt-shift-d>"]

def keyhint(cmd, *args):
    k = key_for(cmd, *args)
    return (" Default key: `" + k + "`.") if k else ""

def keypress(cmd, *args):
    # The user's real bound key for cmd, phrased as a keypress
    # instruction. Falls back to the command-prompt wording when the
    # command is unbound, so copy adapts to mode and custom rebinds.
    k = key_for(cmd, *args)
    if k:
        return "press `" + k + "`"
    return "open the command prompt (`" + ck + "`) and run `" + cmd + "`"

def dismiss_for(cmd, *args):
    # Keys that dismiss a teaching window. Always include the command
    # prompt key. When the command has a bound key, include it too so
    # the single keypress the copy asks for dismisses the window AND
    # falls through to the IDE, dispatching the command that the
    # following wait_command observes.
    k = key_for(cmd, *args)
    return [ck, k] if k else [ck]

intro_md = """\
You already know how to open files and move windows. This tutorial is
about moving around **code**: jumping between files, symbols, and
definitions without reaching for the mouse.

We will answer these questions, one at a time:

- How do I find a file by name?
- How do I find where some text lives?
- How do I jump to where a symbol is defined?
- How do I navigate back and forth as I explore?
- What else can I ask about the symbol under my cursor?

Press `<enter>` or `<space>` to continue.
"""

searchfile_md = """\
**Find a file by name.** `searchfile` opens a fuzzy finder over every
file in the workspace. Type part of a name and it ranks matches as you
go, so `navistar` finds `navigation.star`.""" + keyhint("searchfile") + """

Open it now: """ + keypress("searchfile") + """. Type a few characters,
pick a file, and press `<enter>` to open it.
"""

searchfile_missing_md = """\
`searchfile` comes from the fuzzy-search extension, which is not
installed here. Install it from Rune's console with
`pkg install fuzzy-search`, then rerun this tutorial to try the finder.

Press `<enter>` or `<space>` to continue.
"""

searchtext_md = """\
**Find where text lives.** `searchtext` searches the *contents* of
every file in the workspace, not just names. Use it when you remember a
string, an error message, or a comment but not which file holds
it.""" + keyhint("searchtext") + """

Open it now: """ + keypress("searchtext") + """. Type something to
search for, then pick a result to jump straight to that line.
"""

searchtext_missing_md = """\
`searchtext` also comes from the fuzzy-search extension, which is not
installed here. Install it with `pkg install fuzzy-search` from Rune's
console, then rerun this tutorial.

Press `<enter>` or `<space>` to continue.
"""

lsp_intro_md = """\
Text search finds strings. To understand code, Rune talks to a
**language server**: the same engine your editor uses for autocomplete
and diagnostics. The cross-language `lsp` command exposes it, and it
knows what symbols mean, not just where their letters appear.

The next few steps use `lsp` to jump to definitions, list references,
and read documentation.

Press `<enter>` or `<space>` to continue.
"""

lsp_definition_cursor_md = """\
**Jump to the definition under your cursor.** This is the one you will
reach for most. Put your cursor on any symbol and `lsp definition` (no
argument) jumps straight to where that symbol is defined.""" + keyhint("lsp", "definition") + """

Put your cursor on a symbol, then """ + keypress("lsp", "definition") + """.
"""

lsp_definition_name_md = """\
**Find a definition by name.** You do not even need your cursor on a
symbol: `lsp definition <name>` fuzzy-matches the symbol by name across
the whole workspace and jumps to it. `lsp references` and `lsp hover`
take a name the same way.

Search for a definition now: """ + def_by_name_cta + """ and type a
symbol name. Cannot think of one? Peruse the completion list and press
`<enter>` on any entry.
"""

cursorhistory_md = """\
**Navigate back and forth.** Every jump you make (a definition jump, a
search result, an explorer open) is pushed onto the **cursor history**,
Rune's jumplist. You can walk it in both directions without retracing
your steps.

- `cursorhistory prev` goes back to where you were.""" + keyhint("cursorhistory", "prev") + """
- `cursorhistory next` goes forward again.""" + keyhint("cursorhistory", "next") + """

First, go back to where you jumped from: """ + keypress("cursorhistory", "prev") + """.

`cursorhistory jump` opens a picker over the whole history when you want
to leap several stops at once. `<alt-j>` / `<alt-k>` step through the
diagnostics in the current file the same way.
"""

cursorhistory_next_md = """\
**Now jump forward again.** Going back left a breadcrumb ahead of you.
`cursorhistory next` follows it forward to where you just were, so you
can retrace an exploration in either direction.""" + keyhint("cursorhistory", "next") + """

Go forward again: """ + keypress("cursorhistory", "next") + """.
"""

lsp_more_md = """\
**Ask more about the symbol under your cursor.** The other `lsp` verbs
work the same way: put your cursor on a symbol and run one.

- `lsp references` lists every place a symbol is used.""" + keyhint("lsp", "references") + """
- `lsp implementation` finds the concrete types that satisfy an interface (or the interfaces a type satisfies).""" + keyhint("lsp", "implementation") + """
- `lsp hover` shows the symbol's type and documentation inline.""" + keyhint("lsp", "hover") + """

Try `lsp references` now: put your cursor on a symbol and """ + keypress("lsp", "references") + """.
It opens the **location picker** — a list of every use of that symbol.
Move through the list with """ + move_phrase + """ and press `<enter>` on an
entry to jump to it.
"""

wrapup_md = """\
That is code navigation in Rune. A quick recap:

- **Find a file by name** with `searchfile`.
- **Find where text lives** with `searchtext`.
- **Jump to a definition** with `lsp definition` (under the cursor, or by name).
- **Navigate back and forth** with `cursorhistory prev` / `next`.
- **Ask about a symbol** with `lsp references`, `lsp implementation`, and `lsp hover`.

The `cheatsheet` command collects these and more, and the docs cover
the `lsp` verbs in depth. Run `tutorial start navigation` any time to
replay this walkthrough.

Press `<enter>` or `<space>` to continue.
"""

def teach_searchfile():
    if not key_for("searchfile"):
        floating_window(title = "Find a file by name", text = searchfile_missing_md,
                        dismiss_keys = [ck])
        return
    floating_window(title = "Find a file by name", text = searchfile_md,
                    dismiss_keys = dismiss_for("searchfile"))
    wait_command(
        title    = "Find a file by name",
        command  = "searchfile",
        on_error = "Open the file finder with `<cmd>searchfile` and pick a file.",
    )
    wait_event(
        event    = "open",
        title    = "Find a file by name",
        text     = "Pick a file from the finder and press `<enter>` to open it.",
        on_error = "Choose a file in the finder and press `<enter>` to open it.",
    )
    notify(level = success, message = "You found a file by name.")


def teach_searchtext():
    if not key_for("searchtext"):
        floating_window(title = "Find where text lives", text = searchtext_missing_md,
                        dismiss_keys = [ck])
        return
    floating_window(title = "Find where text lives", text = searchtext_md,
                    dismiss_keys = dismiss_for("searchtext"))
    wait_command(
        title    = "Find where text lives",
        command  = "searchtext",
        on_error = "Open the content search with `<cmd>searchtext` and search for a string.",
    )
    wait_event(
        event    = "open",
        title    = "Find where text lives",
        text     = "Pick a result to jump to that line.",
        on_error = "Choose a result and press `<enter>` to jump to that line.",
    )
    notify(level = success, message = "You found text across the workspace.")


def teach_lsp_intro():
    floating_window(title = "Code intelligence", text = lsp_intro_md,
                    dismiss_keys = [ck])


def teach_definition_at_cursor():
    floating_window(title = "Go to definition", text = lsp_definition_cursor_md,
                    allow_keys = move_keys,
                    dismiss_keys = dismiss_for("lsp", "definition"))
    wait_command(
        title    = "Go to definition",
        command  = "lsp",
        on_error = ("Put your cursor on a symbol and run `<cmd>lsp definition` " +
                    "(no argument) to jump to its definition."),
    )
    notify(level = success, message = "You jumped to the definition under your cursor.")


def teach_definition_by_name():
    floating_window(title = "Find a definition by name",
                    text = lsp_definition_name_md,
                    dismiss_keys = def_by_name_dismiss)
    wait_command(
        title    = "Find a definition by name",
        command  = "lsp",
        on_error = ("Run `<cmd>lsp definition <name>` with a symbol name, or " +
                    "press `<alt-shift-d>` to prefill `lsp definition ` and " +
                    "type the name."),
    )
    notify(level = success, message = "You jumped to a definition by name.")


def teach_cursorhistory():
    floating_window(title = "Navigate back and forth", text = cursorhistory_md,
                    dismiss_keys = dismiss_for("cursorhistory", "prev"))
    wait_command(
        title    = "Jump back",
        command  = "cursorhistory",
        on_error = "Go back with `<cmd>cursorhistory prev`.",
    )
    notify(level = success, message = "You jumped back.")

    floating_window(title = "Jump forward", text = cursorhistory_next_md,
                    dismiss_keys = dismiss_for("cursorhistory", "next"))
    wait_command(
        title    = "Jump forward",
        command  = "cursorhistory",
        on_error = "Go forward again with `<cmd>cursorhistory next`.",
    )
    notify(level = success, message = "You walked the cursor history back and forth.")


def teach_lsp_more():
    floating_window(title = "Ask about a symbol", text = lsp_more_md,
                    allow_keys = move_keys,
                    dismiss_keys = dismiss_for("lsp", "references"))
    wait_command(
        title    = "Ask about a symbol",
        command  = "lsp",
        on_error = ("Put your cursor on a symbol and run `<cmd>lsp references`, " +
                    "`<cmd>lsp implementation`, or `<cmd>lsp hover`."),
    )
    notify(level = success, message = "You asked the language server about a symbol.")


def run():
    if not workspace_open():
        fail("Open a workspace first before running this tutorial. Open the " +
             "command prompt with `" + ck + "` and run `workspaceopen`.")
        return

    if not is_lsp_server_running():
        fail("For this tutorial to be useful, you should run it in a workspace " +
             "that contains a project module of one of our supported languages. " +
             'Run the "help" command to learn about which languages we support. ' +
             "If the workspace is a monorepo with multiple nested projects, " +
             "opening a file in a sub-project will start the language server " +
             "automatically.")
        return

    floating_window(title = "Navigate code", text = intro_md, dismiss_keys = [ck])

    teach_searchfile()
    teach_searchtext()
    teach_lsp_intro()
    teach_definition_at_cursor()
    teach_definition_by_name()
    teach_cursorhistory()
    teach_lsp_more()

    floating_window(title = "You can navigate code", text = wrapup_md,
                    alignment = "top", dismiss_keys = [ck])


tutorial(id = "navigation", title = "Navigate code", version = "7", entry = run)
