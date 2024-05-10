package firestore

import (
	"os"
	"strings"

	"github.com/unstablebuild/blue/config"
	uconfig "go.uber.org/config"
	"unstable.build/go-tui/workspace"
)

const defaultReferenceConfig = `
auth:
  project-id: 1
  credentials-file: 
issue:
  collection: blue-issue
`

type collectionConfig struct {
	Collection string `yaml:"collection"`
}

type authConfig struct {
	ProjectID       string `yaml:"project-id"`
	CredentialsFile string `yaml:"credentials-file"`
}

type cliConfig struct {
	Auth  authConfig       `yaml:"auth"`
	Issue collectionConfig `yaml:"issue"`
}

// rewrite config.NewProvider to use workspace.OpenFile
func newProvider(overridesConfigPath, fallbackConfigLiteral string) (
	config.Provider, error,
) {
	configDef := strings.NewReader(fallbackConfigLiteral)
	if overridesConfigPath == "" {
		return uconfig.NewYAML(uconfig.Source(configDef))
	}
	configFile, err := workspace.OpenFile(overridesConfigPath, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	return uconfig.NewYAML(uconfig.Source(configDef), uconfig.Source(configFile))
}

func sourceConfig(overridesConfigPath string) (
	*cliConfig, error,
) {
	provider, err := newProvider(
		overridesConfigPath, defaultReferenceConfig)
	if err != nil {
		return nil, err
	}

	return sourceConfigFromProvider(provider)
}

func sourceConfigFromProvider(provider config.Provider) (
	c *cliConfig, err error,
) {
	c = new(cliConfig)
	err = config.ProviderGetSections(provider,
		config.Section{Name: "auth", Target: &c.Auth},
		config.Section{Name: "issue", Target: &c.Issue},
	)
	return
}
