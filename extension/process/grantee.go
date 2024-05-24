package process

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
	extensionpb "unstable.build/go-tui/extension/rpc"
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
	extensionpb.RegisterGranteeServer(s, server)
	return nil
}

// GRPCClient satisfies extension.GRPCExtension
func (p *granteeExtension) GRPCClient(
	ctx context.Context, _ *goplugin.GRPCBroker, c *grpc.ClientConn,
) (interface{}, error) {
	pbClient := extensionpb.NewGranteeClient(c)
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
