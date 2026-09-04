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

package shop

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"

	"unstable.build/rune/handler/command"
)

func (r *Root) commandManuals() []command.Manual {
	return []command.Manual{
		{Name: "windowclose", Summary: "Close the current active window and switch focus to the next available window. This command fails if there is only one window remaining."},
		{Name: "windowcloseall", Summary: "Close all windows except the current active window. This command fails if there is only one window remaining."},
		{Name: "windowdefaultsplit", Summary: "Toggle the default split orientation, or set it to the given orientation if passed via arguments. The options are `horizontal`, which places the next split below the current active window, or `vertical`, which places the next split to the right of the current active window.", Synopsis: "(horizontal|vertical)"},
		{Name: "windownew", Summary: "Split the current active window vertically or horizontally in two, moving focus to the new window. If no orientation is passed, the default split orientation is used. See `windowdefaultsplit` for more details on how the default orientation works.", Synopsis: "[right|left|up|down]"},
		{Name: "windowfocus", Summary: "Switch focus to the window on the given side of the current active window.", Synopsis: "(right|left|up|down)"},
		{Name: "windowmove", Summary: "Move the content of the window in focus to the window in the given direction.", Synopsis: "(right|left|up|down)"},
		{Name: "shell", Summary: "Open a new shell in a durable tab and route commands through registered REPL handlers."},
		{Name: "view", Summary: "Like `edit` but opens the file in read-only mode.", Synopsis: "[<page>]"},
		{Name: "fexplorer", Summary: "Toggle the file explorer floating window. The file explorer is a pre-minimized floating window on the left side. Invoking this command will un-minimize and focus the file explorer, or minimize it back if it is already open."},
		{Name: "tabprevious", Summary: "Set the content of the current active window to the previous tab in the tabs list. Wraps around to the end of the tabs list."},
		{Name: "tabnext", Summary: "Set the content of the current active window to the next tab in the tabs list. Wraps around to the start of the tabs list."},
		{Name: "tabclose", Summary: "Close the current active window's tab. It is automatically replaced with the next available tab in the tabs list."},
		{Name: "tabfocus", Summary: "Set the content of the current active window to the tab at the given position in the tabs list.", Synopsis: "[<position>]"},
	}
}

func (r *Root) completePromptCommand(
	_ context.Context, args []string,
) (iterator.Iterator[string], string, error) {
	if len(args) == 0 {
		return iterator.Empty[string](), "", nil
	}
	cmd := args[0]
	var opts []string
	switch cmd {
	case "windownew", "windowfocus", "windowmove":
		if len(args) <= 2 {
			opts = []string{"right", "left", "up", "down"}
		}
	case "windowdefaultsplit":
		if len(args) <= 2 {
			opts = []string{"horizontal", "vertical"}
		}
	case "view":
		if len(args) <= 2 {
			opts = r.allPageNames()
		}
	case "tabfocus":
		if len(args) <= 2 {
			opts = tabIndexOptions(r.b)
		}
	}
	if len(opts) == 0 {
		return iterator.Empty[string](), "", nil
	}
	return iterator.FromSlice(opts), "", nil
}

func (r *Root) dispatchPromptCommand(cmd string, args ...string) bool {
	if r.cmd != nil && r.cmdPrev != nil && !r.cmdPrev.Closed() {
		_ = r.b.SetFocus(r.cmdPrev)
	}
	if err := r.runPromptCommand(cmd, args...); err != nil {
		r.log.Warn("prompt command", "cmd", cmd, "args", args, "err", err)
	}
	return r.isKnownPromptCommand(cmd)
}

func (r *Root) isKnownPromptCommand(cmd string) bool {
	switch cmd {
	case "windowclose", "windowcloseall", "windowdefaultsplit", "windownew",
		"windowfocus", "windowmove", "shell", "view", "fexplorer",
		"tabprevious", "tabnext", "tabclose", "tabfocus":
		return true
	default:
		return false
	}
}

