package plugin

import (
	"os/exec"

	"github.com/hashicorp/go-plugin"
)

func goPluginGranteeBuilder(pluginID, path string, grantor Grantor) (
	*granteeClient, error,
) {
	pluginMap := map[string]plugin.Plugin{
		typeGranteePlugin: &granteePlugin{grantor: grantor},
	}

	cmd := exec.Command(path)
	cmd.Env = append(cmd.Env, pluginEnv...)
	cmd.Env = append(cmd.Env, makeEnvVar(envLogLevel, pluginLogger.Level.String()))

	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig:  handshakeConfig,
		Plugins:          pluginMap,
		Cmd:              cmd,
		Logger:           NewHCLogLogrus(pluginLogger),
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
		// TODO we should validate integrity of plugins
		// SecureConfig:    &secureCfg,
	})

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
