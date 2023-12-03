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
		sys = "system"
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
func Editor(cfg config.Config) (text.Editor, error) {
	edConfig, err := cfg.GetConfig("editor")
	if err != nil {
		if err != config.ErrNotFound {
			err = fmt.Errorf("failed to get 'editor' from config: %v", err)
			return nil, err
		}
		return text.DefaultSimpleEditor(), nil
	}

	mode, err := edConfig.GetString("mode")
	if err != nil {
		if err != config.ErrNotFound {
			err = fmt.Errorf("failed to get 'mode' from editor config: %v", err)
			return nil, err
		}
		mode = "modeless"
	}
	switch mode {
	case "modal":
		return vi.Editor(), nil
	default:
		return text.DefaultSimpleEditor(), nil
	}
}
