# basics.star — the first-run tutorial.
#
# Walks the user through the three things they need to know to be
# productive in Rune: where to put work in the empty workspace,
# how to open a workspace (`:workspaceopen`), and how to start editing
# files inside that workspace (`:edit`), plus the layout model
# (the meta/alt/shift system) and the windows and tabs that hold
# that content.
#
# The DSL is interpreted by ide/idetutorial/starlarktutorial. The
# entry function runs on its own Starlark goroutine; each blocking
# builtin (floating_window, wait_command, confirm) returns a real
# value so authors can branch, loop, and compose helpers with
# regular Starlark control flow.

ck = command_key()
mode = editor_mode()

# Buffer motion and layout direction are separate systems. The file explorer
# uses each editor's native movement, while layout commands use HJKL in modal
# mode, IJKL in standard mode, and PNBF in Emacs mode.
if mode == "modal":
    dir_phrase = "the home row, `h` `j` `k` `l`"
    completer_pick_phrase = "`<ctrl-j>` / `<ctrl-k>` (or `<up>` / `<down>`)"
    modal_surface_allow_keys = ["<esc>"]
elif mode == "emacs":
    dir_phrase = "the motion keys `<ctrl-p>` / `<ctrl-n>` or the arrow keys"
    completer_pick_phrase = "`<ctrl-p>` / `<ctrl-n>` (or `<up>` / `<down>`)"
    modal_surface_allow_keys = []
else:
    dir_phrase = "the arrow keys"
    completer_pick_phrase = "the arrow keys `<up>` / `<down>`"
    modal_surface_allow_keys = []

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
    return ("open the command prompt (`" + ck + "`) and run `" +
            command_line(cmd, args) + "`")

def command_line(cmd, args):
    return cmd + ((" " + " ".join(args)) if len(args) else "")

def keylabel(cmd, *args):
    k = key_for(cmd, *args)
    return "`" + (k if k else command_line(cmd, args)) + "`"

def dismiss_for(cmd, *args):
    # Keys that dismiss a teaching window. Always include the command
    # prompt key. When the command has a bound key, include it too so
    # the single keypress the copy asks for dismisses the window AND
    # falls through to the IDE, dispatching the command that the
    # following wait_command observes.
    k = key_for(cmd, *args)
    return [ck, k] if k else [ck]

workspace_slot_keys = [
    key_for("workspacefocus", "1"),
    key_for("workspacefocus", "2"),
    key_for("workspacefocus", "3"),
    key_for("workspacefocus", "4"),
    key_for("workspacefocus", "5"),
    key_for("workspacefocus", "6"),
    key_for("workspacefocus", "7"),
    key_for("workspacefocus", "8"),
    key_for("workspacefocus", "9"),
]
workspace_slot_key_row = " ".join([
    keylabel("workspacefocus", "1"),
    keylabel("workspacefocus", "2"),
    keylabel("workspacefocus", "3"),
    keylabel("workspacefocus", "4"),
    keylabel("workspacefocus", "5"),
    keylabel("workspacefocus", "6"),
    keylabel("workspacefocus", "7"),
    keylabel("workspacefocus", "8"),
    keylabel("workspacefocus", "9"),
])
welcome_allow_keys = [k for k in workspace_slot_keys + [key_for("terminalneworsplit")] if k]

def args_match(got, want):
    if len(got) != len(want):
        return False
    for i in range(len(want)):
        if got[i] != want[i]:
            return False
    return True

def with_key(text, cmd, args):
    # The hint's own "Or you can press ..." line resolves the bare
    # command name, which has no binding for direction-qualified
    # commands. Name the exact chord the step is waiting for.
    k = key_for(cmd, *args)
    if not k:
        return text
    if text.endswith("."):
        text = text[:-1]
    return text + " with `" + k + "`."

def wait_expected_command(title, command, expected_args, on_error, alignment = ""):
    # A successfully dispatched command has already taken effect, so keep the
    # lesson armed and ask for the intended direction rather than rejecting it.
    #
    # on_error doubles as the hint body. These steps come in sequences that
    # ask for one direction and then another, and the generic "try the
    # <command> command" hint plus its manual cannot say which one is due
    # next -- the same command satisfies both halves.
    hint = with_key(on_error, command, expected_args)
    for _ in range(1000):
        result = wait_command(
            title     = title,
            command   = command_line(command, expected_args),
            on_error  = hint,
            text      = hint,
            alignment = alignment,
        )
        if args_match(result.args, expected_args):
            return
        notify(
            level   = info,
            message = ("That ran `" + command_line(command, result.args) +
                       "`. Now run `" + command_line(command, expected_args) + "`."),
        )

