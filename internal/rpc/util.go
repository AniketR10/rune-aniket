// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package rpc

import (
	"io"

	"os"

	"google.golang.org/grpc/grpclog"
)

// DisableGRPCLogging disables grpc stderr loggers.
func DisableGRPCLogging() {
	os.Setenv("GRPC_GO_LOG_SEVERITY_LEVEL", "FATAL")
	os.Setenv("GRPC_GO_LOG_VERBOSITY_LEVEL", "0")
	discard := grpclog.NewLoggerV2WithVerbosity(io.Discard, io.Discard, io.Discard, 0)
	grpclog.SetLoggerV2(discard)
}

// EnableGRPCLogging disables grpc stderr loggers.
func EnableGRPCLogging(info, warn, err io.Writer) {
	os.Setenv("GRPC_GO_LOG_SEVERITY_LEVEL", "INFO")
	os.Setenv("GRPC_GO_LOG_VERBOSITY_LEVEL", "99")
	logger := grpclog.NewLoggerV2WithVerbosity(info, warn, err, 99)
	grpclog.SetLoggerV2(logger)
}
