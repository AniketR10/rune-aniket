// Copyright (C) 2017-2026 Unstable Build, LLC
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

package font

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCJKFaceIndexForLocale(t *testing.T) {
	cases := []struct {
		locale string
		want   int
	}{
		{"ja_JP.UTF-8", cjkFaceMonoJP},
		{"ja", cjkFaceMonoJP},
		{"ko_KR", cjkFaceMonoKR},
		{"ko_KR.UTF-8", cjkFaceMonoKR},
		{"zh_CN", cjkFaceMonoSC},
		{"zh_CN.UTF-8", cjkFaceMonoSC},
		{"zh_TW", cjkFaceMonoTC},
		{"zh_TW.UTF-8", cjkFaceMonoTC},
		{"zh_Hant", cjkFaceMonoTC},
		{"zh_HK", cjkFaceMonoHK},
		{"zh_MO", cjkFaceMonoHK},
		{"", cjkFaceMonoSC},
		{"en_US.UTF-8", cjkFaceMonoSC},
		{"C", cjkFaceMonoSC},
	}
	for _, tc := range cases {
		t.Run(tc.locale, func(t *testing.T) {
			assert.Equal(t, tc.want, cjkFaceIndexForLocale(tc.locale))
		})
	}
}

func TestCJKFaceIndexEnvPrecedence(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want int
	}{
		{"LC_ALL wins over LC_CTYPE and LANG", map[string]string{
			"LC_ALL": "ja_JP.UTF-8", "LC_CTYPE": "ko_KR.UTF-8", "LANG": "zh_TW.UTF-8",
		}, cjkFaceMonoJP},
		{"LC_CTYPE wins over LANG", map[string]string{
			"LC_CTYPE": "ko_KR.UTF-8", "LANG": "zh_TW.UTF-8",
		}, cjkFaceMonoKR},
		{"LANG is the last resort", map[string]string{
			"LANG": "zh_TW.UTF-8",
		}, cjkFaceMonoTC},
		{"no locale set falls back to SC", nil, cjkFaceMonoSC},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, key := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
				t.Setenv(key, tc.env[key])
			}
			assert.Equal(t, tc.want, cjkFaceIndex())
		})
	}
}
