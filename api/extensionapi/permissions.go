// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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

package extensionapi

// Permission represents a request to access a resource.
type Permission string

const (
	// PermissionFileSystem requests access to manage
	// the files in a workspace.
	PermissionFileSystem Permission = "permfs"
	// PermissionExecute requests access to execute
	// and stop processes in a workspace.
	PermissionExecute Permission = "permexec"
	// PermissionTerminal requests access to manage a workspace's ptys.
	PermissionTerminal Permission = "permpty"
	// PermissionBrowserWindowManager requests access to a browser's window manager.
	PermissionBrowserWindowManager Permission = "permwm"
	// PermissionBrowserResourceOpener requests access to open new files.
	PermissionBrowserResourceOpener Permission = "permopen"
	// PermissionNotifications requests access to send messages to the UI.
	PermissionNotifications Permission = "permnoti"
	// PermissionInterrupt requests access to interrupt the event loop.
	// This is useful if your extension handler does async updates to its state, as
	// it enables interrupting the main event loop to redraw components.
	PermissionInterrupt Permission = "permint"
	// PermissionEditor requests access to the editor.
	PermissionEditor Permission = "permed"
	// PermissionCommands requests access to registering new commands.
	PermissionCommands Permission = "permcmd"
	// PermissionStorage requests access to persistent storage.
	PermissionStorage Permission = "permstore"
	// PermissionConfig requests access to read the loaded workspace configuration.
	PermissionConfig Permission = "permcfg"
)

// Permissions is a set of Permission.
type Permissions map[Permission]any

// NewPermissions builds a new set of Permission with the given permissions.
func NewPermissions(perms ...Permission) Permissions {
	ret := Permissions{}
	for _, perm := range perms {
		ret[perm] = nil
	}
	return ret
}

// AllPermissions returns a set of all the permissions.
func AllPermissions() Permissions {
	return Permissions{
		PermissionFileSystem:            nil,
		PermissionExecute:               nil,
		PermissionTerminal:              nil,
		PermissionBrowserWindowManager:  nil,
		PermissionBrowserResourceOpener: nil,
		PermissionNotifications:         nil,
		PermissionInterrupt:             nil,
		PermissionEditor:                nil,
		PermissionCommands:              nil,
		PermissionStorage:               nil,
		PermissionConfig:                nil,
	}
}