focus_key_row = " | ".join([
    keylabel("windowfocus", "up"),
    keylabel("windowfocus", "left"),
    keylabel("windowfocus", "down"),
    keylabel("windowfocus", "right"),
])
move_key_row = " | ".join([
    keylabel("windowmove", "up"),
    keylabel("windowmove", "left"),
    keylabel("windowmove", "down"),
    keylabel("windowmove", "right"),
])
resize_key_row = " | ".join([
    keylabel("windowresize", "increase", "height"),
    keylabel("windowresize", "decrease", "width"),
    keylabel("windowresize", "decrease", "height"),
    keylabel("windowresize", "increase", "width"),
])
# Every binding the layout table lists, so the page that shows it can
# pass them through to the IDE and let the user try each one.
layout_play_keys = [k for k in [
    key_for("windowfocus", "up"),
    key_for("windowfocus", "left"),
    key_for("windowfocus", "down"),
    key_for("windowfocus", "right"),
    key_for("windowmove", "up"),
    key_for("windowmove", "left"),
    key_for("windowmove", "down"),
    key_for("windowmove", "right"),
    key_for("windowresize", "increase", "height"),
    key_for("windowresize", "decrease", "width"),
    key_for("windowresize", "decrease", "height"),
    key_for("windowresize", "increase", "width"),
] if k]
# Emacs has no bare `windownew`; its split keys are direction-qualified.
# Use the rightward one so every mode ends up with the same layout.
split_window_args = ["right"] if mode == "emacs" else []

if mode == "modal":
    layout_pattern_md = """\
## HJKL controls the layout

Vim's keyboard-first design keeps navigation under your fingers. Repeated
actions become muscle memory, so you spend less time searching for interface
controls and can keep your attention on the work.

Rune carries that same HJKL language into layout management: `H` points left,
`J` down, `K` up, and `L` right.

- Hold `<meta>` and press HJKL to focus a window in that direction.
- Hold `<alt>` and press H/L to focus the previous or next tab.
- Add `<shift>` to move content instead of focus it. `<meta>` + `<shift>` +
  HJKL moves the focused window's content; `<alt>` + `<shift>` + H/L moves
  the current tab left or right in the tab list.
"""
elif mode == "emacs":
    layout_pattern_md = """\
## Emacs directions control the layout

Rune keeps `<ctrl-p>` / `<ctrl-n>` and `<ctrl-b>` / `<ctrl-f>` available for
editing. Rather than teach a second direction map, Rune changes the target:
hold `<meta>` with the same PNBF directions to focus windows, then add `<shift>`
to move window content instead. Reusing that muscle memory keeps repeated
layout actions fast, and the host Meta layer stays reachable from terminals.

- """ + keylabel("windowclose") + """ closes a window, """ + keylabel("windowcloseall") + """ closes the others,
  """ + keylabel("windownew", "down") + """ / """ + keylabel("windownew", "right") + """ split below or right, and
  """ + keylabel("windowtogglemaximize") + """ toggles maximization.
- """ + keylabel("tabclose") + """ closes a tab; adding `<shift>` escalates from the tab to the whole window.
- """ + keylabel("tabprevious") + """ / """ + keylabel("tabnext") + """ cycle tabs, while
  """ + keylabel("tabmove", "left") + """ / """ + keylabel("tabmove", "right") + """ reorder the current tab.
"""
else:
    layout_pattern_md = """\
## Alt drives the layout

Vim made generations of programmers extraordinarily productive by keeping
navigation under their fingers. Repeated actions become muscle memory,
reducing menu hunting and the mental fatigue of switching attention between
code and interface controls.

Keyboard-driven does not have to mean learning an entirely new way to edit.
Rune brings that advantage to a familiar, non-modal editor by treating IJKL
as a second set of arrow keys:

```text
    I
  J K L
```

`I` points up, `J` left, `K` down, and `L` right. For example:

- Hold `<alt>` and press IJKL to focus a window. Add `<shift>` to move its
  content, or add `<meta>` to resize it.
- Use """ + keylabel("tabprevious") + """ / """ + keylabel("tabnext") + """ to switch tabs.
  Add `<shift>` to reorder the current tab instead.
- Use """ + keylabel("windownew") + """ to split a window, """ + keylabel("terminalneworsplit") + """
  to open a terminal, and """ + keylabel("windowclose") + """ to close a window.
- Use """ + keylabel("tabnew") + """ to create a tab and """ + keylabel("tabclose") + """ to close it.

The pattern is Alt plus the target: IJKL affects windows, brackets affect tabs,
and adding `<shift>` moves content instead of focus.
"""

