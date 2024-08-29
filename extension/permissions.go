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

package extension

const (
	// PermissionFileSystem requests access to manage
	// the files in a workspace.
	PermissionFileSystem Permission = "_PermWorkspaceFileSystem"
	// PermissionExecute requests access to execute
	// and stop processes in a workspace.
	PermissionExecute Permission = "_PermWorkspaceExecute"
	// PermissionTerminal requests access to manage
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
