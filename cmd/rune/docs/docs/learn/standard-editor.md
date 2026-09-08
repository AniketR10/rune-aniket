---
sidebar_position: 20
---

# Standard Editor

Rune's standard editor follows the modeless conventions used by Sublime Text,
VS Code, Zed, IntelliJ, and native text controls. Typing inserts text, arrow
keys move the caret, and the ordinary `<shift>` movement keys extend the
selection through the same movement. Semantic-selection shortcuts are listed
separately below.

## Preset

Enable it in `config.yaml`:

```yaml tab
editor:
  mode: "standard"
```

The preset adapts application shortcuts to the operating system:

- On macOS, `<meta>` is Command and `<alt>` is Option.
- On Linux, common application shortcuts use `<ctrl>`, while `<meta>` is Super.

Every key on this page uses Rune's [key syntax](./key-syntax.md). Tables omit a
platform when the same shortcut works everywhere.

The Standard editor accepts the same built-in editing and movement keys on
every operating system. The tables list the conventional macOS and Linux forms,
but either form works on either platform. Application commands come from the
platform's preset and can differ by operating system.

## Remapping keys

Most editing and movement keys are built into the editor, not Rune commands,
so they cannot be rebound through `command.key_bindings`. To swap the physical
keys that trigger them, use [`gui.key_mapping`](./key-mapping.md), which rewrites
a key combination before it reaches the editor.

Application and layout shortcuts are Rune commands and can be changed through
`command.key_bindings`. The editor receives ordinary keys before the command
layer, so a command binding does not replace a key that the standard editor
already handles. Choose an unused chord when remapping a command.

## Windows and tabs

### Standard keybindings preset

When configuring Rune for the first time, we offer the ability to choose one of three
presets. If you choose the standard preset, Rune keeps familiar text-editing conventions
while putting windows, tabs, Git changes, and diagnostics on consistent keyboard layers,
powered by adding a second arrow key cluster at the `ijkl` keys. The same
layout-management bindings apply on macOS and Linux.

