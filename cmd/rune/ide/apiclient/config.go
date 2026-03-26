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

import "time"

const (
	defaultReleaseCollection = "blue-release-bundles"

	defaultGRPCEndpointAddress = "rpc.unstable.build:443"

	defaultHTTPEndpointAddress = "https://api.unstable.build"

	defaultTelemetryPeriod = 30 * time.Second
)

// DefaultConfig returns the default configuration for the Grantee
// returned by NewAPI.
func DefaultConfig() Config {
	return Config{
		HTTPEndpointAddress: defaultHTTPEndpointAddress,
		GRPCEndpointAddress: defaultGRPCEndpointAddress,
		InsecureTransport:   false,
		TelemetryPeriod:     defaultTelemetryPeriod,
		ReleaseCollection:   defaultReleaseCollection,
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
	// TelemetryPeriod is how often do we send statistics to the server.
	TelemetryPeriod time.Duration
}
