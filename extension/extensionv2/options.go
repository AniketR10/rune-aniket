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


package extensionv2

// WithPackageName returns an option that configures the
// package name used for creating panic reports.
func WithPackageName(pkg string) Option {
	return func(cfg *runnerConfig) {
		cfg.pkg = pkg
	}
}

// WithPackageVersion returns an option that configures the
// package version used for creating panic reports.
func WithPackageVersion(version string) Option {
	return func(cfg *runnerConfig) {
		cfg.version = version
	}
}

// WithInsecureAuth returns an option that configures
// host resources to be exposed without authentication or authorization.
func WithInsecureAuth() Option {
	return func(cfg *runnerConfig) {
		cfg.insecureAuth = true
	}
}

// WithInsecureTransport returns an option that configures
// a extension.Manager's to NOT secure communication
// between extensions and host.
func WithInsecureTransport() Option {
	return func(cfg *runnerConfig) {
		cfg.insecureTransport = true
	}
}

// Option is a configuration option for a runner.
type Option func(cfg *runnerConfig)

type runnerConfig struct {
	pkg               string
	version           string
	insecureAuth      bool
	insecureTransport bool
}
