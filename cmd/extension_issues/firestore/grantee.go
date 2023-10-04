package firestore

import (
	"fmt"
	"os"
	"path"

	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/document/firestore"
	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui/api/config"
	issuesExtension "unstable.build/go-tui/cmd/extension_issues/extension"
	"unstable.build/go-tui/extension"
)

const (
	defaultIssuesScheme = "bluectl+issues"
)

// Grantee returns this extension's grantee and the permissions required to run it.
// It uses the local bluectl configuration to load the issue tracker's credentials.
func Grantee(versionTag string) (extension.Grantee, []extension.Permission) {
	return issuesExtension.GranteeWithService(versionTag, defaultIssuesScheme, func(pconfig config.Config) (document.Service, error) {
		bluectlConfigFile, err := pconfig.GetString("bluectl_config")
		if err != nil {
			if err != config.ErrNotFound {
				log.Warnf("could not read property 'bluectl_config': %v", err)
			}
			homeDir, err := os.UserHomeDir()
			if err != nil {
				err = fmt.Errorf("no 'bluectl_config' provided and failed "+
					"to get user home dir: %v", err)
				return nil, err
			}
			bluectlConfigFile = path.Join(homeDir, ".bluectl", "config")
		}
		cfg, err := sourceConfig(bluectlConfigFile)
		if err != nil {
			err = fmt.Errorf("source bluectl configuration: %v", err)
			return nil, err
		}
		svc, err := firestore.New(cfg.Auth.ProjectID,
			cfg.Issue.Collection, cfg.Auth.CredentialsFile)
		if err != nil {
			err = fmt.Errorf("initialize firestore: %v", err)
			return nil, err
		}
		return svc, nil
	})
}
