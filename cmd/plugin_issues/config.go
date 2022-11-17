package main

import (
	"github.com/ernestrc/blue/config"
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

func sourceConfig(overridesConfigPath string) (
	*cliConfig, error,
) {
	provider, err := config.NewProvider(
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