# The workspace-open step branches on the host OS. On macOS the File
# menu's Open Project… item drives the native open panel, so the copy
# points there first; the command prompt stays documented as the
# keyboard alternative. wait_command("workspaceopen") advances on
# either path, so only the wording changes.
if os() == "darwin":
    open_workspace_section = """\
## Opening a project

1. Open the **File** menu in the macOS menu bar and choose
   **Open Project…**.
2. Pick a project directory in the panel and click **Open**.

Prefer the keyboard? Press `""" + ck + """`, type `workspaceopen`, and use
the auto-completer to pick a workspace or type the path yourself."""
else:
    open_workspace_section = """\
## Opening a workspace

1. Press `""" + ck + """` to open the command prompt.
2. Type `workspaceopen` and use the auto-completer
   to **pick a workspace** from the list, or type the path yourself.
3. Press Enter to open it."""

welcome_md = """\
This is the **home workspace**: a scratch workspace rooted at `~/` that
Rune shows when no project workspace is open at the current slot.
Use it for files outside any project, quick terminals,
or to keep notes between sessions.

## Switching workspaces

Rune has **nine workspace slots**. Their current bindings are:

""" + workspace_slot_key_row + """

Press one to jump to that slot.
Every empty slot shows this same home workspace; a slot only gets a project attached when
you open one inside it. So slot 1 may be the project you're working on while slots 2-9 are
still the home workspace, ready for whatever you need.

## Commands and key bindings

IDE-wide operations are exposed as **commands** that you invoke
from the command prompt. Bindings like """ + keylabel("workspacefocus", "1") + """ and """ + keylabel("terminalneworsplit") + """
are bound to those commands through your user configuration under
`command.key_bindings`, so every binding shown here is rebindable.

""" + open_workspace_section + """

Press `<enter>` or `<space>` to continue.
"""

edit_md = """\
You're in a real workspace now. To open a file:

1. Press `""" + ck + """` to open the command prompt.
2. Type `edit` followed by a partial filename.
3. Use the auto-completer to pick the file you want.
4. Press Enter.

Press `<enter>` or `<space>` to continue.
"""

layout_md = """\
Rune is a full tiling window manager: you split the screen into
**windows**, fill each one with **tabs** (files, terminals, task
output), and group whole projects into **workspaces**. The editor is
just one kind of content among many. Every layout action is a command
you can type at the prompt; the default keys are just shortcuts, and
they follow a directional pattern.

""" + layout_pattern_md + """

Press `<enter>` or `<space>` to continue.
"""

directional_layout_md = """\
## Your current layout keys

| Action | Up | Left | Down | Right |
| --- | --- | --- | --- | --- |
| Focus | """ + focus_key_row + """ |
| Move | """ + move_key_row + """ |
| Resize | """ + resize_key_row + """ |

The table follows your current configuration, including custom bindings.

**Play with them and get comfortable.** They all work while this window
is up: the editor and the two terminals are behind it.

Press `<enter>` or `<space>` to continue.
"""

split_window_md = """\
A **window** is a tile on screen, and right now this workspace has just
one.

Split the focused window in two: """ + keypress("windownew", *split_window_args) + """.
The new pane lands to the right.
"""

terminal_md = """\
The new pane is empty, and windows hold any kind of content, not just
files. Fill this one with a terminal.

Open a terminal here: """ + keypress("terminalneworsplit") + """.
"""

modal_surfaces_md = """\
You picked **modal** editor mode, and in modal mode every input surface
is modal, not just the editor. This includes the terminal, Rune's
console, and the file explorer.

The cursor shape tells you which mode a surface is in: a block cursor
means NORMAL mode, a bar cursor means INSERT mode.

So when a terminal or console is focused and you want to open the command
prompt (`""" + ck + """`), first switch back to NORMAL mode with
`<esc>`.
"""

