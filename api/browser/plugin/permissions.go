package plugin

const (
	// PermissionBrowserWindowManager requests access to a browser's window manager.
	PermissionBrowserWindowManager string = "_PermBrowserWindowManager"
	// PermissionBrowserResourceOpener requests access to open new files.
	PermissionBrowserResourceOpener = "_PermBrowserResourceOpener"
	// PermissionBrowserMessenger requests access to send messages to the UI.
	PermissionBrowserMessenger = "_PermBrowserMessenger"
	// PermissionBrowserEventPublisher requests access to publish term events.
	// This is useful if your plugin handler does async updates to its state, as
	// it enables interrupting the main event loop to redraw components.
	// TODO rename to Interrupt
	PermissionBrowserEventPublisher = "_PermBrowserEventPublisher"
)
