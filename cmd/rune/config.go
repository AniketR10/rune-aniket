// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2023-2024 Unstable Build, All Rights Reserved.
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

package main

import (
	_ "embed"
	"errors"
	"fmt"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/tcell/v3"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/term/gui"
)

// defaultSampleConfig is a boiled-down YAML template written to the user's
// config path the first time they run `:config` and the file doesn't exist.
// It only documents the knobs most people actually change; the full default
// tree lives in the embedded rune.star Starlark file.
//
//go:embed sample_config.yaml
var defaultSampleConfig string

//go:embed rune.star
var defaultStarlarkConfig string

func getGUIFontFamily(browser browser.Browser, cfg config.Config) (ret string) {
	family, err := cfg.GetString("font-family")
	if err != nil {
		if err != config.ErrNotFound {
			_, _ = browser.Notify(browserapi.LevelError,
				"Could not load 'gui.font-family' from config: %v", err)
		}
		return
	}
	ret = family
	return
}

func getGUIFontSize(browser browser.Browser, cfg config.Config) (ret float64) {
	ret = 13
	size, err := cfg.GetFloat("font-size")
	if err != nil {
		if err != config.ErrNotFound {
			_, _ = browser.Notify(browserapi.LevelError,
				"Could not load 'gui.font-size' from config: %v", err)
		}
		return
	}
	ret = size
	return
}

func getGUILineHeightOffset(browser browser.Browser, cfg config.Config) (ret float64) {
	ret = -1
	size, err := cfg.GetFloat("line-height-offset")
	if err != nil {
		if err != config.ErrNotFound {
			_, _ = browser.Notify(browserapi.LevelError,
				"Could not load 'gui.line-height-offset' from config: %v", err)
		}
		return
	}
	ret = size
	return
}

func getGUIColumnWidthOffset(browser browser.Browser, cfg config.Config) (ret float64) {
	ret = 0
	size, err := cfg.GetFloat("column-width-offset")
	if err != nil {
		if err != config.ErrNotFound {
			_, _ = browser.Notify(browserapi.LevelError,
				"Could not load 'gui.column-width-offset' from config: %v", err)
		}
		return
	}
	ret = size
	return
}

func getGUIFontDPI(browser browser.Browser, cfg config.Config) (ret float64) {
	ret = 0 // signals that it must be calculated automatically
	dpi, err := cfg.GetFloat("dpi")
	if err != nil {
		if err != config.ErrNotFound {
			_, _ = browser.Notify(browserapi.LevelError,
				"Could not load 'gui.dpi' from config: %v", err)
		}
		return
	}
	ret = dpi
	return
}

func getGUIDefaultColorTheme(browser browser.Browser, cfg config.Config) (
	ret string,
) {
	defaultTheme, err := cfg.GetString("default_theme")
	if err != nil {
		if err != config.ErrNotFound {
			_, _ = browser.Notify(browserapi.LevelError,
				"Could not load 'gui.default_theme' from config: %v", err)
		}
		return
	}
	ret = defaultTheme
	return
}