split_horizontal_md = """\
Splits have a direction, and so far every one has landed to the right.
`windowdefaultsplit` aims the next one.

Send it **below** the focused window instead.

Flip the split direction: """ + keypress("windowdefaultsplit", "h") + """.
"""

terminal_split_md = """\
The same key opens a terminal in a new split when the window already
has content. Do it again and watch the split land along the direction
you just set.

Open another terminal split: """ + keypress("terminalneworsplit") + """.
"""

focus_window_md = """\
The screen holds three windows now: the editor on the left, and the two
terminals stacked on the right. Directional layout commands move focus
across the splits, so you can hop between them without the mouse.

- `windowfocus up` focuses the window above.""" + keyhint("windowfocus", "up") + """
- `windowfocus left` focuses the window to the left.""" + keyhint("windowfocus", "left") + """

First focus the terminal above (""" + keypress("windowfocus", "up") + """),
then the editor on the left (""" + keypress("windowfocus", "left") + """).
"""

move_window_md = """\
`<shift>` turns "go to" into "move". Add it to the focus direction and the
focused window's content swaps with its neighbor.

- `windowmove right` moves the focused window to the right.""" + keyhint("windowmove", "right") + """
- `windowmove left` moves it back to the left.""" + keyhint("windowmove", "left") + """

Move the editor to the right (""" + keypress("windowmove", "right") + """),
then back to the left (""" + keypress("windowmove", "left") + """).
"""

resize_direction_md = ("""\
Window resizing keeps the IJKL directions: `I` makes the window taller, `J`
narrower, `K` shorter, and `L` wider.
""" if mode == "standard" else ("""\
Emacs mode keeps resize on host-Meta arrows instead of taking more editing
letters: up makes the window taller, left narrower, down shorter, and right
wider. These work from terminals as well as editors.
""" if mode == "emacs" else """\
Window resizing uses matching arrow directions: up makes the window taller,
left narrower, down shorter, and right wider.
"""))

resize_window_md = resize_direction_md + """

- `windowresize increase width` makes the focused window wider.""" + keyhint("windowresize", "increase", "width") + """
- `windowresize decrease width` makes it narrower.""" + keyhint("windowresize", "decrease", "width") + """

First make the window wider (""" + keypress("windowresize", "increase", "width") + """),
then make it narrower again (""" + keypress("windowresize", "decrease", "width") + """).
"""

fullscreen_window_md = """\
When you want to focus on one window, `windowtogglemaximize` grows it
to fill the whole editor area. Run it again, or focus another window,
to restore the layout.""" + keyhint("windowtogglemaximize") + """

Toggle fullscreen now: """ + keypress("windowtogglemaximize") + """.
"""

close_window_md = """\
`windowclose` closes the focused split and hands focus back to the
window you came from. It will not close your last window: a workspace
always keeps at least one.""" + keyhint("windowclose") + """

Focus a terminal (""" + keypress("windowfocus", "right") + """), then close
it (""" + keypress("windowclose") + """). Focus lands back on the editor.
"""

emacs_close_others_md = """\
You used """ + keylabel("windowclose") + """ to close one window. It mirrors the familiar
browser pattern where """ + keylabel("tabclose") + """ closes a tab and adding `<shift>` closes
the whole window. """ + keylabel("windowcloseall") + """ keeps the focused window and closes
every other split.

Keep only this window: """ + keypress("windowcloseall") + """.
"""

tabs_intro_md = ("""\
A window shows one **tab** at a time: a file, a terminal, task output. Emacs
mode keeps tab lifecycle on Rune's host Meta layer: """ + keylabel("tabnew") + """ starts a
new tab and """ + keylabel("tabclose") + """ closes the current one.
""" if mode == "emacs" else """\
A window shows one **tab** at a time: a file, a terminal, task output.
""")

tabs_md = tabs_intro_md + """
You already have one open. Let's add another from the file explorer.

Open the file explorer: """ + keypress("fexplorer") + """.
"""

