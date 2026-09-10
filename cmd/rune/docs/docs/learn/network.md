---
sidebar_position: 33
---

# Network

Every machine you run Rune on can join one private, encrypted network of
your own. Once two of them are on it, either one can open a workspace on
the other:

```
workspaceopen rune://<machine>/path/to/project
```

There is no SSH server to run, no port to forward, no public address to
expose, and no credentials to hand out. The machines find each other
through Rune's coordination server and talk directly, so a laptop behind
a home router and a workstation behind an office firewall reach each
other without either being reachable from the internet.

A `rune://` workspace behaves exactly like an [SSH
workspace](./ssh.md) once it is open: files, terminals, language
intelligence, tasks, and the agent all run on the other machine, next to
the code.

## What you need

- **A Rune plan that includes the network.** It is a paid feature. Run
  `login` in the [Rune console](./console.md) to sign in with the
  account that holds your plan. When your account does not cover the
  network, Rune offers to sign you in, sign you up, or upgrade. After
  upgrading, run `login` again so this machine picks up the new plan.
- **Rune running on both machines, signed into the same account.** A
  machine serves its workspaces from the Rune instance running on it. If
  Rune is not open there, the machine is not reachable, and only your own
  machines are ever allowed to connect.

## Joining the network

Rune joins on startup, so usually there is nothing to do. Check where
this machine stands with `network status` in the console:

```
network status
```

It reports the name this machine is known by, its state (`Running` once
it has joined), its addresses on the network, the account it belongs to,
and the last error if it could not join. While a sign-in is pending it
prints a URL to open in a browser to authorize the machine.

To join without restarting, or after a `network down`, run `network up`.
It waits until the machine is a member and then prints the status. If
joining takes longer than 30 seconds, Rune tells you so and keeps trying
in the background; run `network status` to see how it went.

### The `network` command

`network` is a [Rune console](./console.md) command: open the console and
run it as `network <subcommand>`, or submit a one-off from the
[command prompt](./command-prompt.md) with `console network <subcommand>`.

| Command | What it does |
| --- | --- |
| `network status` | Show this machine's name, addresses, and sign-in state, including the last error and any pending authorization URL. |
| `network peers` | List the other machines on the network, with the `rune://` address each one is reachable at. |
| `network up` | Join the network and wait until this machine is a member. |
| `network down` | Leave the network. Open `rune://` workspaces stop working until you run `network up` again. |
| `network help` | Show the command's usage. |

## Opening a workspace on another machine

`network peers` lists what you can open:

```
network peers
```

| machine | workspace | os | state |
| --- | --- | --- | --- |
| carbon | `rune://carbon/` | linux | online |
| studio | `rune://studio/` | macOS | offline |

Open one with `workspaceopen` from the
[command prompt](./command-prompt.md):

```
workspaceopen rune://carbon/src/project
```

The parts of the URL are:

- `carbon` is the machine's name on the network, as shown by
  `network peers`. Typing `rune://` at the `workspaceopen` prompt
  completes it for you.
- `/src/project` is the directory to open on that machine. Any path you
  could open locally there works. Leave it out (`rune://carbon/`) to open
  that machine's home directory.

A `rune://` URL takes no user name: machines authenticate by their own
identity, not by an account on the other end, so `rune://user@carbon/`
is rejected.

Offline machines are listed and completed too. Opening one waits and
connects as soon as it comes back, which is also what happens when a
machine sleeps or changes networks mid-session: Rune reconnects on its
own and the workspace carries on. Two failures are final rather than
retried, because retrying cannot fix them: a machine that is not on the
network at all, and one that belongs to a different account.

## Configuration

The network reads its settings from the `network` section of your
[Rune config](../config.md). The defaults suit most machines.

```yaml tab
network:
  auto_join: true
  hostname: ""
  port: 7473
```

```python tab
config["network"] = {
    "auto_join": True,
    "hostname": "",
    "port": 7473,
}
```

| Key | Type | Default | Effect |
| --- | --- | --- | --- |
| `auto_join` | bool | `true` | Join the network when Rune starts. With it off, the network stays one `network up` away, with no restart needed. |
| `hostname` | string | the machine's own hostname | The name this machine advertises, and the name used in its `rune://` addresses. |
| `port` | int | `7473` | The port workspaces are served on. It is reachable only from the network, never from the machine's other interfaces. |

Names are made unique when a machine joins, so a second machine with the
same hostname is given a suffixed name. `network peers` always shows the
exact name to put in a `rune://` URL.

## How it works

Rune embeds a userspace [WireGuard](https://www.wireguard.com/) node in
its own process. There is no daemon to install, nothing runs as root,
and the rest of your system's traffic is untouched: only Rune uses the
network.

When a machine joins, Rune's coordination server checks your account,
hands the machine short-lived credentials, and tells your machines about
each other. Your files and terminals do not travel through it. Traffic
goes directly between the two machines, encrypted end to end, and falls
back to an encrypted relay only when no direct path can be established.

Each machine serves its workspaces on port 7473 of its network address
only. Every request carries the identity of the machine behind it, and
Rune refuses any whose owning account is not yours, so sharing a network
is never enough to read a machine's files or run commands on it.

A machine's network identity is stored in your Rune data directory,
under `~/.rune/runenet`. `network down` leaves the network but keeps that
identity, so `network up` rejoins without a new sign-in.

## `rune://` and `ssh://`

Both schemes open a directory on another machine and behave the same way
once open. They differ in what they need from you:

| | [`ssh://`](./ssh.md) | `rune://` |
| --- | --- | --- |
| Addressed by | user, host, and port | machine name |
| Reachability | the host must be reachable over SSH | neither machine needs to be reachable from outside |
| Credentials | your SSH keys and `known_hosts` | your Rune account, nothing to manage |
| The other end runs | a workspace server that Rune starts over SSH | the Rune instance already running there |
| Toolchains | Rune mirrors your local language packages onto the host | the machine's own Rune install and packages |

Use `ssh://` for machines that are not yours, or that do not run Rune.
Use `rune://` for your own machines.

## Troubleshooting

**"The network is part of a paid Rune plan."** The account signed in on
this machine does not cover the network. Choose **Sign in** or **Sign
up** if you have not signed in, or **Upgrade** to add it to your plan.
After upgrading, run `login` again so this machine picks up the change.

**A machine is missing from `network peers`.** Check that Rune is
running on it, that `network status` there reports state `Running`, and
that both machines are signed into the same account.

**`network status` prints an authorization URL.** The machine is waiting
to be authorized. Open the URL in a browser and it finishes joining.

**"peer not found in network."** The name in the `rune://` URL is not a
machine on your network. Use the name `network peers` reports, which is
the one the coordination server assigned and may differ from the
machine's local hostname.

**The connection is refused.** The other machine belongs to a different
account. Sign both machines into the same account.

**"not a member after 30s."** Joining is still in progress in the
background. Run `network status` to see whether it succeeded, and read
its `error` field if it did not.

**Workspaces stopped working after `network down`.** Leaving the network
disconnects open `rune://` workspaces. Run `network up` to rejoin.
