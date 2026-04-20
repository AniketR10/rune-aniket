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
	"context"
	"errors"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/cmd/rune/ide/apiclient"
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/ide"
	"unstable.build/go-tui/term/gui"
	"unstable.build/go-tui/text"
)

const (
	cmdSetOpacity        = "guiopacity"
	cmdSetBackgroundBlur = "guibackgroundblur"
	cmdSetTheme          = "guitheme"
)

var fullscreen bool

func subscribeCommands(
	g *gui.GUI, c *apiclient.Client, cerr error,
	i *ide.IDE, transparentEnabled bool, configPath string,
	launchCmd []string,
) (ret error) {
	if err := subscribeGUICommands(g, i, transparentEnabled, launchCmd); err != nil {
		ret = multierror.Append(ret, err)
	}
	if err := subscribeOtherCommands(i, c, cerr, configPath); err != nil {
		ret = multierror.Append(ret, err)
	}
	return ret
}

func subscribeOtherCommands(
	i *ide.IDE, c *apiclient.Client, cerr error, configPath string,
) (ret error) {
	var commands = []struct {
		cmd           textapi.CommandManual
		handleCommand func(context.Context, textapi.Command) (err error)
		completer     func(context.Context, textapi.Command) (
			iterator.Iterator[string], string, error,
		)
	}{
		{
			cmd: textapi.CommandManual{
				Name: "login",
				Summary: "Authenticate with Rune API to enable free and paid-only functionality " +
					"that requires authenticated access.",
			},
			handleCommand: func(ctx context.Context, cmd textapi.Command) (err error) {
				if c == nil {
					return cerr
				}
				return c.Login(ctx)
			},
		}, {
			cmd: textapi.CommandManual{
				Name: "logout",
				Summary: "Log out from the current session. " +
					"You will still be able to use functionality that does not require access " +
					" to the Rune API.",
			},
			handleCommand: func(ctx context.Context, cmd textapi.Command) (err error) {
				if c == nil {
					return cerr
				}
				return c.Logout(ctx)
			},
		},
		{
			cmd: textapi.CommandManual{
				Name: "config",
				Summary: "Edits the current configuration. If there's no configuration on disk, " +
					"the sample configuration is opened and the user has a chance to persist updates to disk.",
			},
			handleCommand: func(ctx context.Context, cmd textapi.Command) error {
				uri, err := workspaceapi.CurrentUserHostURI(configPath)
				if err != nil {
					err = fmt.Errorf("parse config uri: %w", err)
					return err
				}
				_, err = os.Stat(configPath)
				if err == nil {
					return i.Open(uri)
				}
				if err != nil && !errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("stat config file %q: %w", configPath, err)
				}

				err = os.WriteFile(configPath, []byte(defaultSampleConfig), 0666)
				if err != nil {
					return fmt.Errorf("write sample config to config file %q: %w", configPath, err)
				}

				return i.Open(uri)
			},
		},
	}
	if os.Getenv("RUNE_DEBUG") == "true" {
		commands = append(commands, struct {
			cmd           textapi.CommandManual
			handleCommand func(context.Context, textapi.Command) (err error)
			completer     func(context.Context, textapi.Command) (
				iterator.Iterator[string], string, error,
			)
		}{
			cmd: textapi.CommandManual{
				Name: "pprof",
				Summary: "Starts a pprof server for debugging. " +
					"If no addr is passed, :6662 is used",
				Synopsis: "[addr]",
			},
			handleCommand: func(ctx context.Context, cmd textapi.Command) (err error) {
				addr := ":6662"
				if len(cmd.Args) == 1 {
					addr = cmd.Args[0]
				}
				runtime.SetBlockProfileRate(1)
				runtime.SetMutexProfileFraction(1)
				go debug.CapturePanicReport(func() {
					if err := http.ListenAndServe(addr, nil); err != http.ErrServerClosed {
						log.Errorf("listen and serve pprof: %v", err)
					}
				})
				return nil
			},
		})
	}
	for _, man := range commands {
		handler := text.FuncCommandHandler(man.handleCommand, man.completer)
		err := i.SubscribeCommand(man.cmd, handler)
		if err != nil {
			ret = multierror.Append(ret, fmt.Errorf("subscribe '%s': %v", man.cmd.Name, err))
		}
	}
	return
}

