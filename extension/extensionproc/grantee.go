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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	goplugin "github.com/hashicorp/go-plugin"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"unstable.build/go-tui/extension"
	"unstable.build/go-tui/extension/extensionrpc"
	"unstable.build/go-tui/rpc"
)

// handshakeConfigs are used to just do a basic handshake between
// a extension and host. If the handshake fails, a user friendly error is shown.
// This prevents users from executing bad extensions or executing a extension
// directory. It is a UX feature, not a security feature.
var handshakeConfig = goplugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "TUI_PLUGIN",
	MagicCookieValue: "kombucha_for_dogs",
}

const typeGranteeExtension = "tui_grantee_extension"

type granteeExtension struct {
	goplugin.Plugin
	requested []extension.Permission
	grantee   extension.Grantee
	grantor   extension.Grantor
	keepAlive time.Duration
	broker    rpc.MuxBroker
}

// GRPCServer satisfies extension.GRPCExtension
func (p *granteeExtension) GRPCServer(_ *goplugin.GRPCBroker, s *grpc.Server) error {
	server := newGranteeServer(s, p.broker, p.grantee, p.requested, p.keepAlive)
	if log.IsLevelEnabled(log.TraceLevel) {
		server = &loggingGranteeServer{GranteeServer: server}
	}
	extensionrpc.RegisterGranteeServer(s, server)
	return nil
}

// GRPCClient satisfies extension.GRPCExtension
func (p *granteeExtension) GRPCClient(
	ctx context.Context, _ *goplugin.GRPCBroker, c *grpc.ClientConn,
) (interface{}, error) {
	pbClient := extensionrpc.NewGranteeClient(c)
	if log.IsLevelEnabled(log.TraceLevel) {
		pbClient = &loggingGranteeClient{GranteeClient: pbClient}
	}
	client := newGranteeClient(p.broker, pbClient)
	return client, nil
}

func getLogLevelEnv() log.Level {
	addrStr := os.Getenv(envLogLevel)
	if addrStr == "" {
		return log.InfoLevel
	}
	l, err := log.ParseLevel(addrStr)
	if err != nil {
		panic(fmt.Sprintf("failed to parse log level: %s", err))
	}
	return l
}

func getDataDirEnv() string {
	dataDir := os.Getenv(envDataDir)
	if dataDir == "" {
		return os.TempDir()
	}
	return dataDir
}

// Serve attempts to request the given permissions for Grantee
// and serves it as a extension. This function never returns.
// It also configures logrus.StandardLogger to send logs to host.
func Serve(grantee extension.Grantee, request ...extension.Permission) {
	level := getLogLevelEnv()
	formatter := newJSONFormatter()

	initExtensionLogger()
	SetLoggingOutput(os.Stderr)
	SetLoggingLevel(level)
	SetLoggingFormatter(formatter)

	log.SetOutput(os.Stderr)
	log.SetLevel(level)
	log.SetFormatter(formatter)

	rpc.DisableGRPCLogging()

	extensionExecutable := filepath.Base(os.Args[0])

	dataDir := getDataDirEnv()
	pluginMap := map[string]goplugin.Plugin{
		typeGranteeExtension: &granteeExtension{
			requested: request,
			grantee:   grantee,
			keepAlive: defaultHealthCheckTicker,
			broker:    initClientBroker(&extensionLogger, dataDir, extensionExecutable, ""),
		},
	}

	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: handshakeConfig,
		Plugins:         pluginMap,
		Logger:          newHCLogLogrus(extensionExecutable, &extensionLogger),
		GRPCServer: func(opts []grpc.ServerOption) *grpc.Server {
			if dataDir != "" {
				// do not panic, and so allow extension manager to
				// process errors, rather than client read EOF
				const shouldPanic = false
				opts = append(opts, grpc.ChainUnaryInterceptor(
					rpc.UnaryReportRecoveryInterceptor(
						dataDir, extensionExecutable, "", shouldPanic),
				))
				opts = append(opts, grpc.ChainStreamInterceptor(
					rpc.StreamReportRecoveryInterceptor(
						dataDir, extensionExecutable, "", shouldPanic),
				))
			}
			return goplugin.DefaultGRPCServer(opts)
		},
	})
}