tab_open_file_md = """\
The explorer is the workspace tree, and it is a live buffer: edit a
name to rename, add a line to create, delete a line to remove, then
save the buffer via `write` to apply. For now, just open a file.

Move to a file with """ + dir_phrase + """ and press `<enter>`. It opens
as a new tab in the window you came from.
"""

tab_close_explorer_md = """\
`fexplorer` is a toggle: the same key that opened the explorer closes
it again. With your file open, you no longer need the tree taking up
space.

Toggle the explorer closed: """ + keypress("fexplorer") + """.
"""

tab_switch_intro_md = ("""\
That window now holds two tabs. Emacs muscle memory works here too:
""" + keylabel("tabprevious") + """ moves to the previous tab and """ + keylabel("tabnext") + """ moves to the next.
Both wrap around.
""" if mode == "emacs" else """\
That window now holds two tabs. `tabnext` / `tabprevious` cycle through them
and wrap around. Each preset has a horizontal tab pair; adding `<shift>` to
that pair reorders the current tab instead.
""")

tab_switch_md = tab_switch_intro_md + """

Switch to the next tab (""" + keypress("tabnext") + """), then back to
the previous one (""" + keypress("tabprevious") + """).
"""

tab_move_intro_md = ("""\
For tab placement, """ + keylabel("tabmove", "left") + """ moves the current tab left and
""" + keylabel("tabmove", "right") + """ moves it right.
""" if mode == "emacs" else """\
Now use the same horizontal pair with `<shift>` to change the tab's position
instead of switching tabs.
""")

tab_move_md = tab_move_intro_md + """

- `tabmove left` moves it one slot left.""" + keyhint("tabmove", "left") + """
- `tabmove right` moves the current tab one slot right.""" + keyhint("tabmove", "right") + """

Move the current tab left (""" + keypress("tabmove", "left") + """), then
move it back right (""" + keypress("tabmove", "right") + """).
"""

close_tab_md = """\
`tabclose` closes the focused tab; the next tab in the list takes its
place.""" + keyhint("tabclose") + """

To close the current tab, """ + keypress("tabclose") + """.
"""

terminals_md = """\
Rune runs terminals as content, so they live in windows and tabs just
like files. Rune can also run one-shot programs on an ephemeral terminal,
interactively.

## Running a program

- `! <cmd>` runs a program in a floating window that shows its output,
  for example `! git log`.
- `!!` runs a program but hides its output. Use this when you only cares
  whether it worked or not.

Let's run `git log`. At the command prompt (`""" + ck + """`), type
`! git log` and press Enter. Rune opens a floating window streaming its
output.
"""

terminals_close_md = """\
The `git log` output is sitting in a floating window. It is a window,
not a tab, so close it with `windowclose`.

Close it now: """ + keypress("windowclose") + """.
"""

guicommands_md = """\
You have run a lot of commands by now: `windownew`, `terminalneworsplit`, `tabnext`,
`fexplorer`. They are all lowercase, no spaces, and read like `<category><action>`: the
category first (`window`, `terminal`, `tab`), then what you do to it (`new`, `close`,
`next`).

This is the Rune way: **form earns its place by serving function**. The naming
is what makes the fuzzy finder predictable and optimized for recall: every command in a
category shares a prefix, you can guess the sequence and let the finder rank it first.
`workspaceopen` resolves from typing `woso`, and `woso` fires in four keystrokes
instead of thirteen.

Let's try it with themes. Themes control every color Rune draws with, and the `guitheme`
command switches the active one.

1. Press `""" + ck + """` to open the command prompt.
2. Type `guith` and press `<space>` to complete to `guitheme`.
3. Type a space, then use """ + completer_pick_phrase + """ to pick a
   different theme from the completer, and press `<enter>`.
"""

config_open_md = """\
Nice, the whole interface just re-themed at once, terminal colors and
all.

`guitheme` only changed this session. The file that decides what Rune
looks like when it starts is one keypress away.

Open it now: """ + keypress("config") + """.
"""

def config_edit_md(theme):
    return """\
`command.key_bindings` is active: every key this tutorial taught you
lives there. Everything else is commented out at Rune's own defaults,
so reading the file is how you find what is tunable.

Scroll to `gui.default_theme` and set it to `""" + theme + """`, then
save: """ + keypress("write") + """.
"""

