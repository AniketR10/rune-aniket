# Unstable Build LLC ("COMPANY") CONFIDENTIAL
#
# Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
#
# NOTICE: All information contained herein is, and remains the property of COMPANY.
# The intellectual and technical concepts contained herein are proprietary to
# COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
# and are protected by trade secret or copyright law. Dissemination of this information
# or reproduction of this material is strictly forbidden unless prior written permission
# is obtained from COMPANY. Access to the source code contained herein is hereby
# forbidden to anyone except current COMPANY employees, managers or contractors who
# have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
#
# The copyright notice above does not evidence any actual or intended publication or
# disclosure of this source code, which includes information that is confidential and/or
# proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
# DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
# WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
# VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
# THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
# REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
# ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

# agent.star — the Rune Agent tutorial.

ck = command_key()

if editor_mode() == "modal":
    shell_esc_step = "1. Press `<esc>` to enter modal mode.\n"
    shell_prompt_step_num = "2"
    shell_run_step_num = "3"
    esc_allow_keys = ["<esc>"]
else:
    shell_esc_step = ""
    shell_prompt_step_num = "1"
    shell_run_step_num = "2"
    esc_allow_keys = []

def keypress(cmd, *args):
    k = key_for(cmd, *args)
    if k:
        return "press `" + k + "`"
    return "open the command prompt (`" + ck + "`) and run `" + cmd + "`"

def dismiss_for(cmd, *args):
    k = key_for(cmd, *args)
    return [ck, k] if k else [ck]

cleanup_md = """\
Let's start fresh. Clear the layout: """ + keypress("windowcloseall") + """.
"""

agent_install_md = """\
The **Rune Agent** is Rune's builtin AI coding assistant. It ships as a
package you install on demand, so the first step is to install it.

Packages are installed from the **Rune console**, which is not the
command prompt you have been using. The prompt (`""" + ck + """`) is the
one-line prompt that closes again as soon as the command runs. The
console is a separate, durable tab with its own REPL, wired with the
commands that want a persistent output window, like installing a package
or checking an extension's status.

The console **sets up** Rune, the prompt **drives** it.

Open the console now:

1. Press `""" + ck + """` to open the command prompt.
2. Type `console` and press Enter.
"""

agent_pkg_install_md = """\
You're in Rune's console now. Install the agent package:

1. Type `pkg install rune-agent`.
2. Press Enter and wait for the install to finish.

Press `<enter>` or `<space>` to continue.
"""

agent_open_md = """\
Start a conversation with the agent using the `agent` command.

""" + shell_esc_step + shell_prompt_step_num + """. Press `""" + ck + """` to open the command prompt.
""" + shell_run_step_num + """. Run `agent`.

`agent` takes two optional arguments: a conversation name and a model.
Run `agent <name>` to name the conversation, or `agent <name> <model>`
to also pick the model. With no arguments, the agent starts a new
conversation using your default provider.

Press `<enter>` or `<space>` to continue.
"""

help_md = """\
You're almost done. A few tips worth remembering:

- If you find yourself wondering what commands you typed on a previous session, press
  `<meta-r>` to open the command prompt in history mode and search through your command history.
- There's a `help` command that opens the documentation on a separate workspace
  and fires a help agent that you can ask questions.

Try it now: """ + keypress("help") + """. Happy hacking!
"""

def teach_provider(provider, label, action_tokens, run_md, success_msg):
    title = "Connect " + label
    floating_window(title = title, text = run_md, alignment = "top")
    wait_shell(
        title    = title,
        args     = ["models", "providers", provider] + action_tokens,
        on_error = ("In Rune's console, run `models providers " +
                    provider + " " + " ".join(action_tokens) + "`."),
    )
    notify(level = success, message = success_msg)

def teach_cleanup():
    floating_window(title = "Clear the layout", text = cleanup_md,
                    dismiss_keys = dismiss_for("windowcloseall"))
    wait_command(
        title    = "Clear the layout",
        command  = "windowcloseall",
        on_error = "Close every window except the focused one with `<cmd>windowcloseall`.",
    )
    notify(level = success, message = "Layout cleared.")

