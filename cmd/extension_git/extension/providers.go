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
package extension

import (
	"errors"
	"fmt"
	"strings"
)

var _ providerResolver = stringsContainsResolver{}

type providerResolver interface {
	resolveByRemoteDomain(domainName string) (weblinkBuilder, error)
}

type weblinkBuilder interface {
	buildWeblink(parts remoteURLParts) (string, error)
}

type stringsContainsResolver struct{}

func (stringsContainsResolver) resolveByRemoteDomain(domainName string) (weblinkBuilder, error) {
	if strings.Contains(domainName, "github") {
		return github{}, nil
	}

	if strings.Contains(domainName, "gitlab") {
		return gitlab{}, nil
	}

	if strings.Contains(domainName, "bitbucket") {
		return bitbucket{}, nil
	}

	if strings.Contains(domainName, "unstable.build") {
		return gitea{}, nil
	}

	return nil, errors.New("unknown resolver")
}

// github implements weblinkBuilder by generating GitHub links
type github struct{}

func (github) buildWeblink(parts remoteURLParts) (string, error) {
	partsMap := parts.ToMap()
	weblink, err := expand("https://{domain}/{owner}/{repo}/blob/{commit}/{file}#L{line}", partsMap)
	if err != nil {
		return "", fmt.Errorf("expand: %w", err)
	}
	return weblink, nil
}

// bitbucket implements weblinkBuilder by generating Bitbucket links
type bitbucket struct{}

func (bitbucket) buildWeblink(parts remoteURLParts) (string, error) {
	partsMap := parts.ToMap()
	weblink, err := expand("https://{domain}/{owner}/{repo}/src/{commit}/source/{file}#lines-{line}", partsMap)
	if err != nil {
		return "", fmt.Errorf("expand: %w", err)
	}
	return weblink, nil
}

// gitlab implements weblinkBuilder by generating GitLab links
type gitlab struct{}

func (gitlab) buildWeblink(parts remoteURLParts) (string, error) {
	partsMap := parts.ToMap()
	weblink, err := expand("https://{domain}/{owner}/{repo}/-/blob/{commit}/{file}#L{line}", partsMap)
	if err != nil {
		return "", fmt.Errorf("expand: %w", err)
	}
	return weblink, nil
}

// gitlab implements weblinkBuilder by generating Gitea links
type gitea struct{}

func (gitea) buildWeblink(parts remoteURLParts) (string, error) {
	partsMap := parts.ToMap()
	weblink, err := expand("https://{domain}/{owner}/{repo}/src/commit/{commit}/{file}#L{line}", partsMap)
	if err != nil {
		return "", fmt.Errorf("expand: %w", err)
	}
	return weblink, nil
}
