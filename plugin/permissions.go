package plugin

const (
	// PermissionWorkspaceFileSystem requests access to manage
	// the files in a workspace.
	PermissionFileSystem Permission = "_PermWorkspaceFileSystem"
	// PermissionWorkspaceExecute requests access to execute
	// and stop processes in a workspace.
	PermissionExecute Permission = "_PermWorkspaceExecute"
	// PermissionWorkspaceTerminal requests access to manage
	// a workspace's ptys.
	PermissionTerminal Permission = "_PermWorkspaceTerminal"

	// PermissionBrowserWindowManager requests access to a browser's window manager.
	PermissionBrowserWindowManager Permission = "_PermBrowserWindowManager"
	// PermissionBrowserResourceOpener requests access to open new files.
	PermissionBrowserResourceOpener Permission = "_PermBrowserResourceOpener"
	// PermissionBrowserMessenger requests access to send messages to the UI.
	PermissionBrowserMessenger Permission = "_PermBrowserMessenger"
	// PermissionBrowserEventPublisher requests access to publish term events.
	// This is useful if your plugin handler does async updates to its state, as
	// it enables interrupting the main event loop to redraw components.
	// TODO rename to Interrupt
	PermissionBrowserEventPublisher Permission = "_PermBrowserEventPublisher"

	// PermissionEditor requests access to the editor.
	PermissionEditor Permission = "_PermEditor"

	// PermissionStorage requests access to persistent storage.
	PermissionStorage Permission = "_PermStorage"
)
