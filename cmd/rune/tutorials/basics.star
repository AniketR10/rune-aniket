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

agent_install_md = """\
# Set up Rune Agent

The **Rune Agent** is an in-editor AI coding assistant. It ships as a
package you install on demand, so the first step is to add it.

First, open Rune's companion shell:

1. Press `""" + ck + """` to open the command prompt.
2. Type `shell` and press Enter.

Press `<enter>` or `<space>` to continue.
"""

agent_pkg_install_md = """\
# Install the agent package

You're in the companion shell now. Install the agent package:

1. Type `pkg install rune-agent`.
2. Press Enter and wait for the install to finish.

Press `<enter>` or `<space>` to continue.
"""

agent_open_md = """\
# Open Rune Agent

Start a conversation with the agent using the `agent` command.

1. Press `""" + ck + """` to open the command prompt.
2. Run `agent`.

`agent` takes two optional arguments: a conversation name and a model.
Run `agent <name>` to name the conversation, or `agent <name> <model>`
to also pick the model. With no arguments, the agent starts a new
conversation using your default provider.

Press `<enter>` or `<space>` to continue.
"""

def teach_edit():
    floating_window(title = "Open a file", text = edit_md, dismiss_keys = [ck])
    edit = wait_command(
        command  = "edit",
        on_error = ("`<cmd>edit` needs a `<file>` argument. Use the " +
                    "auto-completer (Tab / arrow keys) to pick a " +
                    "file, or type a path inside the workspace; " +
                    "relative paths resolve against the workspace root."),
    )
    notify(level = success, message = "You opened " + edit.args[0])


def teach_provider(provider, action_tokens, run_md, success_msg):
    floating_window(title = "Connect a provider", text = run_md)
    wait_shell(
        args     = ["models", "providers", provider] + action_tokens,
        on_error = ("In the companion shell, run `models providers " +
                    provider + " " + " ".join(action_tokens) + "`."),
    )
    notify(level = success, message = success_msg)


def teach_agent():
    floating_window(title = "Set up the Rune Agent", text = agent_install_md,
                    dismiss_keys = [ck])
    wait_command(
        command  = "shell",
        on_error = ("Open Rune's companion shell: run the `<cmd>shell` " +
                    "command."),
    )

    floating_window(title = "Install the agent package", text = agent_pkg_install_md)
    wait_shell(
        args     = ["pkg", "install", "rune-agent"],
        on_error = "In the companion shell, run `pkg install rune-agent`.",
    )
    notify(level = success, message = "Rune Agent installed.")

    pick = choice(
        message = ("Which provider do you want to connect?\n\n" +
                   "- **OpenAI** — GPT family, billed by API key.\n" +
                   "- **Anthropic** — Claude family, billed by API key.\n" +
                   "- **Gemini** — Gemini family, billed by API key.\n" +
                   "- **Codex** — GPT-5 Codex family. Sign in with " +
                   "ChatGPT (OAuth) and use your ChatGPT subscription, " +
                   "no API key.\n" +
                   "- **Claude** — Claude family through your Claude " +
                   "Pro/Max subscription. Sign in with Claude (OAuth), " +
                   "no API key."),
        options = ["OpenAI", "Anthropic", "Gemini", "Codex", "Claude", "Skip"],
    )
    if not pick.selected or pick.value == "Skip":
        notify(level = info,
               message = ("Configure a provider any time with the " +
                          "`models providers` shell command."))
        return

    if pick.value == "OpenAI":
        run_md = """\
# Connect OpenAI

Add your OpenAI credentials. The key is stored securely and never
written to your config file.

1. In the companion shell, run `models providers openai add default`.
2. Paste your API key at the redacted prompt.

Press `<enter>` or `<space>` to continue.
"""
        teach_provider("openai", ["add", "default"], run_md,
                       "OpenAI connected.")
    elif pick.value == "Anthropic":
        run_md = """\
# Connect Anthropic

Add your Anthropic credentials. The key is stored securely and never
written to your config file.

1. In the companion shell, run `models providers anthropic add default`.
2. Paste your API key at the redacted prompt.

Press `<enter>` or `<space>` to continue.
"""
        teach_provider("anthropic", ["add", "default"], run_md,
                       "Anthropic connected.")
    elif pick.value == "Gemini":
        run_md = """\
# Connect Gemini

Add your Gemini credentials. The key is stored securely and never
written to your config file.

1. In the companion shell, run `models providers gemini add default`.
2. Paste your API key at the redacted prompt.

Press `<enter>` or `<space>` to continue.
"""
        teach_provider("gemini", ["add", "default"], run_md,
                       "Gemini connected.")
    elif pick.value == "Codex":
        run_md = """\
# Connect Codex

Codex authenticates through your browser — no API key to paste.

1. In the companion shell, run `models providers codex login`.
2. Finish the sign-in in your browser.

Press `<enter>` or `<space>` to continue.
"""
        teach_provider("codex", ["login"], run_md, "Codex connected.")
    else:
        run_md = """\
# Connect Claude

Claude signs in through your browser with your Claude Pro or Max
subscription. There is no API key to paste.

Agent activity through this provider draws from your Claude plan's
separate monthly Agent SDK credit, not your interactive usage limits.
Once that credit runs out, further usage bills at standard API rates if
you have usage credits enabled, and otherwise pauses until the credit
refreshes. The separate `anthropic` provider bills the same models by
API key instead.

1. In the companion shell, run `models providers claude login`.
2. Finish the sign-in in your browser.

Press `<enter>` or `<space>` to continue.
"""
        teach_provider("claude", ["login"], run_md, "Claude connected.")

    floating_window(title = "Open the Rune Agent", text = agent_open_md,
                    dismiss_keys = [ck])
    wait_command(
        command  = "agent",
        on_error = ("Run `<cmd>agent` to start a conversation. Add an " +
                    "optional conversation name and model: " +
                    "`<cmd>agent <name> <model>`."),
    )
    notify(level = success, message = "Rune Agent is ready.")


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

    teach_edit()

    if not confirm("Want to set up the Rune Agent now?"):
        notify(level = info,
               message = "Run `<cmd>tutorial start basics` any time to continue.")
        return

    teach_agent()


tutorial(id = "basics", title = "Rune basics", version = "3", entry = run)
