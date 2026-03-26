---
name: code-search
description: Structural code search using tree-sitter queries — find AST patterns like specific function calls, composite literals, or node shapes across the workspace or in a single file. More precise than regex. Use when grep is too imprecise for the pattern. Activated by runectl syntax search/query.
allowed-tools: Bash(runectl:*)
---

# Code Search

Tree-sitter structural search via `runectl syntax`.

## runectl syntax search

Search workspace using a tree-sitter query.

```
Usage:
  runectl syntax search <query> [flags]

Flags:
  -c, --capture stringArray   Capture name filter (repeatable)
  -F, --format string         Output format: table, json, or Go template
  -L, --lang stringArray      Language filter (repeatable, e.g. go, python)
```

For simple "find all functions" queries, prefer `runectl syntax
searchnode` from the code-structure skill instead.

Example — find all struct type definitions in Go files:

```bash
$ runectl syntax search '(type_declaration (type_spec name: (type_identifier) @name type: (struct_type)))' -L go
file:///src/rune-agent/agent/agent.go Config 44:5-44:11 name
file:///src/rune-agent/agent/agent.go Agent 72:5-72:10 name
file:///src/rune-agent/dialogue/dialoguetui/cell_grid.go cellGrid 62:5-62:13 name
...
```

Each line shows `file text line:col-endLine:endCol capture_name`.
The `@name` capture matched the type identifier.

## runectl syntax query

Query a single file using a tree-sitter query.

```
Usage:
  runectl syntax query <file> <query> [flags]

Flags:
  -c, --capture stringArray   Capture name filter (repeatable)
  -F, --format string         Output format: table, json, or Go template
```

Example — find all function calls in `bash.go`:

```bash
$ runectl syntax query agent/agentools/bash.go '(call_expression function: (selector_expression) @fn)'
file:///src/rune-agent/agent/agentools/bash.go json.Unmarshal 108:11-108:25 fn
file:///src/rune-agent/agent/agentools/bash.go fmt.Sprintf 117:35-117:46 fn
file:///src/rune-agent/agent/agentools/bash.go context.WithTimeout 133:16-133:35 fn
file:///src/rune-agent/agent/agentools/bash.go t.exec.Start 149:11-149:23 fn
...
```

## Tree-sitter query syntax

Queries are S-expressions matching syntax tree nodes. Each
pattern is parentheses containing the node type and optional
child matchers.

```bash
# All function declarations
'(function_declaration name: (identifier) @name)'

# Calls to a specific function
'(call_expression function: (identifier) @fn (#eq? @fn "Println"))'

# Composite literals of a specific type
'(composite_literal type: (type_identifier) @type (#eq? @type "Config"))'

# Binary expressions with two number literals
'(binary_expression (number_literal) (number_literal))'
```
