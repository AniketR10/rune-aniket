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
	"os"
	"strings"
)

// Face indices into builtinfont.CJKTTC. The collection holds Noto Sans
// CJK JP,KR,SC,TC,HK followed by Noto Sans Mono CJK in the same order.
// Only the Mono faces are usable in a terminal: their ASCII advance is
// exactly one cell, whereas the proportional faces overflow it.
const (
	cjkFaceMonoJP = 5
	cjkFaceMonoKR = 6
	cjkFaceMonoSC = 7
	cjkFaceMonoTC = 8
	cjkFaceMonoHK = 9
)

// cjkFaceIndex picks the Han-unification variant matching the user's
// locale, following POSIX precedence LC_ALL > LC_CTYPE > LANG.
func cjkFaceIndex() int {
	return cjkFaceIndexForLocale(currentLocale())
}

func currentLocale() string {
	for _, key := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	return ""
}

func cjkFaceIndexForLocale(locale string) int {
	// strip the ".UTF-8" codeset and any "@modifier" suffix
	lang := locale
	if i := strings.IndexAny(lang, ".@"); i >= 0 {
		lang = lang[:i]
	}
	lang = strings.ReplaceAll(strings.ToLower(lang), "-", "_")

	switch {
	case strings.HasPrefix(lang, "ja"):
		return cjkFaceMonoJP
	case strings.HasPrefix(lang, "ko"):
		return cjkFaceMonoKR
	case lang == "zh_hk", lang == "zh_mo",
		strings.HasPrefix(lang, "zh_hant_hk"), strings.HasPrefix(lang, "zh_hant_mo"):
		return cjkFaceMonoHK
	case lang == "zh_tw", strings.HasPrefix(lang, "zh_hant"):
		return cjkFaceMonoTC
	default:
		return cjkFaceMonoSC
	}
}