func subscribeGUICommands(
	g *gui.GUI, i *ide.IDE, transparentEnabled bool,
	launchCmd []string,
) (ret error) {
	var commands = []struct {
		cmd           textapi.CommandManual
		handleCommand func(context.Context, textapi.Command) (err error)
		completer     func(context.Context, textapi.Command) (
			iterator.Iterator[string], string, error,
		)
	}{
		{
			cmd: textapi.CommandManual{
				Name:    "guiminimize",
				Summary: "Minimizes the OS window",
			},
			handleCommand: func(ctx context.Context, cmd textapi.Command) (err error) {
				g.MinimizeWindow()
				return
			},
		},
		{
			cmd: textapi.CommandManual{
				Name: cmdSetOpacity,
				Summary: "Sets the background and optionally foreground opacity. " +
					"Expects at least one float between 0 and 1. It requires " +
					"gui.enable_transparent_window to be set to true in configuration. " +
					"To make this changes permanent, you can set `gui.window_opacity` " +
					"in your configuration.",
				Synopsis: "background [foreground]",
			},
			handleCommand: func(ctx context.Context, cmd textapi.Command) (err error) {
				if len(cmd.Args) == 0 {
					err = errors.New("expected at least one float between " +
						"0 and 1 with the background opacity")
					return
				}
				background, err := strconv.ParseFloat(cmd.Args[0], 64)
				if err != nil {
					return fmt.Errorf("parse background opacity float: %w", err)
				}
				var foreground float64 = 1
				if len(cmd.Args) > 1 {
					foreground, err = strconv.ParseFloat(cmd.Args[1], 64)
					if err != nil {
						return fmt.Errorf("parse foreground opacity float: %w", err)
					}
				}
				if background < 0 || background > 1 || foreground < 0 || foreground > 1 {
					return errors.New("expected opacity values to be between 0 and 1")
				}
				g.SetOpacity(background, foreground)
				return
			},
		},
		{
			cmd: textapi.CommandManual{
				Name: cmdSetBackgroundBlur,
				Summary: "Sets the window background blur. " +
					"Expects one argument with the blur radius. " +
					"gui.enable_transparent_window to be set to true in configuration, " +
					"and background opacity must be less than 1 for the effect to be visible. " +
					"To make this changes permanent, you can set `gui.window_blur_radius` " +
					"in your configuration.",
				Synopsis: "radius",
			},
			handleCommand: func(ctx context.Context, cmd textapi.Command) (err error) {
				if len(cmd.Args) == 0 {
					err = errors.New("expected one argument with the blur radius")
					return
				}
				radius, err := strconv.Atoi(cmd.Args[0])
				if err != nil {
					return fmt.Errorf("parse blur radius integer: %w", err)
				}
				if radius < 0 {
					return errors.New("expected positive blur radius value")
				}
				if runtime.GOOS != "darwin" {
					return errors.New("this feature is only supported on MacOS")
				}
				g.SetBackgroundBlur(radius)
				return
			},
		},
		{
			cmd: textapi.CommandManual{
				Name:    "guimaximize",
				Summary: "Maximizes the OS window",
			},
			handleCommand: func(ctx context.Context, cmd textapi.Command) (err error) {
				g.MaximizeWindow()
				return
			},
		},
		{
			cmd: textapi.CommandManual{
				Name: "guifullscreen",
				Summary: "Toggles the current mode to fullscreen or not. " +
					"When on, the window is automatically enlarged " +
					"to fit the available space. " +
					"This command does nothing on macOS when the window is set to full screen" +
					"natively by the OS, rather than via this command. ",
			},
			handleCommand: func(ctx context.Context, cmd textapi.Command) (err error) {
				fullscreen = !fullscreen
				g.SetFullscreen(fullscreen)
				return
			},
		},
		{
			cmd: textapi.CommandManual{
				Name:     "guiposition",
				Synopsis: "xoffset yoffset",
				Summary: "Sets the window position as an offset from " +
					"the upper-left corner of the current monitor, " +
					"in device-independent pixels. " +
					"If the application is running on full screen mode set via guiToggleFullscren, " +
					"it will set the original window size. ",
			},
			handleCommand: func(ctx context.Context, cmd textapi.Command) (err error) {
				if len(cmd.Args) != 2 {
					return errors.New("expected two integer arguments with the x " +
						"offset and y offset in pixels")
				}
				x, err := strconv.Atoi(cmd.Args[0])
				if err != nil {
					return fmt.Errorf("parse x offset integer: %w", err)
				}
				y, err := strconv.Atoi(cmd.Args[1])
				if err != nil {
					return fmt.Errorf("parse y offset integer: %w", err)
				}
				g.SetWindowPosition(x, y)
				return
			},
		},
		{
			cmd: textapi.CommandManual{
				Name:     "guisize",
				Synopsis: "width height",
				Summary: "Sets the window size in pixels. " +
					"If the application is running in fullscreen mode, set via guiToggleFullscreen, " +
					"it will set the original window size. ",
			},
			handleCommand: func(ctx context.Context, cmd textapi.Command) (err error) {
				if len(cmd.Args) != 2 {
					return errors.New("expected two integer arguments with the width " +
						"and height in pixels")
				}
				width, err := strconv.Atoi(cmd.Args[0])
				if err != nil {
					return fmt.Errorf("parse width integer: %w", err)
				}
				height, err := strconv.Atoi(cmd.Args[1])
				if err != nil {
					return fmt.Errorf("parse height integer: %w", err)
				}

				if width < 0 || height < 0 {
					return errors.New("width and height must be a positive integer")
				}
				g.SetWindowSize(width, height)
				return
			},
		},
		{
			cmd: textapi.CommandManual{
				Name:     "guifont",
				Synopsis: "[family]",
				Summary: "Sets the font collection identified by the given family name. " +
					"If family is set to an empty string, the default system font is used. " +
					"If the family is set to 'builtin', the GUI's builtin fallback font is used. ",
			},
			handleCommand: func(ctx context.Context, cmd textapi.Command) (err error) {
				err = g.SetFont(strings.Join(cmd.Args, " "))
				return
			},
			completer: func(ctx context.Context, cmd textapi.Command) (
				iterator.Iterator[string], string, error,
			) {
				it, err := g.AvailableFontFamilies()
				if err != nil {
					return nil, "", err
				}
				return it, "", nil
			},
		},
		{
			cmd: textapi.CommandManual{
				Name:     cmdSetTheme,
				Synopsis: "[name]",
				Summary: "Sets the color theme. If the name argument is omitted, " +
					"the theme is reset. If a name is passed, this name must be " +
					"one of the themes configured via `gui.themes`.",
			},
			handleCommand: func(ctx context.Context, cmd textapi.Command) (err error) {
				if len(cmd.Args) == 0 {
					cmd.Args = append(cmd.Args, "") // reset theme
				}
				theme, err := g.SetTheme(cmd.Args[0])
				if err == nil {
					i.SetDefaultAttributes(term.Attributes{
						Fg: theme.Foreground,
						Bg: theme.Background,
					})
				}
				return
			},
			completer: func(ctx context.Context, cmd textapi.Command) (
				iterator.Iterator[string], string, error,
			) {
				themes := g.Themes()
				sort.Strings(themes)
				return iterator.FromSlice(themes), "", nil
			},
		},
		{
			cmd: textapi.CommandManual{
				Name: "guifontsize",
				Summary: "Increase or decrease the size of the rendered font." +
					"To make changes permanent, update the 'gui.font-size' configuration.",
			},
			handleCommand: func(ctx context.Context, cmd textapi.Command) (err error) {
				if len(cmd.Args) != 1 {
					return errors.New("expected exactly one argument " +
						"with 'increase' or 'decrease'")
				}
				switch cmd.Args[0] {
				case "increase":
					err = g.IncreaseFontSize()
				case "decrease":
					err = g.DecreaseFontSize()
				default:
					return errors.New("expected exactly one argument " +
						"with 'increase' or 'decrease'")
				}
				return
			},
			completer: func(ctx context.Context, cmd textapi.Command) (
				iterator.Iterator[string], string, error,
			) {
				return iterator.FromSlice([]string{"increase", "decrease"}), "", nil
			},
		},
		{
			cmd: textapi.CommandManual{
				Name: "guicellwidth",
				Summary: "Increases or decreases the cell width of the rendered font. " +
					"To make changes permanent, update the 'gui.column-width-offset' configuration.",
				Synopsis: "(increase|decrease)",
			},
			handleCommand: func(ctx context.Context, cmd textapi.Command) (err error) {
				if len(cmd.Args) != 1 {
					return errors.New("expected exactly one argument " +
						"with 'increase' or 'decrease'")
				}
				switch cmd.Args[0] {
				case "increase":
					err = g.IncreaseCellWidth()
				case "decrease":
					err = g.DecreaseCellWidth()
				default:
					return errors.New("expected exactly one argument " +
						"with 'increase' or 'decrease'")
				}
				return
			},
			completer: func(ctx context.Context, cmd textapi.Command) (
				iterator.Iterator[string], string, error,
			) {
				return iterator.FromSlice([]string{"increase", "decrease"}), "", nil
			},
		},
		{
			cmd: textapi.CommandManual{
				Name: "guilineheight",
				Summary: "Increases or decreases the line height of the rendered font. " +
					"To make changes permanent, update the 'gui.line-height-offset' configuration.",
				Synopsis: "(increase|decrease)",
			},
			handleCommand: func(ctx context.Context, cmd textapi.Command) (err error) {
				if len(cmd.Args) != 1 {
					return errors.New("expected exactly one argument " +
						"with 'increase' or 'decrease'")
				}
				switch cmd.Args[0] {
				case "increase":
					err = g.IncreaseLineHeight()
				case "decrease":
					err = g.DecreaseLineHeight()
				default:
					return errors.New("expected exactly one argument " +
						"with 'increase' or 'decrease'")
				}
				return
			},
			completer: func(ctx context.Context, cmd textapi.Command) (
				iterator.Iterator[string], string, error,
			) {
				return iterator.FromSlice([]string{"increase", "decrease"}), "", nil
			},
		},
		{
			cmd: textapi.CommandManual{
				Name:    "guiwindownew",
				Summary: "Opens a new OS-level window by spawning a new Rune process.",
			},
			handleCommand: func(ctx context.Context, cmd textapi.Command) (err error) {
				if len(launchCmd) == 0 {
					return errors.New("unable to spawn new window: launch command not captured")
				}
				c := exec.Command(launchCmd[0], launchCmd[1:]...)
				c.Env = append(os.Environ(), "EBITENGINE_COCOA_HIDE_DOCK=1")
				if err := c.Start(); err != nil {
					return fmt.Errorf("spawn new window: %w", err)
				}
				go debug.CapturePanicReport(func() {
					_ = c.Wait()
				})
				return nil
			},
		},
	}

	for _, man := range commands {
		man := man
		err := i.SubscribeCommand(man.cmd,
			text.FuncCommandHandler(func(ctx context.Context, cmd textapi.Command) error {
				switch cmd.Name {
				case cmdSetOpacity, cmdSetBackgroundBlur:
					if !transparentEnabled {
						return errors.New("cannot change opacity if " +
							"transparent window is not enabled. Enable it by setting" +
							"`gui.enable_transparent_window` to `true` in your configuration")
					}
					fallthrough
				default:
					return man.handleCommand(ctx, cmd)
				}
			}, man.completer))
		if err != nil {
			ret = multierror.Append(ret, fmt.Errorf("subscribe '%s': %v", man.cmd.Name, err))
		}
	}
	return ret
}
