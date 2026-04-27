// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.

package hooks

import (
	"strings"
	"testing"
	"time"
)

func TestParse_RoundTrip(t *testing.T) {
	t.Parallel()
	raw := map[string]any{
		"PostToolUse": []any{
			map[string]any{
				"matcher": "edit_file|write_file",
				"hooks": []any{
					map[string]any{
						"type":    "command",
						"command": "fmt.sh",
						"timeout": "30s",
					},
				},
			},
		},
		"UserPromptSubmit": []any{
			map[string]any{
				"hooks": []any{
					map[string]any{
						"type":             "http",
						"url":              "http://localhost:8765/p",
						"headers":          map[string]any{"Authorization": "Bearer ${T}"},
						"allowed_env_vars": []any{"T"},
						"timeout":          "5s",
					},
				},
			},
		},
	}
	cfg, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(cfg.PostToolUse) != 1 || len(cfg.PostToolUse[0].Hooks) != 1 {
		t.Fatalf("PostToolUse: %+v", cfg.PostToolUse)
	}
	if got := cfg.PostToolUse[0].Hooks[0].Timeout; got != 30*time.Second {
		t.Fatalf("timeout = %s, want 30s", got)
	}
	if cfg.PostToolUse[0].Matcher != "edit_file|write_file" {
		t.Fatalf("matcher = %q", cfg.PostToolUse[0].Matcher)
	}
	ups := cfg.UserPromptSubmit
	if len(ups) != 1 || len(ups[0].Hooks) != 1 {
		t.Fatalf("UserPromptSubmit: %+v", ups)
	}
	h := ups[0].Hooks[0]
	if h.Type != HookTypeHTTP || h.URL == "" {
		t.Fatalf("http hook: %+v", h)
	}
	if h.Headers["Authorization"] != "Bearer ${T}" {
		t.Fatalf("header: %+v", h.Headers)
	}
	if len(h.AllowedEnvVars) != 1 || h.AllowedEnvVars[0] != "T" {
		t.Fatalf("AllowedEnvVars: %+v", h.AllowedEnvVars)
	}
}

func TestParse_MissingTypeFails(t *testing.T) {
	t.Parallel()
	raw := map[string]any{
		"Stop": []any{
			map[string]any{
				"hooks": []any{
					map[string]any{"command": "x"},
				},
			},
		},
	}
	_, err := Parse(raw)
	if err == nil || !strings.Contains(err.Error(), "type required") {
		t.Fatalf("expected type error, got %v", err)
	}
}

func TestParse_DefaultTimeout(t *testing.T) {
	t.Parallel()
	raw := map[string]any{
		"Stop": []any{
			map[string]any{
				"hooks": []any{
					map[string]any{"type": "command", "command": "x"},
				},
			},
		},
	}
	cfg, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := cfg.Stop[0].Hooks[0].Timeout; got != DefaultTimeout {
		t.Fatalf("timeout = %s, want %s", got, DefaultTimeout)
	}
}
