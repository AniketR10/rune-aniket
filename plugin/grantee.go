package plugin

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/hashicorp/go-plugin"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"unstable.build/go-tui/config"
	pluginpb "unstable.build/go-tui/plugin/rpc"
	"unstable.build/go-tui/proto"
)

// Permission represents a type of resource access.
type Permission string

// Permissions is a set of Permission.
type Permissions map[Permission]struct{}

// Grant binds a granted Permission with a Token that
// can be used with the plugin API.
type Grant struct {
	Token string
	Permission
}

// Grantee needs to be implemented by plugins that want
// to access plugin host resources.
type Grantee interface {
	Connected(proto.MuxBroker, config.Config)
	PermissionGranted([]Grant)
	PermissionDenied([]Permission)
	Shutdown(reason string) error
	Health() error
}

// handshakeConfigs are used to just do a basic handshake between
// a plugin and host. If the handshake fails, a user friendly error is shown.
// This prevents users from executing bad plugins or executing a plugin
// directory. It is a UX feature, not a security feature.
var handshakeConfig = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "TUI_PLUGIN",
	MagicCookieValue: "kombucha_for_dogs",
}

const typeGranteePlugin = "tui_grantee_plugin"

type granteePlugin struct {
	plugin.Plugin
	requested []Permission
	grantee   Grantee
	grantor   Grantor
	keepAlive time.Duration
	broker    proto.MuxBroker
}

// GRPCServer satisfies plugin.GRPCPlugin
func (p *granteePlugin) GRPCServer(_ *plugin.GRPCBroker, s *grpc.Server) error {
	server := newGranteeServer(p.broker, p.grantee, p.requested, p.keepAlive)
	if log.IsLevelEnabled(log.TraceLevel) {
		server = &loggingGranteeServer{GranteeServer: server}
	}
	pluginpb.RegisterGranteeServer(s, server)
	return nil
}

// GRPCClient satisfies plugin.GRPCPlugin
func (p *granteePlugin) GRPCClient(
	ctx context.Context, _ *plugin.GRPCBroker, c *grpc.ClientConn,
) (interface{}, error) {
	pbClient := pluginpb.NewGranteeClient(c)
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
// and serves it as a plugin. This function never returns.
// It also configures logrus.StandardLogger to send logs to host.
func Serve(grantee Grantee, request ...Permission) {
	level := getLogLevelEnv()
	formatter := newJSONFormatter()

	initPluginLogger()
	SetLoggingOutput(os.Stderr)
	SetLoggingLevel(level)
	SetLoggingFormatter(formatter)

	log.SetOutput(os.Stderr)
	log.SetLevel(level)
	log.SetFormatter(formatter)

	proto.DisableGRPCLogging()

	pluginMap := map[string]plugin.Plugin{
		typeGranteePlugin: &granteePlugin{
			requested: request,
			grantee:   grantee,
			keepAlive: defaultHealthCheckTicker,
			broker:    initClientBroker(&pluginLogger, getDataDirEnv()),
		},
	}

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: handshakeConfig,
		Plugins:         pluginMap,
		Logger:          NewHCLogLogrus(&pluginLogger),
		GRPCServer:      plugin.DefaultGRPCServer,
	})
}