config_done_md = """\
Rune reads the config once at startup, so `gui.default_theme` takes
effect the next time you launch. `guitheme` stays the quick way to
change the theme for the session you are in right now.

Press `<enter>` or `<space>` to continue.
"""

cheatsheet_md = """\
There's a `cheatsheet` command that condenses all of this tutorial's
learnings and more into a single cheat sheet you can pull up any time.

Open it now: """ + keypress("cheatsheet") + """.
"""

def teach_edit():
    floating_window(title = "Open a file", text = edit_md, dismiss_keys = [ck])
    edit = wait_command(
        title    = "Open a file",
        command  = "edit",
        on_error = ("`<cmd>edit` needs a `<file>` argument. Use the " +
                    "auto-completer (Tab / arrow keys) to pick a " +
                    "file, or type a path inside the workspace; " +
                    "relative paths resolve against the workspace root."),
    )
    notify(level = success, message = "You opened " + edit.args[0])


def teach_layout():
    floating_window(title = "Layout management", text = layout_md,
                    dismiss_keys = [ck])


def teach_directional_layout():
    # This lesson and the focus/move/resize ones anchor at the top so the
    # lower half of the layout stays visible: the user needs to watch the
    # windows they just opened move, swap, and resize.
    floating_window(title = "Your directional layout", text = directional_layout_md,
                    allow_keys = layout_play_keys,
                    alignment = "top")


def teach_split_window():
    floating_window(title = "Split a window", text = split_window_md,
                    dismiss_keys = dismiss_for("windownew", *split_window_args))
    if len(split_window_args):
        wait_expected_command(
            title         = "Split a window",
            command       = "windownew",
            expected_args = split_window_args,
            on_error      = "Split the active window to the right.",
        )
    else:
        wait_command(
            title    = "Split a window",
            command  = "windownew",
            on_error = ("Split the active window into two. Add an optional " +
                        "direction (`<cmd>windownew right` / `left` / `up` / " +
                        "`down`) to choose where the new pane lands."),
        )
    notify(level = success, message = "You split the window.")


def teach_terminal():
    floating_window(title = "Open a terminal", text = terminal_md,
                    dismiss_keys = dismiss_for("terminalneworsplit"))
    wait_command(
        title    = "Open a terminal",
        command  = "terminalneworsplit",
        on_error = ("Open a terminal in the focused window. When the window " +
                    "is empty the terminal fills it in place."),
    )
    notify(level = success, message = "You opened a terminal.")


def teach_modal_surfaces():
    if mode != "modal":
        return
    floating_window(title = "Modal everywhere", text = modal_surfaces_md,
                    allow_keys = modal_surface_allow_keys,
                    dismiss_keys = [ck])


def teach_split_horizontal():
    floating_window(title = "Aim the next split", text = split_horizontal_md,
                    dismiss_keys = dismiss_for("windowdefaultsplit", "h"))
    wait_expected_command(
        title         = "Aim the next split",
        command       = "windowdefaultsplit",
        expected_args = ["h"],
        on_error      = ("Send the next split below with " +
                         "`<cmd>windowdefaultsplit h`."),
    )
    notify(level = success, message = "Splits now land below.")


def teach_terminal_split():
    floating_window(title = "Open a terminal split", text = terminal_split_md,
                    dismiss_keys = dismiss_for("terminalneworsplit"))
    wait_command(
        title    = "Open a terminal split",
        command  = "terminalneworsplit",
        on_error = ("Split the focused window and open a terminal in the new " +
                    "pane. Because the window already has content, it opens " +
                    "a split instead of filling it in place."),
    )
    notify(level = success, message = "You opened a terminal split.")


def teach_focus_window():
    floating_window(title = "Move between windows", text = focus_window_md,
                    dismiss_keys = dismiss_for("windowfocus", "up"),
                    alignment = "top")
    wait_expected_command(
        title         = "Move between windows",
        command       = "windowfocus",
        expected_args = ["up"],
        on_error      = "Focus the terminal above.",
        alignment     = "top",
    )
    wait_expected_command(
        title         = "Move between windows",
        command       = "windowfocus",
        expected_args = ["left"],
        on_error      = "Now focus the editor on the left.",
        alignment     = "top",
    )
    notify(level = success, message = "You moved between windows.")


