package util

import (
	"errors"
	"fmt"

	"unstable.build/go-tui/api/config"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/text/clipboard"
	sysclip "unstable.build/go-tui/text/clipboard/system"
	"unstable.build/go-tui/text/vi"
)

// Tabspaces extract browser.tabspaces from the given cfg.
func Tabspaces(cfg config.Config) (int, error) {
	browserConfig, err := cfg.GetConfig("browser")
	if err != nil {
		if err != config.ErrNotFound {
			err = fmt.Errorf("failed to get 'tabspaces' from config: %v", err)
			return 0, err
		}
		return cell.DefaultTabspaces, nil
	}

	ret, err := browserConfig.GetInt("tabspaces")
	if err != nil {
		if err != config.ErrNotFound {
			err = fmt.Errorf("failed to get 'tabspaces' from config: %v", err)
			return 0, err
		}
		ret = cell.DefaultTabspaces
	}
	return ret, nil
}

// WindowManagerFrame extract browser.window_manager.frame from the given cfg.
func WindowManagerFrame(cfg config.Config) (bool, error) {
	browserConfig, err := cfg.GetConfig("browser")
	if err != nil {
		if err != config.ErrNotFound {
			err = fmt.Errorf("failed to get 'tabspaces' from config: %v", err)
			return false, err
		}
		return browser.DefaultConfig().Frame, nil
	}

	wmConfig, err := browserConfig.GetConfig("window_manager")
	if err != nil {
		if err != config.ErrNotFound {
			err = fmt.Errorf("failed to get 'tabspaces' from config: %v", err)
			return false, err
		}
		return browser.DefaultConfig().Frame, nil
	}

	ret, err := wmConfig.GetBool("frame")
	if err != nil {
		if err != config.ErrNotFound {
			err = fmt.Errorf("failed to get 'tabspaces' from config: %v", err)
			return false, err
		}
		ret = browser.DefaultConfig().Frame
	}
	return ret, nil
}

// Clipboard returns the configured clipboard.
func Clipboard(cfg config.Config) (clipboard.Register, error) {
	sys, err := cfg.GetString("clipboard")
	if err != nil && err != config.ErrNotFound {
		err = fmt.Errorf("failed to get 'tabspaces' from config: %v", err)
		return nil, err
	}

	if err == config.ErrNotFound {
		sys = "memory"
	}

	switch sys {
	case "memory":
		return clipboard.NewInMemory(), nil
	case "system":
		clip, err := sysclip.NewRegister()
		if err != nil {
			return clipboard.NewInMemory(), nil
		}
		return clip, nil
	default:
		return nil, errors.New("unknown clipboard")
	}
}

// Editor returns the editor implementation as configured.
func Editor(clipboard clipboard.Register, cfg config.Config) (text.Editor, error) {
	mode, err := editorMode(cfg)
	if err != nil {
		return nil, err
	}
	switch mode {
	case "modal":
		return vi.Editor(vi.WithClipboard(clipboard)), nil
	default:
		return text.DefaultSimpleEditor(clipboard), nil
	}
}

// Wrap returns the current editor implementation is configured with wrap mode.
func Wrap(cfg config.Config) (bool, error) {
	var def bool
	edConfig, err := cfg.GetConfig("editor")
	if err != nil {
		if err != config.ErrNotFound {
			err = fmt.Errorf("failed to get 'editor' from config: %v", err)
			return false, err
		}
		return false, nil
	}

	mode, err := editorMode(cfg)
	if err != nil {
		return false, err
	}
	modeConfig, err := edConfig.GetConfig(mode)
	if err != nil {
		if err != config.ErrNotFound {
			err = fmt.Errorf("failed to get '%s' from editor config: %v", mode, err)
			return false, err
		}
		return def, nil
	}

	wrap, err := modeConfig.GetBool("wrap")
	if err != nil {
		if err != config.ErrNotFound {
			err = fmt.Errorf("failed to get 'wrap' from editor config: %v", err)
			return false, err
		}
	}
	return wrap, nil
}

func editorMode(cfg config.Config) (string, error) {
	def := "modeless"
	edConfig, err := cfg.GetConfig("editor")
	if err != nil {
		if err != config.ErrNotFound {
			err = fmt.Errorf("failed to get 'editor' from config: %v", err)
			return "", err
		}
		return def, nil
	}

	mode, err := edConfig.GetString("mode")
	if err != nil {
		if err != config.ErrNotFound {
			err = fmt.Errorf("failed to get 'mode' from editor config: %v", err)
			return "", err
		}
		mode = def
	}
	return mode, nil
}
