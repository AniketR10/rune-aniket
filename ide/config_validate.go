// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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

package ide

import (
	"fmt"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/llm"
	"unstable.build/go-tui/text"
)

func validateConfig(cfg map[string]any) (err error) {
	c := ideConfig{cfg: cfg, errors: make(map[string]error)}

	if err = validateAliases(&c, cfg); err != nil {
		return
	}

	if err = validateCommandPrompt(&c, cfg); err != nil {
		return
	}

	if err = validateExo(&c, cfg); err != nil {
		return
	}

	if err = llm.ValidateConfig(c.llmConfig()); err != nil {
		return
	}
	if err = validateNotice(cfg); err != nil {
		return
	}
	return
}

func validateAliases(c *ideConfig, cfg map[string]any) (err error) {
	// Cycle detection only inspects alias Commands so we parse the
	// commands without building completer chains, which would require
	// a storage-backed history accessor that the validator doesn't have.
	aliases, _ := c.parseAliasCommands()
	err = text.ValidateCommandAliases(aliases)
	if err != nil {
		// void aliases but keep the rest of config intact.
		// this ensures that text.NewComponent doesn't hard error,
		// preventing user from editing using this same IDE.
		_, ok := c.cfg["command"]
		if !ok {
			panic("empty command config but detected invalid aliases")
		}
		cfg["command"].(map[string]any)[keyCommandAliases] = map[string]any{}
		err = fmt.Errorf("'command.%s' is invalid: %w", keyCommandAliases, err)
	}
	return
}

func validateCommandPrompt(c *ideConfig, cfg map[string]any) (err error) {
	commandKey := c.commandKey()
	editorMode := c.editorMode()

	switch editorMode {
	case editorModeModal:
		/* no validation needed */
	case editorModeStandard, editorModeEmacs:
		// unfortunately <c-space> is mapped and dispatched with Ch == ' '.
		// Handle edge case to avoid false positive.
		isCtrlSpace := commandKey.Ch == ' ' &&
			commandKey.Key == term.KeySpace && commandKey.Mod == term.ModCtrl
		isIncompatible := !isCtrlSpace && commandKey.Ch != 0 && commandKey.Mod == 0

		if isIncompatible {
			cfg["command"].(map[string]any)[keyCommandKey] = "<c-space>"
			return fmt.Errorf("command key must use ctrl or alt modifiers in " +
				"standard or emacs editor mode otherwise you wouldn't be able to activate it," +
				" falling back to <c-space>")
		}
	}
	return
}

// validateExo checks that editor.exo.command is well-formed when the
// editor mode is "exo". On failure it rewrites editor.mode back to
// "modal" so the IDE still boots, and returns a descriptive error.
// editor.exo.goto and editor.exo.quit are required and validated
// the same way; an empty or unparseable value falls back to "modal".
func validateExo(c *ideConfig, cfg map[string]any) (err error) {
	if c.editorMode() != editorModeExo {
		return
	}
	command := c.exoCommand()
	if command == "" || !strings.Contains(command, "{file}") {
		// Rewrite editor.mode back to modal so the IDE boots.
		if ed, ok := cfg["editor"].(map[string]any); ok {
			ed["mode"] = editorModeModal
		}
		return fmt.Errorf("editor.exo.command is required and must contain " +
			"{file} when editor.mode = \"exo\"; falling back to \"modal\"")
	}
	gotoTpl := c.exoGoto()
	if gotoTpl == "" {
		if ed, ok := cfg["editor"].(map[string]any); ok {
			ed["mode"] = editorModeModal
		}
		return fmt.Errorf("editor.exo.goto is required when " +
			"editor.mode = \"exo\"; falling back to \"modal\"")
	}
	if perr := parseGotoTemplate(gotoTpl); perr != nil {
		if ed, ok := cfg["editor"].(map[string]any); ok {
			ed["mode"] = editorModeModal
		}
		return fmt.Errorf("editor.exo.goto is invalid: %w; "+
			"falling back to \"modal\"", perr)
	}
	quitTpl := c.exoQuit()
	if quitTpl == "" {
		if ed, ok := cfg["editor"].(map[string]any); ok {
			ed["mode"] = editorModeModal
		}
		return fmt.Errorf("editor.exo.quit is required when " +
			"editor.mode = \"exo\"; falling back to \"modal\"")
	}
	if _, perr := term.ParseKeys(quitTpl); perr != nil {
		if ed, ok := cfg["editor"].(map[string]any); ok {
			ed["mode"] = editorModeModal
		}
		return fmt.Errorf("editor.exo.quit is invalid: %w; "+
			"falling back to \"modal\"", perr)
	}
	if err = validateExoFallback(c, cfg); err != nil {
		return
	}
	return
}

