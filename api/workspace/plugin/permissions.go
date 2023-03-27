package plugin

const (
	// PermissionWorkspaceFileSystem requests access to manage
	// the files in a workspace.
	PermissionFileSystem string = "_PermWorkspaceFileSystem"
	// PermissionWorkspaceExecute requests access to execute
	// and stop processes in a workspace.
	PermissionExecute string = "_PermWorkspaceExecute"
	// PermissionWorkspaceTerminal requests access to manage
	// a workspace's ptys.
	PermissionTerminal string = "_PermWorkspaceTerminal"
)
