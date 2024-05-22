package ide

import (
	"fmt"

	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
)

func validateConfig(cfg map[string]any) (err error) {
	c := ideConfig{cfg: cfg}

	if err = validateAliases(&c, cfg); err != nil {
		return
	}

	if err = validateCommandPrompt(&c, cfg); err != nil {
		return
	}
	return
}

func validateAliases(c *ideConfig, cfg map[string]any) (err error) {
	err = text.ValidateCommandAliases(c.commandAliases())
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
	case editorModeModeless:
		// unfortunately <c-space> is mapped and dispatched with Ch == ' '.
		// Handle edge case to avoid false positive.
		isCtrlSpace := commandKey.Ch == ' ' && commandKey.Key == term.KeyCtrlSpace
		isIncompatible := !isCtrlSpace && commandKey.Ch != 0 && commandKey.Mod == 0

		if isIncompatible {
			cfg["command"].(map[string]any)[keyCommandKey] = "<c-space>"
			return fmt.Errorf("Command key must use ctrl or alt modifiers in " +
				"modeless editor mode otherwise you wouldn't be able to activate it." +
				" Falling back to <c-space>.")
		}
	}
	return
}