// validateExoFallback ensures editor.exo.fallback, when set, resolves
// to a valid fallback via normalizeEditorFallback. Unrecognised values
// are rewritten to the canonical default so the IDE still boots. An
// absent fallback key is treated as valid and uses the default.
func validateExoFallback(c *ideConfig, cfg map[string]any) error {
	exo, ok := c.exo()
	if !ok {
		return nil
	}
	raw, err := exo.GetString("fallback")
	if err != nil {
		if err == config.ErrNotFound {
			return nil
		}
		return fmt.Errorf("editor.exo.fallback: %w", err)
	}
	if raw == "" {
		return nil
	}
	if _, ok := normalizeEditorFallback(raw); ok {
		return nil
	}
	if ed, ok := cfg["editor"].(map[string]any); ok {
		if b, ok := ed["exo"].(map[string]any); ok {
			b["fallback"] = editorFallbackStandard
		}
	}
	return fmt.Errorf("editor.exo.fallback must be %q or %q; got %q, "+
		"falling back to %q",
		editorFallbackModal, editorFallbackStandard, raw,
		editorFallbackStandard)
}

// parseGotoTemplate splits tpl around {line}/{col} placeholders and
// validates that each literal segment parses with term.ParseKeys.
// Returns nil if the template is well-formed.
func parseGotoTemplate(tpl string) error {
	segments := splitGotoTemplate(tpl)
	for _, seg := range segments {
		if seg.placeholder != "" {
			continue
		}
		if seg.literal == "" {
			continue
		}
		if _, perr := term.ParseKeys(seg.literal); perr != nil {
			return fmt.Errorf("segment %q: %w", seg.literal, perr)
		}
	}
	return nil
}

// gotoSegment is either a literal key sequence string or a {line}/{col}
// placeholder.
type gotoSegment struct {
	literal     string
	placeholder string // "line" or "col"
}

// validateNotice rewrites an invalid workspace.notice.show back to
// "once" so the IDE still boots when the user supplies an
// unrecognised value.
func validateNotice(cfg map[string]any) error {
	ws, ok := cfg["workspace"].(map[string]any)
	if !ok {
		return nil
	}
	raw, ok := ws["notice"]
	if !ok {
		return nil
	}
	notice, ok := raw.(map[string]any)
	if !ok {
		return fmt.Errorf("workspace.notice must be a mapping, got %T", raw)
	}
	for k := range notice {
		switch k {
		case "path", "literal", "show":
		default:
			return fmt.Errorf("workspace.notice has unknown key %q "+
				"(expected path, literal, show)", k)
		}
	}
	if v, present := notice["show"]; present {
		s, ok := v.(string)
		if !ok {
			notice["show"] = "once"
			return fmt.Errorf("workspace.notice.show must be a string, "+
				"got %T; falling back to %q", v, "once")
		}
		switch s {
		case "", "once", "always":
		default:
			notice["show"] = "once"
			return fmt.Errorf("workspace.notice.show must be %q or %q; "+
				"got %q, falling back to %q", "once", "always", s, "once")
		}
	}
	return nil
}

// splitGotoTemplate splits tpl into literal/placeholder segments.
// Unknown {...} placeholders are treated as literal text.
func splitGotoTemplate(tpl string) []gotoSegment {
	var out []gotoSegment
	for tpl != "" {
		lineIdx := strings.Index(tpl, "{line}")
		colIdx := strings.Index(tpl, "{col}")
		var idx int
		var name string
		var width int
		switch {
		case lineIdx == -1 && colIdx == -1:
			out = append(out, gotoSegment{literal: tpl})
			return out
		case lineIdx == -1:
			idx, name, width = colIdx, "col", len("{col}")
		case colIdx == -1:
			idx, name, width = lineIdx, "line", len("{line}")
		case lineIdx < colIdx:
			idx, name, width = lineIdx, "line", len("{line}")
		default:
			idx, name, width = colIdx, "col", len("{col}")
		}
		if idx > 0 {
			out = append(out, gotoSegment{literal: tpl[:idx]})
		}
		out = append(out, gotoSegment{placeholder: name})
		tpl = tpl[idx+width:]
	}
	return out
}
