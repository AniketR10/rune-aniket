// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2023-2024 Unstable Build, All Rights Reserved.
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

package apiclient

import (
	"fmt"
	"runtime"
	"time"
)

const (
	defaultTelemetryPeriod = 5 * time.Minute
)

// DefaultHTTPEndpointAddress and DefaultGRPCEndpointAddress are the
// API endpoints baked into the binary at build time. They default to
// the development API; release builds override them via `-ldflags -X`
// to point at the production API. They are package-level variables
// (not constants) so the linker can replace them; do not assign to
// them at runtime.
var (
	DefaultHTTPEndpointAddress = "https://api.unstable.build"
	DefaultGRPCEndpointAddress = "rpc.unstable.build:443"
)

// DefaultDownloadsHost is the public CDN origin that hosts the
// per-arch release manifest (`<host>/<os>-<arch>/manifest.json`) and
// artifacts. The in-product `:upgrade` flow reads from this host.
//
// Defaults to the staging downloads bucket. Production release builds
// override it via `-ldflags -X` to point at the prod CDN — users
// should never have to think about this. Not a constant so the linker
// can replace it; do not assign to it at runtime.
var DefaultDownloadsHost = "https://downloads.unstable.build"

// defaultReleaseCollection returns the Firestore collection name for
// the current platform, e.g. "rune-release-darwin-arm64". The prefix
// is shared across dev and prod intentionally: collections live in
// per-environment GCP projects, so there's no risk of collision and
// no need to flip the name per build.
func defaultReleaseCollection() string {
	return fmt.Sprintf("rune-release-%s-%s", runtime.GOOS, runtime.GOARCH)
}

// DefaultConfig returns the default configuration for the Grantee
// returned by NewAPI.
func DefaultConfig() Config {
	return Config{
		HTTPEndpointAddress: DefaultHTTPEndpointAddress,
		GRPCEndpointAddress: DefaultGRPCEndpointAddress,
		InsecureTransport:   false,
		TelemetryPeriod:     defaultTelemetryPeriod,
		ReleaseCollection:   defaultReleaseCollection(),
		EnableTelemetry:     false,
	}
}

// Config holds the configuration for this client
type Config struct {
	// HTTPEndpointAddress is the HTTP endpoint address of Rune's API.
	HTTPEndpointAddress string
	// GRPCEndpointAddress is the GRPC endpoint address of Rune's API.
	GRPCEndpointAddress string
	// InsecureTransport configures the grpc client to not use per-RPC
	// credentials or TLS.
	InsecureTransport bool
	// ReleaseCollection is the document collection for the package manager.
	ReleaseCollection string
	// TelemetryPeriod is how often to send statistics to the server.
	// Must be positive when EnableTelemetry is true; zero panics.
	TelemetryPeriod time.Duration
	// EnableTelemetry controls whether telemetry is active. When false,
	// no telemetry data is collected or sent.
	EnableTelemetry bool
}