![Rune Standard editor preset default keybindings](https://assets.rune.build/images/standard-editor-keyboard-cheatsheet-v6.svg)

The preset uses a single `<alt>` layer, so a binding is guessable from the
key alone:

- `<alt>` plus `ijkl` points up, left, down, and right.
- `<alt>` plus a layout letter acts on a window or a tab.
- `<alt>` plus a code letter asks the language server about the caret.
- Adding `<shift>` turns focus into movement, or asks for a symbol by name.
- Adding `<meta>` to the `ijkl` cluster resizes instead of focusing.

No application binding in this preset needs three modifiers.

The tables below provide a copyable reference for the preset bindings:

| Action | Up / left / down / right |
| --- | --- |
| Focus a window | `<alt-i>` / `<alt-j>` / `<alt-k>` / `<alt-l>` |
| Move a window | `<alt-shift-i>` / `<alt-shift-j>` / `<alt-shift-k>` / `<alt-shift-l>` |
| Resize a window | `<alt-meta-i>` / `<alt-meta-j>` / `<alt-meta-k>` / `<alt-meta-l>` |

The resize directions are taller, narrower, shorter, and wider.
`<shift-meta-backspace>` restores the default size, while `<shift-meta-+>`
and `<shift-meta-->` maximize and minimize the focused window.

Common window actions stay on the same `<alt>` layer:

| Action | Key |
| --- | --- |
| Split the focused window | `<alt-n>` |
| Open a terminal in place or in a new split | `<alt-enter>` |
| Close the focused window | `<alt-q>` |
| Close every window | `<alt-shift-q>` |
| Toggle maximization of the focused window | `<alt-m>` |
| Set horizontal / vertical split orientation | `<alt-h>` / `<alt-v>` |
| Prefill `windowconverttab` in the command prompt | `<alt-shift-enter>` |

Tabs use brackets for left and right. Add `<shift>` to reorder the current
tab:

| Action | Key |
| --- | --- |
| Close tab | `<alt-w>` |
| Previous / next tab | `<alt-[>` / `<alt-]>` |
| Move tab left / right | `<alt-shift-[>` / `<alt-shift-]>` |
| Focus tab 1 to 9 | `<alt-1>` … `<alt-9>` |
| Move the current tab to position 1 to 9 | `<alt-shift-1>` … `<alt-shift-9>` |
| Search open tabs | ``<alt-`>`` |

New tab is a platform-native shortcut rather than an `<alt>` binding:
`<meta-t>` on macOS and `<ctrl-n>` on Linux. macOS also keeps `<meta-w>` as a
native close-tab alias. See
[Layout Management](./layout-management.md) for the complete layout system.

Common navigation commands avoid keys owned by the editor:

| Action | macOS | Linux |
| --- | --- | --- |
| Toggle the file explorer | `<shift-meta-e>` | `<ctrl-shift-e>` |
| Previous / next cursor position | `<alt-,>` / `<alt-.>` | `<alt-,>` / `<alt-.>` |
| Open the cheatsheet | `<alt-/>` | `<alt-/>` |

Code-intelligence commands share the same `<alt>` layer and are listed under
[Language intelligence](#language-intelligence-diagnostics-and-git-changes).

## Selection

Add `<shift>` to a movement to select through the same destination. Releasing
`<shift>` and moving the caret clears the selection. Pressing `<esc>` also
clears the selection. `<ctrl-shift-left>` and `<ctrl-shift-right>` are semantic
selection commands rather than word-selection variants.

| Movement | macOS | Linux |
| --- | --- | --- |
| Select one character | `<shift-left>` / `<shift-right>` | `<shift-left>` / `<shift-right>` |
| Select one line vertically | `<shift-up>` / `<shift-down>` | `<shift-up>` / `<shift-down>` |
| Select by word | `<alt-shift-left>` / `<alt-shift-right>` | `<alt-shift-left>` / `<alt-shift-right>` |
| Select to line boundary | `<shift-meta-left>` / `<shift-meta-right>` | `<shift-home>` / `<shift-end>` |
| Select to document boundary | `<shift-meta-up>` / `<shift-meta-down>` | `<ctrl-shift-home>` / `<ctrl-shift-end>` |
| Select to previous / next paragraph | `<ctrl-shift-up>` / `<ctrl-shift-down>` | `<ctrl-shift-up>` / `<ctrl-shift-down>` |

Line-start movements include leading whitespace. For example,
`<shift-meta-left>` on macOS selects all the way to column zero, not only to the
first non-blank character.

### Selection commands

| Action | macOS | Linux |
| --- | --- | --- |
| Select all | `<meta-a>` or `<ctrl-a>` | `<ctrl-a>` or `<meta-a>` |
| Select current line; repeat to extend | `<meta-l>` | `<meta-l>` |
| Select next occurrence | `<meta-d>` or `<ctrl-d>` | `<ctrl-d>` or `<meta-d>` |
| Select previous occurrence | `<ctrl-meta-d>` | `<ctrl-meta-d>` |
| Select indentation level | `<shift-meta-j>` | `<shift-meta-j>` |
| Expand semantic selection | `<ctrl-w>` or `<shift-meta-space>` | `<ctrl-w>` or `<shift-meta-space>` |
| Shrink semantic selection | `<ctrl-shift-w>` | `<ctrl-shift-w>` |
| Shrink / expand semantic selection | `<ctrl-shift-left>` / `<ctrl-shift-right>` | `<ctrl-shift-left>` / `<ctrl-shift-right>` |
| Select enclosing `()`, `{}`, or `[]` | `<ctrl-shift-m>` | `<ctrl-shift-m>` |
| Undo / redo a selection change | `<meta-u>` / `<shift-meta-u>` | `<meta-u>` / `<shift-meta-u>` |
| Clear selection | `<esc>` | `<esc>` |

Semantic selection depends on language support for the current buffer.

## Cursor movement

### Common movement

| Key | Movement |
| --- | --- |
| `<left>` / `<right>` | one character |
| `<up>` / `<down>` | one line |
| `<home>` / `<end>` | start / end of line |
| `<pgup>` / `<pgdn>` | one viewport up / down |

### Platform movement

| Movement | macOS | Linux |
| --- | --- | --- |
| Previous word start | `<alt-left>` | `<ctrl-left>` |
| Next word end | `<alt-right>` | `<ctrl-right>` |
| Previous / next paragraph | `<ctrl-up>` / `<ctrl-down>` | `<ctrl-up>` / `<ctrl-down>` |
| Start of line | `<meta-left>` | `<home>` |
| End of line | `<meta-right>` | `<end>` |
| Start of document | `<meta-up>` | `<ctrl-home>` |
| End of document | `<meta-down>` | `<ctrl-end>` |
| Jump to matching delimiter | `<ctrl-m>` | `<ctrl-m>` |

`<meta-left>` always means the actual start of the line. Rune keeps the
Meta-arrow combinations available to the editor rather than using them for
window focus or movement.

### Scrolling

| Action | macOS | Linux |
| --- | --- | --- |
| Move one viewport | `<pgup>` / `<pgdn>` | `<pgup>` / `<pgdn>` |
| Select one viewport | `<shift-pgup>` / `<shift-pgdn>` | `<shift-pgup>` / `<shift-pgdn>` |
| Center the current line | `<ctrl-l>` | `<ctrl-l>` |
| Scroll one line without moving the caret | `<ctrl-alt-up>` / `<ctrl-alt-down>` | `<ctrl-alt-up>` / `<ctrl-alt-down>` |

## Editing

### Typing and deletion

| Action | macOS | Linux |
| --- | --- | --- |
| Insert text | any printable key | any printable key |
| Newline | `<enter>` | `<enter>` |
| Indent / outdent | `<tab>` / `<shift-tab>` | `<tab>` / `<shift-tab>` |
| Delete left / right | `<backspace>` / `<delete>` | `<backspace>` / `<delete>` |
| Delete previous word | `<alt-backspace>` or `<ctrl-backspace>` | `<ctrl-backspace>` or `<alt-backspace>` |
| Delete next word | `<alt-delete>` or `<ctrl-delete>` | `<ctrl-delete>` or `<alt-delete>` |
| Delete to start of line | `<meta-backspace>` | `<meta-backspace>` |
| Delete to end of line | `<meta-delete>` | `<meta-delete>` |
| Cut to end of line | `<ctrl-k>` | `<ctrl-k>` |
| Delete current line | `<shift-meta-k>` | `<ctrl-shift-k>` |
| Transpose characters around the caret | `<ctrl-t>` | `<ctrl-t>` |

Word deletion follows the caret direction: Backspace deletes the word to the
left, and Delete removes the word to the right. At the end of a line,
`<ctrl-k>` cuts the newline and joins the following line.

### Clipboard

| Action | macOS | Linux |
| --- | --- | --- |
| Copy | `<meta-c>` or `<ctrl-c>` | `<ctrl-c>` or `<meta-c>` |
| Cut | `<meta-x>` or `<ctrl-x>` | `<ctrl-x>` or `<meta-x>` |
| Paste | `<meta-v>` or `<ctrl-v>` | `<ctrl-v>` or `<meta-v>` |
| Paste and reindent | `<shift-meta-v>` or `<ctrl-shift-v>` | `<ctrl-shift-v>` or `<shift-meta-v>` |
| Paste from clipboard history | `<alt-meta-v>` | `<alt-meta-v>` |

Copy and cut use the current line when there is no selection. Copy leaves the
buffer unchanged; cut removes the line. A later paste preserves the line-wise
clipboard behavior. After pasting, repeat `<alt-meta-v>` to replace that paste
with progressively older clipboard-history entries.

### Undo and redo

| Action | macOS | Linux |
| --- | --- | --- |
| Undo | `<meta-z>` or `<ctrl-z>` | `<ctrl-z>` or `<meta-z>` |
| Redo | `<shift-meta-z>`, `<meta-y>`, `<ctrl-shift-z>`, or `<ctrl-y>` | `<ctrl-shift-z>`, `<ctrl-y>`, `<shift-meta-z>`, or `<meta-y>` |

### Line operations

| Action | macOS | Linux |
| --- | --- | --- |
| Insert line below | `<ctrl-enter>` | `<ctrl-enter>` |
| Insert line above | `<ctrl-shift-enter>` | `<ctrl-shift-enter>` |
| Move line up / down | `<alt-up>` / `<alt-down>` | `<alt-up>` / `<alt-down>` |
| Duplicate line up / down | `<alt-shift-up>` / `<alt-shift-down>` | `<alt-shift-up>` / `<alt-shift-down>` |
| Duplicate line below | `<shift-meta-d>` | `<shift-meta-d>` |
| Join with next line | `<meta-j>` or `<ctrl-shift-j>` | `<meta-j>` or `<ctrl-shift-j>` |
| Indent line | `<meta-]>` or `<ctrl-]>` | `<ctrl-]>` or `<meta-]>` |
| Outdent line | `<meta-[>` or `<ctrl-[>` | `<ctrl-[>` or `<meta-[>` |
| Toggle line comment | `<meta-/>` or `<ctrl-/>` | `<ctrl-/>` or `<meta-/>` |
| Toggle block comment | `<alt-meta-/>` | `<alt-meta-/>` |
| Reflow paragraph at the ruler | `<alt-meta-q>` | `<alt-meta-q>` |
| Toggle soft wrap | `<alt-z>` | `<alt-z>` |

## Search

| Action | macOS | Linux |
| --- | --- | --- |
| Find in the current buffer | `<meta-f>` or `<ctrl-f>` | `<ctrl-f>` or `<meta-f>` |
| Open replace | `<meta-r>` | `<meta-r>` |
| Upgrade an open find to replace | `<meta-r>` | `<meta-r>` |
| Next match while open | `<enter>`, `<meta-f>`, or `<ctrl-f>` | `<enter>`, `<ctrl-f>`, or `<meta-f>` |
| Cycle between find and replacement inputs | `<tab>` / `<shift-tab>` | `<tab>` / `<shift-tab>` |
| Close and leave the active match selected | `<esc>` | `<esc>` |
| Next / previous match after closing | `<ctrl-->` / `<ctrl-shift-->` | `<ctrl-->` / `<ctrl-shift-->` |

Find opens a floating widget and selects the active match in the buffer as you
type. In replace mode, **Replace** changes the active occurrence and advances
to the next one, while **All** replaces every occurrence. These buttons are
mouse-only; `<enter>` always advances to the next match. The input fields use
Standard editing behavior, including drag selection and double-click word
selection. Search is case-sensitive, does not match across line boundaries, and
wraps from the last match to the first. Rune starts from the caret and retains
the last query for the next search. Closing an empty search restores the caret
to its starting position instead of leaving a match selected.

### Search configuration

Configure the widget under `editor.standard.search`. The examples show the
default keys and visual attributes:

```yaml tab
editor:
  standard:
    search:
      find_key: "<meta-f>"
      replace_key: "<meta-r>"
      attr: { fg: default, bg: default }
      input_attr: { fg: default, bg: default }
      placeholder_attr: { fg: gray, bg: default }
      frame_attr: { fg: gray, bg: default }
      focus_frame_attr: { fg: silver, bg: default }
      button_attr: { fg: default, bg: gray }
      button_hover_attr: { fg: default, bg: blue }
      match_attr: { fg: grey, bg: yellow }
      current_match_attr: { fg: default, bg: default }
```

```python tab
config["editor"]["standard"]["search"] = {
    "find_key": "<meta-f>",
    "replace_key": "<meta-r>",
    "attr": attr(fg = "default", bg = "default"),
    "input_attr": attr(fg = "default", bg = "default"),
    "placeholder_attr": attr(fg = "gray", bg = "default"),
    "frame_attr": attr(fg = "gray", bg = "default"),
    "focus_frame_attr": attr(fg = "silver", bg = "default"),
    "button_attr": attr(fg = "default", bg = "gray"),
    "button_hover_attr": attr(fg = "default", bg = "blue"),
    "match_attr": attr(fg = "grey", bg = "yellow"),
    "current_match_attr": attr(fg = "default", bg = "default"),
}
```

`attr` styles the widget background. The input, placeholder, frame, focused
frame, button, hovered button, inactive match, and active match can each be
styled independently with their corresponding keys. Attribute colors accept
Rune's named colors and hex values. The optional `status_attr` styles the
legacy status-bar Find prompt used only if Rune cannot open the floating Find
window; it is unset by default.

## Language intelligence, diagnostics, and Git changes

In-file navigation uses a `<ctrl-meta>` IJKL layer. The vertical `i` / `k`
pair visits Git changes, while the horizontal `j` / `l` pair visits
diagnostics:

| Action | Key |
| --- | --- |
| Previous Git change | `<ctrl-meta-i>` |
| Next Git change | `<ctrl-meta-k>` |
| Previous diagnostic | `<ctrl-meta-j>` |
| Next diagnostic | `<ctrl-meta-l>` |

Everything else the language server offers sits on a plain `<alt>` letter.
Each binding acts on the symbol under the caret; adding `<shift>` opens the
Command Prompt with that command ready for a symbol name instead:

| Action | Key | By name |
| --- | --- | --- |
| Go to definition | `<alt-d>` | `<alt-shift-d>` |
| Find references | `<alt-r>` | `<alt-shift-r>` |
| Find implementations | `<alt-p>` | `<alt-shift-p>` |
| Show hover and type information | `<alt-t>` | `<alt-shift-t>` |
| Go to declaration | `<alt-c>` | `<alt-shift-c>` |
| Go to type definition | `<alt-y>` | `<alt-shift-y>` |
| Show signature help | `<alt-g>` | |
| Format the buffer | `<alt-b>` | |
| List diagnostics | `<alt-e>` | |
| Complete at the caret | `<ctrl-space>` | |

Three more `<alt>` letters jump to a symbol declared in the current file by
name, using the language's tree-sitter `locals.scm` queries rather than the
language server:

| Action | Key |
| --- | --- |
| Jump to a function or method | `<alt-f>` |
| Jump to a type | `<alt-s>` |
| Jump to a variable | `<alt-x>` |

Symbol rename is available through the
[Command Prompt](./command-prompt.md) as `lsp rename`.

## Folds

| Key | Action |
| --- | --- |
| `<ctrl-{>` | collapse the fold at the caret |
| `<ctrl-}>` | expand the fold at the caret |
| `<ctrl-shift-a>` | toggle all folds |
| `<alt-meta-[>` / `<alt-meta-]>` | collapse / expand the fold at the caret |
| `<meta-k><meta-j>` | expand all folds |
| `<meta-k><meta-1>` | collapse all folds |

## Hidden lines

| Key | Action |
| --- | --- |
| `<ctrl-alt-h>` | hide the lines covered by the selection |
| `<ctrl-alt-v>` | reveal the hidden block at the caret |

## Macros

| Key | Action |
| --- | --- |
| `<ctrl-q>` | start or stop recording to the unnamed register |
| `<ctrl-shift-q>` | play the recorded macro |

## Legacy `<meta-k>` prefix

The standard editor retains a one-shot `<meta-k>` compatibility layer for a
small set of advanced operations:

| Sequence | Action |
| --- | --- |
| `<meta-k><meta-space>` | set mark at the caret |
| `<meta-k><meta-a>` | select from mark to caret |
| `<meta-k><meta-w>` | delete from mark to caret |
| `<meta-k><meta-x>` | swap caret and mark |
| `<meta-k><meta-g>` | clear mark |
| `<meta-k><meta-u>` | uppercase selection |
| `<meta-k><meta-l>` | lowercase selection |
| `<meta-k><meta-k>` | delete to end of line |
| `<meta-k><meta-backspace>` | delete to start of line |
| `<meta-k><meta-j>` | expand all folds |
| `<meta-k><meta-1>` | collapse all folds |

This layer is optional compatibility, not the primary standard keymap. On
Linux, normal cut remains `<ctrl-x>`.

## Auto-pair and paste

When `auto_pair` is enabled, typing `(`, `[`, `{`, `"`, or `'` inserts the
matching close. Typing a supported closer immediately before the same existing
character moves over it instead of inserting a duplicate. `<backspace>` inside
a supported empty pair removes both characters. `<enter>` between `{}` splits
the braces across a blank line; other pairs receive a normal indented newline.

When no selection is active, bracketed terminal paste is inserted verbatim as
one text run. It does not add auto-paired closers or rewrite indentation.

## Command prompt and configuration

| Action | macOS | Linux |
| --- | --- | --- |
| Open the command prompt | `<shift-meta-p>` | `<ctrl-shift-p>` |
| Open Rune's configuration | `<meta-,>` | `<ctrl-,>` or `<meta-,>` |

See [Command Prompt](./command-prompt.md) for commands, aliases, completion,
and custom bindings.

## What's not here

The standard editor does not currently implement:

- Multi-cursor editing.
- An Emacs `<ctrl-x>` prefix layer.
- An editor-owned completion popup. Completion lives in language extensions;
  the Standard preset invokes it with `<ctrl-space>` when available.
- A dedicated goto-line key. Use the [Command Prompt](./command-prompt.md).

For complete Vim, Neovim, Helix, Emacs, or another editor's behavior, use
[Exoeditor](./exoeditor.md).
