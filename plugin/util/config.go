package util

import (
	"fmt"

	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/plugin"
)

// Tabspaces extract browser.tabspaces from the given cfg.
func Tabspaces(cfg plugin.Config) (int, error) {
	browserConfig, err := cfg.GetConfig("browser")
	if err != nil {
		if err != plugin.ErrNotFound {
			err = fmt.Errorf("failed to get 'tabspaces' from config: %v", err)
			return 0, err
		}
		return cell.DefaultTabspaces, nil
	}

	ret, err := browserConfig.GetInt("tabspaces")
	if err != nil {
		if err != plugin.ErrNotFound {
			err = fmt.Errorf("failed to get 'tabspaces' from config: %v", err)
			return 0, err
		}
		ret = cell.DefaultTabspaces
	}
	return ret, nil
}
