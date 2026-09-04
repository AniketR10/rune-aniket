// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

// Package hooks provides a Claude-Code-style hook system for the
// rune-agent extension. Hooks are configured under
// extensions.rune-agent.config.hooks and fire at well-defined points
// in the agent loop and extension handlers (SessionStart,
// UserPromptSubmit, PostToolUse, Stop, etc.).
package hooks

import (
	"fmt"
	"time"
)

// Event identifies one of the supported hook event types.
type Event string

// Supported hook events.
const (
	EventSessionStart     Event = "SessionStart"
	EventSessionEnd       Event = "SessionEnd"
	EventUserPromptSubmit Event = "UserPromptSubmit"
	EventPostToolUse      Event = "PostToolUse"
	EventStop             Event = "Stop"
	EventSubagentStop     Event = "SubagentStop"
	EventPreCompact       Event = "PreCompact"
	EventNotification     Event = "Notification"
)

// Hook handler types.
const (
	HookTypeCommand = "command"
	HookTypeHTTP    = "http"
)

// DefaultTimeout is the default per-hook timeout when none is configured.
const DefaultTimeout = 60 * time.Second

// Config holds the parsed hook configuration: each event maps to an
// ordered list of Group entries.
type Config struct {
	SessionStart     []Group
	SessionEnd       []Group
	UserPromptSubmit []Group
	PostToolUse      []Group
	Stop             []Group
	SubagentStop     []Group
	PreCompact       []Group
	Notification     []Group
}

// groupsFor returns a copy of the group list for the given event.
func (c Config) groupsFor(ev Event) []Group {
	switch ev {
	case EventSessionStart:
		return c.SessionStart
	case EventSessionEnd:
		return c.SessionEnd
	case EventUserPromptSubmit:
		return c.UserPromptSubmit
	case EventPostToolUse:
		return c.PostToolUse
	case EventStop:
		return c.Stop
	case EventSubagentStop:
		return c.SubagentStop
	case EventPreCompact:
		return c.PreCompact
	case EventNotification:
		return c.Notification
	}
	return nil
}

// Group binds a matcher to a list of hooks.
type Group struct {
	Matcher string
	Hooks   []Hook
}

// Hook describes a single hook handler. Type selects the dispatcher:
// "command" runs a shell command; "http" POSTs to a URL.
type Hook struct {
	Type    string
	Command string
	Shell   string
	URL     string
	Headers map[string]string
	// AllowedEnvVars enumerates env-var names that may be interpolated
	// (as ${VAR}) in HTTP header values. Variables not in this list
	// are left literal.
	AllowedEnvVars []string
	Timeout        time.Duration
}

// Parse converts a raw hook map (as decoded from the workspace
// config) into a Config. Unknown event keys are ignored.
func Parse(raw map[string]any) (Config, error) {
	if raw == nil {
		return Config{}, nil
	}
	var cfg Config
	for key, val := range raw {
		groups, err := parseGroups(val)
		if err != nil {
			return Config{}, fmt.Errorf("%s: %w", key, err)
		}
		switch Event(key) {
		case EventSessionStart:
			cfg.SessionStart = groups
		case EventSessionEnd:
			cfg.SessionEnd = groups
		case EventUserPromptSubmit:
			cfg.UserPromptSubmit = groups
		case EventPostToolUse:
			cfg.PostToolUse = groups
		case EventStop:
			cfg.Stop = groups
		case EventSubagentStop:
			cfg.SubagentStop = groups
		case EventPreCompact:
			cfg.PreCompact = groups
		case EventNotification:
			cfg.Notification = groups
		default:
			// Unknown events are ignored; future-proofing.
		}
	}
	return cfg, nil
}

func parseGroups(v any) ([]Group, error) {
	if v == nil {
		return nil, nil
	}
	list, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("expected list, got %T", v)
	}
	out := make([]Group, 0, len(list))
	for i, item := range list {
		g, err := parseGroup(item)
		if err != nil {
			return nil, fmt.Errorf("group %d: %w", i, err)
		}
		out = append(out, g)
	}
	return out, nil
}

func parseGroup(v any) (Group, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return Group{}, fmt.Errorf("expected map, got %T", v)
	}
	var g Group
	if s, ok := m["matcher"].(string); ok {
		g.Matcher = s
	}
	hookList, ok := m["hooks"].([]any)
	if !ok && m["hooks"] != nil {
		return Group{}, fmt.Errorf("hooks: expected list, got %T", m["hooks"])
	}
	g.Hooks = make([]Hook, 0, len(hookList))
	for i, item := range hookList {
		h, err := parseHook(item)
		if err != nil {
			return Group{}, fmt.Errorf("hook %d: %w", i, err)
		}
		g.Hooks = append(g.Hooks, h)
	}
	return g, nil
}

func parseHook(v any) (Hook, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return Hook{}, fmt.Errorf("expected map, got %T", v)
	}
	var h Hook
	if s, ok := m["type"].(string); ok {
		h.Type = s
	}
	if s, ok := m["command"].(string); ok {
		h.Command = s
	}
	if s, ok := m["shell"].(string); ok {
		h.Shell = s
	}
	if s, ok := m["url"].(string); ok {
		h.URL = s
	}
	if hs, ok := m["headers"].(map[string]any); ok {
		h.Headers = make(map[string]string, len(hs))
		for k, v := range hs {
			if sv, ok := v.(string); ok {
				h.Headers[k] = sv
			}
		}
	}
	if envs, ok := m["allowed_env_vars"].([]any); ok {
		for _, e := range envs {
			if s, ok := e.(string); ok {
				h.AllowedEnvVars = append(h.AllowedEnvVars, s)
			}
		}
	}
	if v, ok := m["timeout"]; ok && v != nil {
		switch t := v.(type) {
		case string:
			d, err := time.ParseDuration(t)
			if err != nil {
				return Hook{}, fmt.Errorf("timeout %q: %w", t, err)
			}
			h.Timeout = d
		case int:
			h.Timeout = time.Duration(t) * time.Second
		case int64:
			h.Timeout = time.Duration(t) * time.Second
		case float64:
			h.Timeout = time.Duration(t * float64(time.Second))
		default:
			return Hook{}, fmt.Errorf("timeout: expected duration string, got %T", v)
		}
	}
	switch h.Type {
	case HookTypeCommand:
		if h.Command == "" {
			return Hook{}, fmt.Errorf("command type requires command")
		}
	case HookTypeHTTP:
		if h.URL == "" {
			return Hook{}, fmt.Errorf("http type requires url")
		}
	case "":
		return Hook{}, fmt.Errorf("type required (one of: command, http)")
	default:
		return Hook{}, fmt.Errorf("unknown hook type %q", h.Type)
	}
	if h.Timeout <= 0 {
		h.Timeout = DefaultTimeout
	}
	return h, nil
}
