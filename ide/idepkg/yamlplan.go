// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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

package idepkg

import (
	"fmt"

	"github.com/unstablebuild/blue/release"
	"gopkg.in/yaml.v3"
)

type configChangePlan struct {
	// prompt is true only when conflictCfg is non-empty.
	prompt       bool
	missingYAML  []byte
	userDoc      *yaml.Node
	pkgDoc       *yaml.Node
	autoApplyDoc *yaml.Node
}

func planConfigChange(
	pkgConfigFile string, pkgConfigData []byte,
	userCfg map[string]any,
	pkgID string, pkgVersion release.Version,
	dataDir, editorMode string,
) (configChangePlan, error) {
	runeVarMapping := func(key string) (string, bool) {
		switch key {
		case "RUNE_DATADIR":
			return dataDir, true
		case "RUNE_PKG_ID":
			return pkgID, true
		case "RUNE_PKG_VERSION":
			return string(pkgVersion), true
		}
		// Leave $key literal in the merged config so it is expanded
		// later at Rune startup against the resolved environment.
		return "", false
	}

	pkgOverlayCfg, err := loadIdePkgConfigOverlay(
		pkgConfigFile, pkgConfigData, map[string]any{},
		pkgID, pkgVersion, dataDir, editorMode,
	)
	if err != nil {
		return configChangePlan{}, fmt.Errorf("decode package config: %w", err)
	}
	if len(pkgOverlayCfg) == 0 {
		return configChangePlan{}, nil
	}

	versionDependent, err := versionDependentOverlayKeys(
		pkgConfigFile, pkgConfigData, pkgOverlayCfg,
		pkgID, dataDir, editorMode,
	)
	if err != nil {
		return configChangePlan{}, err
	}
	expandMapValues(pkgOverlayCfg, runeVarMapping)

	newCfg, conflictCfg := idePkgConfigDiff(userCfg, pkgOverlayCfg, versionDependent, nil)
	if newCfg == nil && conflictCfg == nil {
		return configChangePlan{}, nil
	}

	userDoc, err := mapToYAMLDocument(userCfg)
	if err != nil {
		return configChangePlan{}, fmt.Errorf("user config to yaml: %w", err)
	}

	plan := configChangePlan{userDoc: userDoc}

	if newCfg != nil {
		autoApplyDoc, err := mapToYAMLDocument(newCfg)
		if err != nil {
			return configChangePlan{}, fmt.Errorf("auto-apply config to yaml: %w", err)
		}
		plan.autoApplyDoc = autoApplyDoc
	}

	if conflictCfg != nil {
		pkgDoc, err := mapToYAMLDocument(conflictCfg)
		if err != nil {
			return configChangePlan{}, fmt.Errorf("package config to yaml: %w", err)
		}
		missingYAML, err := yaml.Marshal(conflictCfg)
		if err != nil {
			return configChangePlan{}, fmt.Errorf("marshal conflicting keys: %w", err)
		}
		plan.prompt = true
		plan.pkgDoc = pkgDoc
		plan.missingYAML = missingYAML
	}

	return plan, nil
}

type mergedConfig struct {
	yamlDoc *yaml.Node
	// starDiff is the approved diff to deep-merge into the managed
	// rune_config section. It is only populated on the .star path.
	starDiff map[string]any
}

func buildMergedConfig(
	userDoc, pkgDoc *yaml.Node, starConfig bool,
) (mergedConfig, error) {
	if starConfig {
		diff, err := loadIdePkgConfigFromYAMLDoc(pkgDoc)
		if err != nil {
			return mergedConfig{}, fmt.Errorf("decode package diff doc: %w", err)
		}
		return mergedConfig{starDiff: diff}, nil
	}
	applyConfigDiff(userDoc.Content[0], pkgDoc.Content[0])
	return mergedConfig{yamlDoc: userDoc}, nil
}
