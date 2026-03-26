---
name: code-refactoring
description: Rename symbols across the workspace, discover available code actions and quick fixes, and format files or ranges using the language server. Use instead of find-and-replace when renaming a function, variable, or type. Activated by runectl lsp rename, code-actions, or formatting.
allowed-tools: Bash(runectl:*)
---

# Code Refactoring

Language-aware refactoring via `runectl lsp`.

## runectl lsp prepare-rename

Prepare for rename operation — check if rename is valid.

```
Usage:
  runectl lsp prepare-rename (<file> <line> <col> | <symbol>) [flags]

Flags:
  -F, --format string   Output format: table, json, or Go template
```

Example — check if `bashTool` can be renamed:

```bash
$ runectl lsp prepare-rename bashTool
44:5-44:13 "bashTool"
```

Returns the symbol's range and current name. Always call this
before `rename` to confirm the symbol is renameable.

## runectl lsp rename

Rename a symbol across the workspace, updating all references.

```
Usage:
  runectl lsp rename (<file> <line> <col> <new-name> | <symbol> <new-name>) [flags]

Flags:
      --dry-run         Preview changes without applying
  -F, --format string   Output format: table, json, or Go template
      --no-color        Disable colored diff output
```

Example — preview renaming `bashTool` to `bashExecutor`:

```bash
$ runectl lsp rename --dry-run bashTool bashExecutor
file:///src/rune-agent/agent/agentools/bash.go 5
```

The dry run shows 5 edits would be made in `bash.go`. Without
`--dry-run`, the rename is applied immediately. Always use
`--dry-run` first to preview the scope of changes.

## runectl lsp code-actions

List code actions at a position — refactorings and quick fixes.

```
Usage:
  runectl lsp code-actions <file> <line> <col> [flags]

Available Commands:
  commands    List code action commands at a position
  edits       List code action edits at a position

Flags:
  -F, --format string   Output format: table, json, or Go template
```

Example — discover actions at line 114 of `bash.go`:

```bash
$ runectl lsp code-actions agent/agentools/bash.go 114 5
[source.addTest] "Add test for Execute"
[source.assembly] "Browse arm64 assembly for Execute"
[source.doc] "Browse documentation for package agentools"
[source.splitPackage] "Split package \"agentools\""
...
```

Each action shows `[kind] "title"`. Use `code-actions edits`
to get the actual edits an action would apply.

## runectl lsp formatting

Format an entire document.

```
Usage:
  runectl lsp formatting <file> [flags]

Flags:
      --dry-run           Preview changes without applying
  -F, --format string     Output format: table, json, or Go template
      --insert-spaces     Use spaces instead of tabs (default true)
      --no-color          Disable colored diff output
      --tab-size uint32   Tab size for formatting (default 4)
```

Use instead of `go fmt`. Supports `--dry-run` to preview
changes before applying.

## runectl lsp range-formatting

Format a specific range within a document.

```
Usage:
  runectl lsp range-formatting <file> <start-line> <start-col> <end-line> <end-col> [flags]

Flags:
      --dry-run           Preview changes without applying
  -F, --format string     Output format: table, json, or Go template
      --insert-spaces     Use spaces instead of tabs (default true)
      --no-color          Disable colored diff output
      --tab-size uint32   Tab size for formatting (default 4)
```

Use to format just a section of code. Same flags as `formatting`.
