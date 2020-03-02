package plugin

import (
	"os/exec"

	"github.com/hashicorp/go-plugin"
	log "github.com/sirupsen/logrus"
)

func goPluginGranteeBuilder(
	pluginID, path string, grantor Grantor, logger *log.Logger,
) (*granteeClient, error) {
	pluginMap := map[string]plugin.Plugin{
		typeGranteePlugin: &granteePlugin{logger: logger, grantor: grantor},
	}

	cmd := exec.Command(path)
	cmd.Env = append(cmd.Env, pluginEnv...)
	cmd.Env = append(cmd.Env, makeEnvVar(envLogLevel, logger.Level.String()))

	config := &plugin.ClientConfig{
		HandshakeConfig:  handshakeConfig,
		Plugins:          pluginMap,
		Cmd:              cmd,
		Logger:           NewHCLogLogrus(logger),
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
		// TODO we should validate integrity of plugins
		// SecureConfig:    &secureCfg,
	}
	client := plugin.NewClient(config)

	rpcClient, err := client.Client()
	if err != nil {
		return nil, err
	}

	raw, err := rpcClient.Dispense(typeGranteePlugin)
	if err != nil {
		return nil, err
	}

	grantee := raw.(*granteeClient)
	grantee.bindPluginClient(client)

	return grantee, nil
}
