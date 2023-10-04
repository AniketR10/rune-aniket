package extension

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
	// PermissionBrowserNotifications requests access to send messages to the UI.
	PermissionBrowserNotifications Permission = "_PermBrowserNotifications"
	// PermissionBrowserEventPublisher requests access to publish term events.
	// This is useful if your extension handler does async updates to its state, as
	// it enables interrupting the main event loop to redraw components.
	// TODO rename to Interrupt
	PermissionBrowserEventPublisher Permission = "_PermBrowserEventPublisher"

	// PermissionEditor requests access to the editor.
	PermissionEditor Permission = "_PermEditor"

	// PermissionStorage requests access to persistent storage.
	PermissionStorage Permission = "_PermStorage"

	// PermissionConfig requests access to read the loaded configuration.
	PermissionConfig Permission = "_PermConfig"

	// PermissionSchemeManager requests access to the workspace's URI scheme manager.
	PermissionSchemeManager Permission = "_PermSchemeManager"
)