func getGUIColorThemes(browser browser.Browser, cfg config.Config) (
	ret map[string]gui.Theme,
) {
	themesMap, err := cfg.GetMap("themes")
	if err != nil {
		if err != config.ErrNotFound {
			_, _ = browser.Notify(browserapi.LevelError,
				"Could not load 'gui.themes' from config: %v", err)
		}
		return
	}

	ret = make(map[string]gui.Theme, len(themesMap))
	for name := range themesMap {
		themesCfg := config.MapConfig(themesMap)
		colorsMap, err := themesCfg.GetMap(name)
		if err != nil {
			if err != config.ErrNotFound {
				_, _ = browser.Notify(browserapi.LevelError,
					"Could not load 'gui.themes.%s' from config: %v", name, err)
			}
			return
		}

		theme := gui.Theme{Colors: make(map[tcell.Color]tcell.Color)}
		colorsCfg := config.MapConfig(colorsMap)
		for colorName := range colorsMap {
			switch colorName {
			case "cursor":
				value, err := colorsCfg.GetColor(colorName)
				if err != nil {
					_, _ = browser.Notify(browserapi.LevelWarn,
						"Invalid color value for 'cursor' in "+
							"'gui.themes.%s' config: %v", name, err)
					continue
				}
				theme.Cursor = value
			case "foreground":
				value, err := colorsCfg.GetColor(colorName)
				if err != nil {
					_, _ = browser.Notify(browserapi.LevelWarn,
						"Invalid color value for 'foreground' in "+
							"'gui.themes.%s' config: %v", name, err)
					continue
				}
				theme.Foreground = value
			case "background":
				value, err := colorsCfg.GetColor(colorName)
				if err != nil {
					_, _ = browser.Notify(browserapi.LevelWarn,
						"Invalid color value for 'background' in "+
							"'gui.themes.%s' config: %v", name, err)
					continue
				}
				theme.Background = value
			default:
				color, ok := tcell.ColorNames[colorName]
				if !ok {
					_, _ = browser.Notify(browserapi.LevelWarn,
						"Unknown color '%s' in 'gui.themes.%s' config. "+
							"Name must be a known W3C name.", colorName, name)
					continue
				}

				value, err := colorsCfg.GetColor(colorName)
				if err != nil {
					_, _ = browser.Notify(browserapi.LevelWarn,
						"Invalid color value for color '%s' in "+
							"'gui.themes.%s' config: %v", colorName, name, err)
					continue
				}
				theme.Colors[color] = value
			}
		}
		ret[name] = theme
	}
	return
}

func getGUILigatures(browser browser.Browser, cfg config.Config) (ret bool) {
	ret = false
	ligatures, err := cfg.GetBool("ligatures")
	if err != nil {
		if err != config.ErrNotFound {
			_, _ = browser.Notify(browserapi.LevelError,
				"Could not load 'gui.ligatures' from config: %v", err)
		}
		return
	}
	ret = ligatures
	return
}

func getGUIBackgroundBlur(browser browser.Browser, cfg config.Config) (ret int) {
	radius, err := cfg.GetInt("window_blur_radius")
	if err != nil {
		if err != config.ErrNotFound {
			_, _ = browser.Notify(browserapi.LevelError,
				"Could not load 'gui.window_blur_radius' from config: %v", err)
		}
		return
	}
	ret = radius
	return
}

func getGUIEnvVars(browser browser.Browser, cfg config.Config) (ret config.Config) {
	ret = config.NopConfig()
	env, err := cfg.GetConfig("env")
	if err != nil {
		if err != config.ErrNotFound {
			_, _ = browser.Notify(browserapi.LevelError,
				"Could not load 'gui.env' from config: %v", err)
		}
		return
	}
	ret = env
	return
}

func getGUITransparentWindow(browser browser.Browser, cfg config.Config) (ret bool) {
	transparent, err := cfg.GetBool("enable_transparent_window")
	if err != nil {
		if err != config.ErrNotFound {
			_, _ = browser.Notify(browserapi.LevelError,
				"Could not load 'gui.enable_transparent_window' from config: %v", err)
		}
		return
	}
	ret = transparent
	return
}

func getGUIWindowOpacity(browser browser.Browser, cfg config.Config) (fg, bg float64) {
	fg, bg = 1, 1
	cfg, err := cfg.GetConfig("window_opacity")
	if err != nil {
		if err != config.ErrNotFound {
			_, _ = browser.Notify(browserapi.LevelError,
				"Could not load 'gui.window_opacity' from config: %v", err)
		}
		return
	}
	fgCfg, err := cfg.GetFloat("fg")
	if err != nil {
		if err != config.ErrNotFound {
			_, _ = browser.Notify(browserapi.LevelError,
				"Could not load 'gui.window_opacity.fg' from config: %v", err)
		}
	} else {
		fg = fgCfg
	}
	bgCfg, err := cfg.GetFloat("bg")
	if err != nil {
		if err != config.ErrNotFound {
			_, _ = browser.Notify(browserapi.LevelError,
				"Could not load 'gui.window_opacity.bg' from config: %v", err)
		}
	} else {
		bg = bgCfg
	}
	return
}

func getGUIConfig(cfg config.Config) (config.Config, bool, error) {
	cfg, err := cfg.GetConfig("gui")
	if errors.Is(err, config.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("get 'gui' section from config: %w", err)
	}
	return cfg, true, nil
}
