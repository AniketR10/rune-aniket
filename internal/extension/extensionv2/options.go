// Copyright (C) 2017-2026 The Rune Authors
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

package extensionv2

import "google.golang.org/grpc"

// WithInsecureAuth returns an option that configures
// host resources to be exposed without authentication or authorization.
func WithInsecureAuth() Option {
	return func(cfg *runnerConfig) {
		cfg.insecureAuth = true
	}
}

// WithInsecureTransport returns an option that configures
// a extension.Runner's to NOT secure communication
// between extensions and host.
func WithInsecureTransport() Option {
	return func(cfg *runnerConfig) {
		cfg.insecureTransport = true
	}
}

// WithSocketEnv returns an option that configures
// what environment variable to use to share the socket
// with ad-hoc programs.
func WithSocketEnv(env string) Option {
	return func(cfg *runnerConfig) {
		cfg.socketEnv = env
	}
}

// WithDataDirEnv returns an option that configures
// what environment variable to use to share the data directory
// with ad-hoc programs.
func WithDataDirEnv(env string) Option {
	return func(cfg *runnerConfig) {
		cfg.dataDirEnv = env
	}
}

// WithInstallDirEnv returns an option that configures
// what environment variable to use to share the install directory
// with ad-hoc programs.
func WithInstallDirEnv(env string) Option {
	return func(cfg *runnerConfig) {
		cfg.installDirEnv = env
	}
}

// WithAuthTokenEnv returns an option that configures
// what environment variable to use to share the oauth2 token
// with ad-hoc programs.
func WithAuthTokenEnv(env string) Option {
	return func(cfg *runnerConfig) {
		cfg.authTokenEnv = env
	}
}

// WithAuthCertEnv returns an option that configures
// what environment variable to use to share the tls cert
// with ad-hoc programs.
func WithAuthCertEnv(env string) Option {
	return func(cfg *runnerConfig) {
		cfg.authCertEnv = env
	}
}

// WithServerInterceptors returns an option that appends the given
// interceptors to the gRPC server's interceptor chains. Nil entries
// are allowed and skipped. This exists so out-of-tree harnesses (e.g.
// cmd/xsandbox) can observe every extension RPC without forking the
// runner; production callers do not need it.
func WithServerInterceptors(
	stream grpc.StreamServerInterceptor, unary grpc.UnaryServerInterceptor,
) Option {
	return func(cfg *runnerConfig) {
		if stream != nil {
			cfg.extraStreamInterceptors = append(cfg.extraStreamInterceptors, stream)
		}
		if unary != nil {
			cfg.extraUnaryInterceptors = append(cfg.extraUnaryInterceptors, unary)
		}
	}
}

// Option is a configuration option for a runner.
type Option func(cfg *runnerConfig)

type runnerConfig struct {
	insecureAuth            bool
	insecureTransport       bool
	authCertEnv             string
	authTokenEnv            string
	socketEnv               string
	dataDirEnv              string
	installDirEnv           string
	extraStreamInterceptors []grpc.StreamServerInterceptor
	extraUnaryInterceptors  []grpc.UnaryServerInterceptor
}
