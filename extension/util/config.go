package util

import (
	"fmt"

	"unstable.build/go-tui/api/config"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/text/clipboard"
	sysclip "unstable.build/go-tui/text/clipboard/system"
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
	switch sys {
	case "memory":
		return clipboard.NewInMemory(), nil
	default:
		clip, err := sysclip.NewRegister()
		if err != nil {
			return clipboard.NewInMemory(), nil
		}
		return clip, nil
	}
}
