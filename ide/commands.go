package ide

import (
	"fmt"

	textapi "unstable.build/go-tui/api/text"
)

const (
	cmdEdit                   = "edit"
	cmdChangeSplitOrientation = "changeSplitOrientation"
	cmdSplitWindow            = "splitWindow"
	cmdNewWindow              = "newWindow"
	cmdSetDefaultColors       = "setDefaultColors"
)

type commandAll struct {
	man     textapi.CommandManual
	handler func(*ex, ...string) error
}

var (
	// these commands are treated specially in that
	// they're not delegated first to handler in focus
	windowControlCommands = map[string]struct{}{
		"changeSplitOrientation": {},
		cmdSplitWindow:           {},
		cmdNewWindow:             {},
		"focusNextWindow":        {},
		"focusPrevWindow":        {},
		"focusAboveWindow":       {},
		"focusBelowWindow":       {},
		"toggleFullscreen":       {},
		"newTerminal":            {},
		cmdSwitchToWorkspace:     {}, // workspace_handler
		"closeTab":               {},
		"closeWindow":            {},
		"previousTab":            {},
		"nextTab":                {},
	}
	exCommands = map[string]commandAll{
		"renameTab": {
			man: textapi.CommandManual{
				Summary:  "Rename the current tab in focus.",
				Synopsis: "name",
			},
			handler: (*ex).renameTab,
		},
		"previousTab": {
			man: textapi.CommandManual{
				Summary: "Set the content of the current active window to the previous tab in the tabs list. " +
					"Wraps around the start of the tabs list.",
			},
			handler: (*ex).previousTab,
		},
		"nextTab": {
			man: textapi.CommandManual{
				Summary: "Set the content of the current active window to the next tab in the tabs list. " +
					"Wraps around the end of the tabs list.",
			},
			handler: (*ex).nextTab,
		},
		"closeTab": {
			man: textapi.CommandManual{
				Summary: "Close the current active window's tab. It automatically replaces it " +
					"with the next available tab in the tabs list.",
			},
			handler: (*ex).closeTab,
		},
		"closeAllTabs": {
			man: textapi.CommandManual{
				Summary: "Closes all tabs in the tabs list.",
			},
			handler: (*ex).closeAllTabs,
		},
		"closeWindow": {
			man: textapi.CommandManual{
				Summary: "Closes the current active window and switches focus " +
					"to the next available window. This command fails if there's only one " +
					"window remaining.",
			},
			handler: (*ex).closeFocusWindow,
		},
		"writeQuit": {
			man: textapi.CommandManual{
				Summary: "Write the current file to disk and exits if and only if " +
					"there are no files with changes pending to be written to disk.",
			},
			handler: (*ex).flushClose,
		},
		"writeForceQuit!": {
			man: textapi.CommandManual{
				Summary: "Write the current file to disk and exits. If there are files with " +
					"pending changes, these are ignored and stashed away.",
			},
			handler: (*ex).flushCloseIgnoreNonFlushed,
		},
		"write": {
			man: textapi.CommandManual{
				Summary: "Write the current file to disk with any pending changes along with it. " +
					"This is the standard way to save changes to a file. It fails if file was " +
					"open read-only or when there is another reason why the file can't be written.",
			},
			handler: (*ex).forceFlush,
		},
		// same behaviour as write but users used to w! trigger writeForceQuit!
		// exit when using that shorthand
		"forceWrite!": {
			man: textapi.CommandManual{
				Summary: "Write the current file to disk with any pending changes along with it. " +
					"This is the standard way to save changes to a file. It fails if file was " +
					"open read-only or when there is another reason why the file can't be written.",
			},
			handler: (*ex).forceFlush,
		},
		"forceQuit!": {
			man: textapi.CommandManual{
				Summary: "Exit without writing any pending changes to disk.",
			},
			handler: (*ex).forceQuit,
		},
		"quit": {
			man: textapi.CommandManual{
				Summary: "Exit if and only if there are no files with changes pending to be " +
					"written to disk.",
			},
			handler: (*ex).quit,
		},
		"reloadFile": {
			man: textapi.CommandManual{
				Summary: "Reloads the file in the current active window, if it is a workspace file.",
			},
			handler: (*ex).reloadFile,
		},
		cmdChangeSplitOrientation: {
			man: textapi.CommandManual{
				Summary: "Toggle the next window split orientation or change it to the " +
					"given orientation if passed via arguments. The options are 'horizontal' which " +
					"sets the next split to be below the current active window or 'vertical' which " +
					"sets the next split to be right of the current active window.",
				Synopsis: "[horizontal|vertical]",
			},
			handler: (*ex).splitDirectionChange,
		},
		cmdSplitWindow: manSplitWindow,
		cmdNewWindow:   manSplitWindow,
		"focusNextWindow": {
			man: textapi.CommandManual{
				Summary: "Switches the window focus to the window on the right-side of the current active window.",
			},
			handler: (*ex).focusNextWindow,
		},
		"focusPrevWindow": {
			man: textapi.CommandManual{
				Summary: "Switches the window focus to the window on the left-side of the current active window.",
			},
			handler: (*ex).focusPrevWindow,
		},
		"focusAboveWindow": {
			man: textapi.CommandManual{
				Summary: "Switches the window focus to the window above of the current active window.",
			},
			handler: (*ex).focusAboveWindow,
		},
		"focusBelowWindow": {
			man: textapi.CommandManual{
				Summary: "Switches the window focus to the window below of the current active window.",
			},
			handler: (*ex).focusBelowWindow,
		},
		"toggleFullscreen": {
			man: textapi.CommandManual{
				Summary: "Sets the contents of the current active window to full-screen. A subsequent invocation of this command will effectively undo this.",
			},
			handler: (*ex).toggleFullscreen,
		},
		"notificationsCloseAll": {
			man: textapi.CommandManual{
				Summary: "Closes all active notifications rendered by the browser.",
			},
			handler: (*ex).closeNotifications,
		},
		"notificationsPauseAll": {
			man: textapi.CommandManual{
				Summary: "Pauses automatic closure of all active notifications rendered by the browser.",
			},
			handler: (*ex).pauseNotifications,
		},
		"notificationsResumeAll": {
			man: textapi.CommandManual{
				Summary: "Resumes automatic closure of all previously paused notifications rendered by the browser.",
			},
			handler: (*ex).resumeNotifications,
		},
		"panic": {
			man: textapi.CommandManual{
				Summary: "Causes the editor to panic. This is internal and for debugging purposes only.",
			},
			handler: (*ex).panic,
		},
		"newTerminal": {
			man: textapi.CommandManual{
				Summary: "Opens a new terminal emulator and starts a shell on the current " +
					"active window, replacing its contents. If 'shell' is not set in " +
					"terminal config, then the system shell defined in the SHELL " +
					"environment variable is used.",
			},
			handler: (*ex).newTerminalTab,
		},
		cmdEdit: {
			man: textapi.CommandManual{
				Summary: "Opens the given file URI for editing on the current active " +
					"window, replacing its contents. If no scheme is provided, file:// " +
					"is used by default. This allows a user opening files in " +
					"workspaces outside the current workspace or host. " +
					"A .swp file is created in the same folder to prevent multiple sessions " +
					"from overriding each others changes. " +
					"If file has any pending changes that were lost due to a crash or " +
					"there's another session currently editing the file, a prompt is opened " +
					"to come to a decision.",
				Synopsis: "[scheme:][//[userinfo@]host][/]filepath",
			},
			handler: (*ex).editFiles,
		},
		"!": {
			man: textapi.CommandManual{
				Summary: "Runs an executable, whether its a plugin or not, on an " +
					"ephemeral terminal emulator. The stdout and stderr of the execution " +
					"are printed on a floating window along with stats and a progress sign until " +
					"user closes the window or hits the ESC key. " +
					"If not executable is passed, the companion terminal emulator is opened. " +
					"This emulator is different " +
					"than a terminal emulator created by newTerminal in that it preserves " +
					"the session output accross invocations. The floating window created as a " +
					"result of this command can be closed via standard window or tab close commands.",
				Synopsis: "[executable [args]]",
			},
			handler: (*ex).executePlugin,
		},
		cmdSetDefaultColors: {
			man: textapi.CommandManual{
				Summary: "Changes the default background and optionally foreground colors of" +
					"the window in focus. The color can be a named color or an RGB value " +
					"in hexadecimal notation (i.e. #FFFFFF).",
				Synopsis: "background [foreground]",
			},
			handler: (*ex).setDefaultColors,
		},
	}

	manSplitWindow = commandAll{
		man: textapi.CommandManual{
			Summary: "Splits the current active window vertically or horizontally in two, " +
				"changing the window focus to it. " +
				"If no orientation is passed, the default split orientation is used. " +
				fmt.Sprintf("Check %s for more details on how changing the "+
					"default orientation works.", cmdChangeSplitOrientation),
			Synopsis: "[right|left|top|bottom]",
		},
		handler: (*ex).newWindow,
	}
)
