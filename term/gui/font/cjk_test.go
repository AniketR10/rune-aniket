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
