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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDomainNameProviderResolver(t *testing.T) {
	tsuite := []struct {
		name      string
		domain    string
		mustError bool
		expect    weblinkBuilder
	}{
		{
			name:      "unknown",
			domain:    "an.example.com",
			mustError: true,
		},
		{
			name:   "unstable build gitea",
			domain: "git.unstable.build",
			expect: gitea{},
		},
		{
			name:   "github.com",
			domain: "github.com",
			expect: github{},
		},
		{
			name:   "gitlab.com",
			domain: "gitlab.com",
			expect: gitlab{},
		},
		{
			name:   "self-hosted gitlab",
			domain: "gitlab.gnome.org",
			expect: gitlab{},
		},
		{
			name:   "bitbucket.org",
			domain: "bitbucket.org",
			expect: bitbucket{},
		},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			resolver := stringsContainsResolver{}
			result, err := resolver.resolveByRemoteDomain(tcase.domain)
			if tcase.mustError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tcase.expect, result)
		})
	}
}

func TestBuildWeblink(t *testing.T) {
	tsuite := []struct {
		name      string
		provider  weblinkBuilder
		parts     remoteURLParts
		mustError bool
		expect    string
	}{
		{
			name:     "unstable build gitea",
			provider: gitea{},
			parts: remoteURLParts{
				domain: "git.unstable.build",
				owner:  "unstablebuild",
				repo:   "gitproj6",
				commit: "5367f818c4e7092a224ae0f32da1b7bab8843cc0",
				file:   "recipes/guasacaca.md",
				line:   33,
			},
			expect: "https://git.unstable.build/unstablebuild/gitproj6" +
				"/src/commit/5367f818c4e7092a224ae0f32da1b7bab8843cc0/recipes/guasacaca.md#L33",
		},
		{
			name:     "github.com",
			provider: github{},
			parts: remoteURLParts{
				domain: "github.com",
				owner:  "avelino",
				repo:   "awesome-go",
				commit: "dcc5e1896883e22fa6b19039d6a67ba72fd07c6c",
				file:   "pkg/slug/generator_test.go",
				line:   12,
			},
			expect: "https://github.com/avelino/awesome-go" +
				"/blob/dcc5e1896883e22fa6b19039d6a67ba72fd07c6c/pkg/slug/generator_test.go#L12",
		},
		{
			name:     "gitlab.com",
			provider: gitlab{},
			parts: remoteURLParts{
				domain: "gitlab.com",
				owner:  "gnachman",
				repo:   "iterm2",
				commit: "d74c190a7faac3bbaab647f5f8a4551ea6c92471",
				file:   "api/library/python/iterm2/gen_profile.py",
				line:   6,
			},
			expect: "https://gitlab.com/gnachman/iterm2" +
				"/-/blob/d74c190a7faac3bbaab647f5f8a4551ea6c92471/api/library/python/iterm2/gen_profile.py#L6",
		},
		{
			name:     "self-hosted gitlab",
			provider: gitlab{},
			parts: remoteURLParts{
				domain: "gitlab.gnome.org",
				owner:  "GNOME",
				repo:   "gnome-shell",
				commit: "e997a0a7249b449828ae375d727dd23e7f5af54d",
				file:   "src/main.c",
				line:   89,
			},
			expect: "https://gitlab.gnome.org/GNOME/gnome-shell" +
				"/-/blob/e997a0a7249b449828ae375d727dd23e7f5af54d/src/main.c#L89",
		},
		{
			name:     "bitbucket.org",
			provider: bitbucket{},
			parts: remoteURLParts{
				domain: "bitbucket.org",
				owner:  "ftrack",
				repo:   "ftrack-connect",
				commit: "202c5acd169096b6cbbbdc9218dcd2f6fc4def91",
				file:   "ftrack_connect/usage.py",
				line:   12,
			},
			expect: "https://bitbucket.org/ftrack/ftrack-connect" +
				"/src/202c5acd169096b6cbbbdc9218dcd2f6fc4def91/source/ftrack_connect/usage.py#lines-12",
		},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {

			result, err := tcase.provider.buildWeblink(tcase.parts)

			if tcase.mustError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tcase.expect, result)
		})
	}
}
