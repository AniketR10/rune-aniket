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
	"fmt"
	"os"
	"path"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/document/firestore"
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
