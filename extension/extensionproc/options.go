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

package extensionproc

import (
	"sync"
	"time"

	"unstable.build/go-tui/api/workspaceapi"
	"unstable.build/go-tui/browser"
)

// WithHandshakeTimeout returns an Option which
// configures a manager to timeout extensions if handshake is not
// completed within d.
func WithHandshakeTimeout(d time.Duration) Option {
	return func(cfg *managerConfig) {
		cfg.handshakeTimeout = d
	}
}

// WithHealthTimeout returns an Option which
// configures a manager to timeout if extensions do not respond
// to health checks within d.
func WithHealthTimeout(d time.Duration) Option {
	return func(cfg *managerConfig) {
		cfg.healthCheckTicker = d
	}
}

// WithHealthRetries returns an Option which
// configures a manager to try to assert a extension's health
// up to 1 + retries before giving up.
func WithHealthRetries(retries int) Option {
	return func(cfg *managerConfig) {
		cfg.healthRetries = retries
	}
}

// WithLocker returns an Option that configures
// the manager resources sync.Locker to be the given
// locker.
func WithLocker(locker sync.Locker) Option {
	return func(cfg *managerConfig) {
		cfg.locker = locker
	}
}

// WithWorkspace returns an option that configures the
// workspace directory.
func WithWorkspace(uri workspaceapi.URI) Option {
	return func(cfg *managerConfig) {
		cfg.workspace = uri
	}
}

// WithDataDir returns an option that configures the
// storage directory.
func WithDataDir(dataDir string) Option {
	return func(cfg *managerConfig) {
		cfg.dataDir = dataDir
	}
}

// WithPackageName returns an option that configures the
// package name used for creating panic reports.
func WithPackageName(pkg string) Option {
	return func(cfg *managerConfig) {
		cfg.pkg = pkg
	}
}

// WithPackageVersion returns an option that configures the
// package version used for creating panic reports.
func WithPackageVersion(version string) Option {
	return func(cfg *managerConfig) {
		cfg.version = version
	}
}

// WithNotifications returns an option that configures
// a extension.Manager's notifications.
func WithNotifications(n browser.Notifications) Option {
	return func(cfg *managerConfig) {
		cfg.notifications = n
	}
}
