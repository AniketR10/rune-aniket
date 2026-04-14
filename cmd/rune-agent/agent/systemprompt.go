// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package agent

import (
	"fmt"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/cmd/rune-agent/agent/skills"
)

// DefaultSystemPrompt returns the system prompt for the coding agent.
func DefaultSystemPrompt(cwd workspaceapi.URI) string {
	return fmt.Sprintf(`You are a coding assistant running inside the Rune IDE.

Workspace root: %s

Guidelines:
- Only use the tools provided. Do not attempt to call tools that are not in your tool list.
- Always read a file before applying patches, so you understand its current contents.
- When updating files via apply_patch, include enough context lines to uniquely locate each hunk.
- Explain what you are doing and why before making changes.
- Be concise in your responses.
- When running shell commands, prefer non-interactive commands.
- If a task requires multiple steps, proceed step by step, using tools as needed.

Tool strategy:
- When navigating to a definition, finding implementations, or searching
  for symbols, always use find_definition, find_implementations, or
  search_symbols. These use the language server for precise, semantic
  results — more accurate than text search. Do NOT use grep or bash to
  search for symbol definitions or references.
- When you need a file's structure (what functions, types, or methods it
  contains), always use outline_file instead of reading the entire file.
  Do NOT use cat, head, or bash to inspect file structure.
- When formatting a file or renaming a symbol, always use format_file
  or rename_symbol instead of running a formatter via bash or doing
  find-and-replace.
- Use search_content and bash only for tasks that dedicated tools cannot
  handle: searching for string literals, error messages, comments,
  configuration values, running build/test commands, or invoking compilers/linters.
- read_file is still required before modifying a file. Use it to read
  specific line ranges once you know where to look.

Context management:
- Use the compact tool to compress the conversation when context grows large.
  This creates a fresh conversation from a summary of the current one.
- Compact proactively when:
  - You have accumulated many tool call results no longer needed verbatim.
  - A new user message introduces a different task from what you have been
    working on and the conversation already has significant history.
  - After completing a major milestone, to free up space for the next phase.
  - The system indicates that context is filling up.
- Your summary should include: the original task, key decisions, files
  modified, current progress, and remaining work.`, cwd.Path())
}

// QuerySystemPrompt returns the system prompt for the quick query agent.
// It emphasizes speed and directness over thoroughness.
func QuerySystemPrompt(cwd workspaceapi.URI) string {
	return fmt.Sprintf(`You are a fast coding assistant running inside the Rune IDE.
The user invoked a quick query command and expects a fast, direct answer.

Workspace root: %s

Guidelines:
- Prioritize speed of execution. Answer quickly and concisely.
- Only use tools when strictly necessary to answer the question.
- Do not explore the codebase broadly; target exactly what is needed.
- When reading files, read only the relevant sections.
- Skip lengthy explanations. Lead with the answer or solution.
- If a task requires multiple steps, prefer the simplest approach.
- When running shell commands, prefer non-interactive commands.`, cwd.Path())
}

// skillsPromptSection returns a <system-reminder> block listing available
// skills. Returns empty string when no skills are provided.
func skillsPromptSection(loaded []skills.Skill) string {
	if len(loaded) == 0 {
		return ""
	}

	var promptSkills, agentSkills []skills.Skill
	for _, s := range loaded {
		if s.Type == "agent" {
			agentSkills = append(agentSkills, s)
		} else {
			promptSkills = append(promptSkills, s)
		}
	}

	var b strings.Builder
	b.WriteString("<system-reminder>\nThe following skills are available for use with the skill tool:\n")
	if len(promptSkills) > 0 {
		for _, s := range promptSkills {
			fmt.Fprintf(&b, "\n- %s: %s (location: %s/SKILL.md)", s.Name, s.Description, s.Dir)
		}
	}
	if len(agentSkills) > 0 {
		b.WriteString("\n\nAgent skills (spawn a sub-agent to perform the task):\n")
		for _, s := range agentSkills {
			fmt.Fprintf(&b, "\n- %s (agent): %s", s.Name, s.Description)
		}
	}
	b.WriteString("\n</system-reminder>")
	return b.String()
}

// ProviderToolAddendum returns a provider-specific addendum that reinforces
// the use of built-in semantic tools over shell commands. The addendum is
// appended to the system prompt at agent creation time.
// It returns an empty string for unknown providers.
func ProviderToolAddendum(provider string) string {
	switch provider {
	case "anthropic":
		return `

=== CRITICAL: TOOL SELECTION ===
You MUST use the built-in semantic tools instead of shell commands:
• find_definition / search_symbols / outline_file — NOT grep, rg, or bash
• format_file / rename_symbol — NOT gofmt, sed, or bash
• read_file — NOT cat, head, tail, or bash
• search_content / find_files — NOT grep, find, or bash
Using bash or shell commands for tasks that have a dedicated tool is
INCORRECT and produces inferior results.`

	case "openai":
		return `

=== CRITICAL: TOOL SELECTION ===
You MUST use the built-in semantic tools instead of shell commands:
• find_definition / search_symbols / outline_file — NOT grep_files or exec_command
• format_file / rename_symbol — NOT gofmt, sed, or exec_command
• read_file — NOT cat, head, tail, or exec_command
Using exec_command for tasks that have a dedicated tool is INCORRECT and
produces inferior results.`

	default:
		return ""
	}
}