def teach_move_window():
    floating_window(title = "Move a window", text = move_window_md,
                    dismiss_keys = dismiss_for("windowmove", "right"),
                    alignment = "top")
    wait_expected_command(
        title         = "Move a window",
        command       = "windowmove",
        expected_args = ["right"],
        on_error      = "Move the focused window to the right.",
        alignment     = "top",
    )
    wait_expected_command(
        title         = "Move a window",
        command       = "windowmove",
        expected_args = ["left"],
        on_error      = "Now move it back to the left.",
        alignment     = "top",
    )
    notify(level = success, message = "You moved a window.")


def teach_resize_window():
    floating_window(title = "Resize a window", text = resize_window_md,
                    dismiss_keys = dismiss_for("windowresize", "increase", "width"),
                    alignment = "top")
    wait_expected_command(
        title         = "Resize a window",
        command       = "windowresize",
        expected_args = ["increase", "width"],
        on_error      = "Make the focused window wider.",
        alignment     = "top",
    )
    wait_expected_command(
        title         = "Resize a window",
        command       = "windowresize",
        expected_args = ["decrease", "width"],
        on_error      = "Now make it narrower again.",
        alignment     = "top",
    )
    notify(level = success, message = "You resized a window.")


def teach_fullscreen_window():
    floating_window(title = "Fullscreen a window", text = fullscreen_window_md,
                    dismiss_keys = dismiss_for("windowtogglemaximize"))
    wait_command(
        title    = "Fullscreen a window",
        command  = "windowtogglemaximize",
        on_error = "Toggle the focused window to fullscreen and back.",
    )
    notify(level = success, message = "You toggled fullscreen.")


def teach_close_window():
    floating_window(title = "Close a window", text = close_window_md,
                    dismiss_keys = dismiss_for("windowfocus", "right"))
    wait_expected_command(
        title         = "Close a window",
        command       = "windowfocus",
        expected_args = ["right"],
        on_error      = "Focus one of the terminals on the right.",
    )
    wait_command(
        title    = "Close a window",
        command  = "windowclose",
        on_error = ("Close the focused split. It will not close your last " +
                    "window."),
        text     = "Now close the terminal you just focused.",
    )
    notify(level = success, message = "You closed the window.")


def teach_emacs_close_others():
    if mode != "emacs":
        return
    floating_window(title = "Keep one window", text = emacs_close_others_md,
                    dismiss_keys = dismiss_for("windowcloseall"))
    wait_command(
        title    = "Keep one window",
        command  = "windowcloseall",
        on_error = "Close every window except the focused one.",
    )
    notify(level = success, message = "You cleaned up the window layout.")


def teach_tabs():
    floating_window(title = "Open another tab", text = tabs_md,
                    dismiss_keys = dismiss_for("fexplorer"))
    wait_command(
        title    = "Open another tab",
        command  = "fexplorer",
        on_error = "Open the file explorer with `<cmd>fexplorer`.",
    )
    notify(level = success, message = "File explorer open.")

    wait_event(
        event    = "open",
        title    = "Open a file",
        text     = tab_open_file_md,
        on_error = "Move to a file in the explorer and press `<enter>` to open it.",
    )
    notify(level = success, message = "You opened a file in a new tab.")

    floating_window(title = "Close the explorer", text = tab_close_explorer_md,
                    dismiss_keys = dismiss_for("fexplorer"))
    wait_command(
        title    = "Close the explorer",
        command  = "fexplorer",
        on_error = "Toggle the file explorer closed with `<cmd>fexplorer`.",
    )
    notify(level = success, message = "File explorer closed.")


def teach_switch_tabs():
    floating_window(title = "Switch tabs", text = tab_switch_md,
                    dismiss_keys = dismiss_for("tabnext"))
    wait_command(
        title    = "Switch tabs",
        command  = "tabnext",
        on_error = ("Move to the next tab in this window. If the window " +
                    "only has one tab, open a second file from the " +
                    "explorer first."),
    )
    wait_command(
        title    = "Switch tabs",
        command  = "tabprevious",
        on_error = "Now move back to the previous tab.",
        text     = "Now move back to the previous tab.",
    )
    notify(level = success, message = "You switched tabs.")


