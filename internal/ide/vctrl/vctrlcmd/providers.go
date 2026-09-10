// Copyright (C) 2017-2026 The Rune Authors
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package vctrlcmd

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