func (r *Root) runPromptCommand(cmd string, args ...string) error {
	switch cmd {
	case "windowclose":
		return r.windowclose()
	case "windowcloseall":
		return r.windowcloseall()
	case "windowdefaultsplit":
		return r.windowdefaultsplit(args...)
	case "windownew":
		return r.windownew(args...)
	case "windowfocus":
		return r.windowfocus(args...)
	case "windowmove":
		return r.windowmove(args...)
	case "shell":
		return r.showShell()
	case "view":
		return r.view(args...)
	case "fexplorer":
		return r.toggleFileExplorer()
	case "tabprevious":
		return r.tabprevious()
	case "tabnext":
		return r.tabnext()
	case "tabclose":
		return r.tabclose()
	case "tabfocus":
		return r.tabfocus(args...)
	default:
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func (r *Root) view(args ...string) error {
	if len(args) == 0 {
		return errors.New("view expects a page name")
	}
	return r.showPageByName(args[0])
}

func (r *Root) windowclose() error {
	return r.invokeWindow().Close()
}

func (r *Root) windowcloseall() error {
	return r.b.CloseOtherWindows(r.invokeWindow())
}

func (r *Root) windowdefaultsplit(args ...string) error {
	if len(args) == 0 {
		return errors.New("command expects argument 'horizontal', 'h', 'vertical', 'v'")
	}
	switch args[0] {
	case "horizontal", "h":
		r.b.SetDefaultSplit(browserapi.OrientationBottom)
	case "vertical", "v":
		r.b.SetDefaultSplit(browserapi.OrientationRight)
	default:
		return fmt.Errorf("invalid orientation %q", args[0])
	}
	return nil
}

func (r *Root) windownew(args ...string) error {
	o, err := parseOrientationArg(args...)
	if err != nil {
		return err
	}
	if _, ok := r.b.Split(o, r.invokeWindow(), nil); !ok {
		return errors.New("could not create window split")
	}
	return nil
}

func (r *Root) windowfocus(args ...string) error {
	if len(args) == 0 {
		return errors.New("command expects at least one argument")
	}
	switch args[0] {
	case "right":
		r.b.FocusRight()
	case "down":
		r.b.FocusDown()
	case "left":
		r.b.FocusLeft()
	case "up":
		r.b.FocusUp()
	default:
		return fmt.Errorf("invalid argument %q", args[0])
	}
	return nil
}

func (r *Root) windowmove(args ...string) error {
	if len(args) == 0 {
		return errors.New("command expects at least one argument")
	}
	var ok bool
	switch args[0] {
	case "right":
		ok = r.b.SwapContentRight()
		if ok {
			r.b.FocusRight()
		}
	case "down":
		ok = r.b.SwapContentDown()
		if ok {
			r.b.FocusDown()
		}
	case "left":
		ok = r.b.SwapContentLeft()
		if ok {
			r.b.FocusLeft()
		}
	case "up":
		ok = r.b.SwapContentUp()
		if ok {
			r.b.FocusUp()
		}
	default:
		return fmt.Errorf("invalid argument %q", args[0])
	}
	if !ok {
		return errors.New("cannot move window in this direction")
	}
	return nil
}

func (r *Root) tabprevious() error {
	r.b.PreviousTab(r.invokeWindow())
	return nil
}

func (r *Root) tabnext() error {
	r.b.NextTab(r.invokeWindow())
	return nil
}

func (r *Root) tabclose() error {
	r.b.RemoveWindowContent(r.invokeWindow())
	return nil
}

func (r *Root) tabfocus(args ...string) error {
	if len(args) < 1 {
		return errors.New("invalid tab")
	}
	idx, err := strconv.Atoi(args[0])
	if err != nil {
		return errors.New("invalid tab")
	}
	if idx == 0 {
		return errors.New("the first tab is 1")
	}
	if !r.b.SetContentToTab(r.invokeWindow(), idx-1) {
		return errors.New("invalid tab")
	}
	return nil
}

func parseOrientationArg(args ...string) (browserapi.Orientation, error) {
	o := browserapi.OrientationDefault
	if len(args) == 0 {
		return o, nil
	}
	switch args[0] {
	case "right":
		return browserapi.OrientationRight, nil
	case "down":
		return browserapi.OrientationBottom, nil
	case "left":
		return browserapi.OrientationLeft, nil
	case "up":
		return browserapi.OrientationTop, nil
	default:
		return o, fmt.Errorf("invalid orientation argument %q", args[0])
	}
}