def teach_move_tabs():
    floating_window(title = "Reorder tabs", text = tab_move_md,
                    dismiss_keys = dismiss_for("tabmove", "left"))
    wait_expected_command(
        title         = "Reorder tabs",
        command       = "tabmove",
        expected_args = ["left"],
        on_error      = "Move the current tab one slot to the left.",
    )
    wait_expected_command(
        title         = "Reorder tabs",
        command       = "tabmove",
        expected_args = ["right"],
        on_error      = "Now move it one slot back to the right.",
    )
    notify(level = success, message = "You reordered the tabs.")


def teach_close_tab():
    floating_window(title = "Close a tab", text = close_tab_md,
                    dismiss_keys = dismiss_for("tabclose"))
    wait_command(
        title    = "Close a tab",
        command  = "tabclose",
        on_error = "Close the focused tab.",
    )
    notify(level = success, message = "You closed the tab.")


def teach_terminals():
    floating_window(title = "Run a program", text = terminals_md,
                    dismiss_keys = [ck])
    wait_command(
        title    = "Run a program",
        command  = "! git log",
        on_error = "At the command prompt, run `<cmd>! git log`.",
    )
    notify(level = success, message = "Program running in a window.")

    floating_window(title = "Close the output window", text = terminals_close_md,
                    dismiss_keys = dismiss_for("windowclose"))
    wait_command(
        title    = "Close the output window",
        command  = "windowclose",
        on_error = "Close the `git log` output window with `<cmd>windowclose`.",
    )
    notify(level = success, message = "Output window closed.")


def teach_cheatsheet():
    floating_window(title = "Your cheatsheet", text = cheatsheet_md,
                    dismiss_keys = dismiss_for("cheatsheet"))
    wait_command(
        title    = "Your cheatsheet",
        command  = "cheatsheet",
        on_error = "Run the `<cmd>cheatsheet` command to open your cheatsheet.",
    )
    notify(level = success, message = "That is your cheatsheet.")


def teach_guicommands():
    floating_window(title = "Why commands look like that", text = guicommands_md,
                    dismiss_keys = [ck])
    theme = wait_command(
        title    = "Switch the theme",
        command  = "guitheme",
        on_error = ("Run `<cmd>guitheme` and pass a theme name. Type " +
                    "`guith`, press `<tab>` to complete, then use " +
                    completer_pick_phrase + " to pick a theme from the " +
                    "completer."),
    )
    notify(level = success, message = "You switched the theme.")
    return theme.args[0] if len(theme.args) else "romero"


def teach_config(theme):
    floating_window(title = "Your configuration", text = config_open_md,
                    dismiss_keys = dismiss_for("config"))
    wait_command(
        title    = "Your configuration",
        command  = "config",
        on_error = "Run `<cmd>config` to open your configuration file.",
    )
    notify(level = success, message = "That is your config file.")

    # `config` opens whatever path the binary was launched with
    # (config.yaml, config.star, or a `-c` override), so match the URI
    # on a substring rather than a full path.
    wait_event(
        event = "flush",
        uri   = "config",
        title = "Make it stick",
        text  = config_edit_md(theme),
    )
    notify(level = success, message = "Config saved.")

    floating_window(title = "Make it stick", text = config_done_md,
                    dismiss_keys = [ck])


def run():
    floating_window(
        title = "Welcome",
        text = welcome_md,
        allow_keys = welcome_allow_keys,
        dismiss_keys = [ck],
    )

    ws = wait_command(
        title    = "Welcome",
        command  = "workspaceopen",
        on_error = ("`<cmd>workspaceopen` needs a `<workspacepath>` " +
                    "argument. Use the auto-completer (Tab / arrow " +
                    "keys) to pick a workspace, or type a directory " +
                    "path (it will be created if it doesn't exist)."),
    )
    notify(level = success, message = "Opened workspace: " + ws.args[0])

    teach_edit()

    teach_layout()
    teach_split_window()
    teach_terminal()
    teach_modal_surfaces()
    teach_split_horizontal()
    teach_terminal_split()
    teach_directional_layout()
    teach_focus_window()
    teach_move_window()
    teach_resize_window()
    teach_fullscreen_window()
    teach_close_window()
    teach_emacs_close_others()
    teach_tabs()
    teach_switch_tabs()
    teach_move_tabs()
    teach_close_tab()
    teach_terminals()

    theme = teach_guicommands()
    teach_config(theme)

    teach_cheatsheet()


tutorial(id = "basics", title = "Rune basics", version = "58", entry = run)
