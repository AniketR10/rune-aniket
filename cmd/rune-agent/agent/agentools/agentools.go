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

package agentools

import (
	"unstable.build/go-tui/cmd/rune-agent/agent"
	"unstable.build/go-tui/cmd/rune-agent/agent/skills"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

// Config holds optional configuration for the default tool set.
type Config struct {
	// MaxLineBytes is the per-line truncation limit for read_file output.
	// 0 uses agent.DefaultMaxLineBytes.
	MaxLineBytes int
}

// DefaultTools returns all available tools pre-configured with the
// workspace working directory and the shared FileTracker that should
// be passed to LSPTools and SyntaxTools.
func DefaultTools(
	fs workspaceapi.FileSystem,
	exec workspaceapi.Executor,
	cwd workspaceapi.URI,
	cfg Config,
) ([]agent.Tool, *FileTracker) {
	tracker := NewFileTracker()
	return []agent.Tool{
		newReadFile(fs, cwd, tracker, cfg.MaxLineBytes),
		newApplyPatch(fs, cwd, tracker),
		newSearch(fs, cwd, tracker),
		newFindFiles(fs, cwd, tracker),
		newBash(exec, cwd),
		newCompact(),
	}, tracker
}

// SessionTools returns session management tools for a
// parent agent session. The agents slice is embedded in
// the agent tool's description. skillRegistry adds
// agent-type skills to the description. childEvents
// receives sub-agent events for TUI rendering. Sub-agents
// do NOT receive these tools.
func SessionTools(
	spawner agent.Spawner, agents []agent.AgentSummary,
	childEvents chan<- agent.ChildEvent,
	skillRegistry *skills.SkillRegistry,
) []agent.Tool {
	return []agent.Tool{
		NewAgentTool(spawner, agents, childEvents, skillRegistry),
	}
}

// MemoryTools returns tools related to the memory system.
// Only registered when the memory workspace is available.
func MemoryTools(exec workspaceapi.Executor, memoryPath string) []agent.Tool {
	return []agent.Tool{NewRecallConversation(exec, memoryPath)}
}
