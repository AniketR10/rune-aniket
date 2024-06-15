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

type issueConfig struct {
	Collection string `yaml:"collection"`
	Author     string `yaml:"author"`
}

type authConfig struct {
	ProjectID       string `yaml:"project-id"`
	CredentialsFile string `yaml:"credentials-file"`
}

type cliConfig struct {
	Auth  authConfig  `yaml:"auth"`
	Issue issueConfig `yaml:"issue"`
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