def teach_agent():
    floating_window(title = "Set up the Rune Agent", text = agent_install_md,
                    dismiss_keys = [ck])
    wait_command(
        title    = "Set up the Rune Agent",
        command  = "console",
        on_error = "Open Rune's console: run the `<cmd>console` command.",
    )

    floating_window(title = "Install the agent package", text = agent_pkg_install_md,
                    alignment = "top")
    wait_shell(
        title    = "Install the agent package",
        args     = ["pkg", "install", "rune-agent"],
        on_error = "In Rune's console, run `pkg install rune-agent`.",
    )
    notify(level = success, message = "Rune Agent installed.")

    pick = choice(
        message = ("Which provider do you want to connect?\n\n" +
                   "- **OpenAI**: GPT family, billed by API key.\n" +
                   "- **Anthropic**: Claude family, billed by API key.\n" +
                   "- **Gemini**: Gemini family, billed by API key.\n" +
                   "- **Codex**: GPT-5 Codex family. Sign in with " +
                   "ChatGPT (OAuth) and use your ChatGPT subscription, " +
                   "no API key.\n" +
                   "- **Claude**: Claude family through your Claude " +
                   "Pro/Max subscription. Sign in with Claude (OAuth), " +
                   "no API key."),
        options = ["OpenAI", "Anthropic", "Gemini", "Codex", "Claude"],
    )
    if not pick.selected:
        notify(level = info,
               message = ("Configure a provider any time with the " +
                          "`models providers` console command."))
        return

    if pick.value == "OpenAI":
        run_md = """\
Add your OpenAI credentials. The key is stored securely and never
written to your config file.

1. In Rune's console, run `models providers openai add default`.
2. Paste your API key at the redacted prompt.

Press `<enter>` or `<space>` to continue.
"""
        teach_provider("openai", "OpenAI", ["add", "default"], run_md,
                       "OpenAI connected.")
    elif pick.value == "Anthropic":
        run_md = """\
Add your Anthropic credentials. The key is stored securely and never
written to your config file.

1. In Rune's console, run `models providers anthropic add default`.
2. Paste your API key at the redacted prompt.

Press `<enter>` or `<space>` to continue.
"""
        teach_provider("anthropic", "Anthropic", ["add", "default"], run_md,
                       "Anthropic connected.")
    elif pick.value == "Gemini":
        run_md = """\
Add your Gemini credentials. The key is stored securely and never
written to your config file.

1. In Rune's console, run `models providers gemini add default`.
2. Paste your API key at the redacted prompt.

Press `<enter>` or `<space>` to continue.
"""
        teach_provider("gemini", "Gemini", ["add", "default"], run_md,
                       "Gemini connected.")
    elif pick.value == "Codex":
        run_md = """\
Codex authenticates through your browser. No API key is needed.

1. In Rune's console, run `models providers codex login`.
2. Finish the sign-in in your browser.

Press `<enter>` or `<space>` to continue.
"""
        teach_provider("codex", "Codex", ["login"], run_md, "Codex connected.")
    else:
        run_md = """\
Claude signs in through your browser with your Claude Pro or Max
subscription. There is no API key to paste.

Agent activity through this provider draws from your Claude plan's
separate monthly Agent SDK credit, not your interactive usage limits.
Once that credit runs out, further usage bills at standard API rates if
you have usage credits enabled, and otherwise pauses until the credit
refreshes. The separate `anthropic` provider bills the same models by
API key instead.

1. In Rune's console, run `models providers claude login`.
2. Finish the sign-in in your browser.

Press `<enter>` or `<space>` to continue.
"""
        teach_provider("claude", "Claude", ["login"], run_md, "Claude connected.")

    floating_window(title = "Open the Rune Agent", text = agent_open_md,
                    allow_keys = esc_allow_keys,
                    dismiss_keys = [ck])
    wait_command(
        title    = "Open the Rune Agent",
        command  = "agent",
        on_error = ("Run `<cmd>agent` to start a conversation. Add an " +
                    "optional conversation name and model: " +
                    "`<cmd>agent <name> <model>`."),
    )
    notify(level = success, message = "Rune Agent is ready.")

def teach_help():
    floating_window(title = "One last thing", text = help_md,
                    dismiss_keys = dismiss_for("help"))
    wait_command(
        title    = "One last thing",
        command  = "help",
        on_error = "Run the `<cmd>help` command to open the docs and ask the help agent.",
    )
    notify(level = success, message = "That is the help command.")

def run():
    teach_cleanup()
    teach_agent()
    teach_help()

tutorial(id = "agent", title = "Rune Agent", version = "4", entry = run)
