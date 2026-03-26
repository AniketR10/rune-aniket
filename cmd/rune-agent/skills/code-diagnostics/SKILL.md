---
name: code-diagnostics
description: Get compilation errors, warnings, and linter diagnostics from the language server for a single file or the entire workspace. Use instead of go build or go vet to check for errors after editing code. Activated by runectl lsp diagnostics.
allowed-tools: Bash(runectl:*)
---

# Code Diagnostics

Check for errors via the language server.

## runectl lsp diagnostics

Get diagnostics for a file.

```
Usage:
  runectl lsp diagnostics <file> [flags]

Flags:
  -F, --format string   Output format: table, json, or Go template
```

Example — check `bash.go` for issues:

```bash
$ runectl lsp diagnostics agent/agentools/bash.go
[Hint] 170:3: Replace []byte(fmt.Sprintf...) with fmt.Appendf (fmtappendf, default)
[Hint] 128:5: if statement can be modernized using min (minmax, default)
```

Each diagnostic shows `[Severity] line:col: message (source, code)`.
Severity is `[Error]`, `[Warning]`, `[Information]`, or `[Hint]`.

## runectl lsp workspace-diagnostics

Get diagnostics for the entire workspace.

```
Usage:
  runectl lsp workspace-diagnostics [flags]

Flags:
  -F, --format string   Output format: table, json, or Go template
```

Use to check the whole project for issues at once. Same output
format as `diagnostics` but prefixed with the file URI.
