---
sidebar_position: 6
---

# Zig

Rune ships first-class Zig support: code intelligence through
[zls](https://zigtools.org/), build-on-save diagnostics powered by the
Zig build system, and a dedicated [Rune console](../learn/console.md)
command for driving the `zig` toolchain from the editor.

## Setup

There is no setup. The Zig extension locates `zls` and `zig` on the
workspace host automatically, checking the bundled install first and
falling back to well-known locations and your shell's PATH.

If you want to use your own binaries, point Rune at them in
`config.yaml`:

```yaml tab
extensions:
  zig:
    config:
      lsp_path: "/path/to/zls"
      zig_path: "/path/to/zig"
```

## How Rune finds your Zig project

Zig projects are built by the Zig build system. A project's root is the
folder that contains its `build.zig` build script (and, for packages,
the `build.zig.zon` manifest). If a project does not have one yet,
running `zig init` in a terminal at the project root creates both.

Rune uses those files to decide where a Zig project starts and to root
the language server there:

- **Workspace root.** If the folder you open as your workspace contains
  a `build.zig` or `build.zig.zon`, or simply has `.zig` files at the
  top level, the language server starts right away, rooted at the
  workspace.
- **Nested projects.** A `build.zig` deeper in the workspace works too.
  When you open a `.zig` file, Rune walks up from that file to the
  nearest enclosing folder with a `build.zig` or `build.zig.zon` and
  starts a language server rooted at that folder. In a repository with
  several Zig projects, each project gets its own language server the
  first time you open one of its files, and that server is reused for
  every other file in the project.

A `.zig` file with no `build.zig` in any folder between it and the
workspace root gets no code intelligence, because there is no project
to root a server at. Run `zig init` at the project root and reopen the
file.

Working across several projects in one repository? See the
[Monorepos](./monorepo.md) guide for how per-project discovery and
workspace-wide search fit together.

## Code intelligence: the `lsp` command

The cross-language `lsp` command drives code intelligence for Zig the
same way it works everywhere: hover, definition, references,
completion, rename, formatting, and diagnostics, all with the same
default key bindings. See
[the `lsp` command](./intelligence.md#code-intelligence-the-lsp-command)
for the full table. zls does not implement go-to-implementation, code
lenses, range formatting, or call hierarchy; those `lsp` subcommands
report no results for Zig files.

### Language server logs

Rune starts zls with stderr logging enabled and captures the output. Run
`process status` in the [Rune console](../learn/console.md#managing-processes-with-process)
to find the zls PID, then run
`process stdio <pid>` to view its logs. This also works after zls exits; use
`process audit` to find its previous PID. See
[Troubleshoot](../troubleshoot.md#inspect-any-process-rune-starts)
for details.

The default log level is `info`. Configure it with `debug.log_level`:

```yaml tab
extensions:
  zig:
    config:
      debug:
        log_level: debug
```

Accepted values are `err`, `warn`, `info`, and `debug`.

## Build-on-save diagnostics

Rune sends document changes and saves to zls for every Zig file. zls uses those
notifications to surface syntax and semantic errors from its own analysis as
you type. This immediate diagnostic path is always active and does not depend
on a build step.

Some compiler errors require the complete project, including type errors across
files. For those, zls can additionally invoke the Zig build system after a save
and report the compiler's results as diagnostics. This is the separate
**build-on-save** path.

Build-on-save turns on automatically when your `build.zig` declares a
step named `check`. The conventional shape compiles your code without
installing anything:

```zig
const exe_check = b.addExecutable(.{
    .name = "check",
    .root_module = b.createModule(.{
        .root_source_file = b.path("src/main.zig"),
        .target = target,
        .optimize = optimize,
    }),
});
const check = b.step("check", "Typecheck the project");
check.dependOn(&exe_check.step);
```

When your `build.zig` has no `check` step, ordinary zls diagnostics continue to
work. Rune posts a one-time hint explaining only that the additional
full-project compiler pass is inactive. You can add the conventional `check`
step or control that compiler pass explicitly:

```yaml tab
extensions:
  zig:
    config:
      build_on_save: true
      build_on_save_args: ["-Dtarget=wasm32-wasi"]
```

`build_on_save: true` runs the build after saves even without a `check`
step, falling back to the project's `install` step; `false` turns the feature
off. `build_on_save_args` customizes the arguments passed to `zig build`.

## Driving the toolchain: the console `zig` command

The `zig` command drives the Zig toolchain from the editor. It is a
[Rune console](../learn/console.md) command: open the console and run
it as `zig <subcommand>`, or submit a one-off from the
[command prompt](../learn/command-prompt.md) with
`console zig <subcommand>`. Arguments after the subcommand are passed
through to `zig`:

| Command | What it does |
| --- | --- |
| `zig build [<step>] [<args>]` | Build the project from `build.zig`. |
| `zig test [<file>] [<args>]` | Run the project's tests. |
| `zig run [<file>] [<args>]` | Build and run an executable. |
| `zig fmt [<paths>]` | Format Zig sources in place. |
| `zig version` | Show the zig compiler version. |
| `zig reload` | Restart the language server. |

`zig reload` reinitializes every language server the workspace has
brought up, for toolchain changes made outside the editor.

## Debugging

Zig compiles to native code with DWARF debug info, so Rune's
LLDB-based `lldb-dap` debug adapter works out of the box: breakpoints,
stepping, stack traces, and variable inspection behave the same as for
C or Rust binaries. Build with a debug-friendly mode (the default
`Debug` optimize mode) and point a launch configuration at the emitted
binary. See the [Debugger](../learn/debugger.md) guide for how debug
sessions work and how to register an adapter.
