package extension

import (
	textapi "unstable.build/go-tui/api/text"
	"unstable.build/go-tui/extension"
	extutil "unstable.build/go-tui/extension/util"
)

// Grantee returns this extension's Grantee and the permissions required to run it.
func Grantee() (extension.Grantee, []extension.Permission) {
	return extutil.NewEditorEventHandler(ChaosHandlerCommands, newChaosCommandHandler,
		ChaosHandlerEvents, ChaosHandlerPermissions...)
}

var (
	// ChaosHandlerCommands returns the commands that this extension is
	// interested in registering.
	ChaosHandlerCommands = []textapi.CommandManual{
		{
			Name: commandChaosHandler,
			Summary: "Creates a new split window with a broken TUI handler." +
				"Two modes can be specified (panic or slow)." +
				"If no argument is passed then 'panic' is assumed.",
			Synopsis: "[(panic|slow [duration])]",
		},
		{
			Name: commandChaosUpdateEventLatency,
			Summary: "Updates the (added) latency of the event subscriber " +
				"installed by the chaos extension. If no duration is passed, then " +
				"this acts as a reset to zero, which is the default.",
			Synopsis: "[duration]",
		},
		{
			Name: commandChaosUpdateCommandLatency,
			Summary: "Updates the (added) latency of the chaos extension's command handler. " +
				"If no duration is passed, then this acts as a reset to zero, " +
				"which is the default. Note that this hinders the ability",
			Synopsis: "[duration]",
		},
	}

	// ChaosHandlerEvents returns the events that this extension is
	// interested in subscribing to.
	ChaosHandlerEvents = []textapi.EventType{
		textapi.EventTypeOpen,
		textapi.EventTypeClose,
		textapi.EventTypeEdit,
		textapi.EventTypeFlush,
		textapi.EventTypeScroll,
		textapi.EventTypeFocus,
		textapi.EventTypeUnfocus,
		textapi.EventTypeCursor,
	}
	// ChaosHandlerPermissions are the required permissions for this
	// extension to run.
	ChaosHandlerPermissions = []extension.Permission{
		extension.PermissionBrowserWindowManager,
		extension.PermissionEditor,
	}
)
