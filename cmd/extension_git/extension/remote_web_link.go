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
	"regexp"
	"strconv"
	"strings"
)

// extracts protocol, domain, owner and repo name from a git remote URL.
var gitRemoteRegex = regexp.MustCompile(`^(?:(https)://|(git)\@)([^/:]+)[:/]([^/]+)/([\w-]+)(?:\.git)?$`)

func remoteWebLink(
	git gitService, remoteName string, workPath string, line int,
) (string, error) {
	// line of the document starts at 0 whereas in text editors start at 1
	correctedLine := line + 1

	remoteURL, err := git.remoteURL(workPath, remoteName)
	if err != nil {
		return "", fmt.Errorf("remote url: %w", err)
	}
	if remoteURL == "" {
		return "", errors.New("empty parsed remote url")
	}

	urlParts, err := parseRemoteURL(remoteURL)
	if err != nil {
		return "", fmt.Errorf("parse remote url: %w", err)
	}

	currentCommit, err := git.currentCommit(workPath)
	if err != nil {
		return "", fmt.Errorf("current commit: %w", err)
	}

	fileRelPath, err := git.relPath(workPath)
	if err != nil {
		return "", fmt.Errorf("rel path: %w", err)
	}

	urlParts.commit = currentCommit
	urlParts.file = fileRelPath
	urlParts.line = correctedLine

	urlPartsMap := urlParts.ToMap()
	webLink, err := expand("https://{domain}/{owner}/{repo}/src/commit/{commit}/{file}#L{line}", urlPartsMap)
	if err != nil {
		return "", fmt.Errorf("expand tpl map: %w", err)
	}

	return webLink, nil
}

type remoteURLParts struct {
	domain string
	owner  string
	repo   string
	commit string
	file   string
	line   int
}

func (r remoteURLParts) ToMap() map[string]string {
	return map[string]string{
		"domain": r.domain,
		"owner":  r.owner,
		"repo":   r.repo,
		"commit": r.commit,
		"file":   r.file,
		"line":   strconv.Itoa(r.line),
	}

}

// parseRemoteURL parses domain, owner, repo from a git SSH or HTTPS URL.
func parseRemoteURL(remoteURL string) (remoteURLParts, error) {
	// Match HTTPS URLs as well as git SSH URLs like the ones below:
	//
	// - git@git.unstable.build:unstablebuild/go-tui.git
	// - https://git.unstable.build/unstablebuild/go-tui.git
	//
	matches := gitRemoteRegex.FindStringSubmatch(remoteURL)
	if len(matches) != 6 {
		return remoteURLParts{}, errors.New("parse remote url")
	}

	_ = matches[2] //  holds the protocol: "https" or "git"

	ret := remoteURLParts{}
	ret.domain = matches[3]
	ret.owner = matches[4]
	ret.repo = matches[5]

	return ret, nil
}

// expand rewrites s to replace {k} with match[k] for each key k in match. All
// template `{placeholders}` must be filled otherwise error is returned.
func expand(s string, match map[string]string) (string, error) {
	oldNew := make([]string, 0, 2*len(match))
	for k, v := range match {
		oldNew = append(oldNew, "{"+k+"}", v)
	}
	ret := strings.NewReplacer(oldNew...).Replace(s)
	if strings.Contains(ret, "{") || strings.Contains(ret, "}") {
		return "", fmt.Errorf("template left with empty placeholders: %s", ret)
	}
	return ret, nil
}
