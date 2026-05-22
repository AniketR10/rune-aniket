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

package idecmd

import "fmt"

// checkReferences returns an error when token references a
// positional argument $N (N=1..9) with N greater than argCount.
func checkReferences(token string, argCount int) error {
	for _, name := range scanDollarRefs(token) {
		if len(name) != 1 || name[0] < '1' || name[0] > '9' {
			continue
		}
		pos := int(name[0]-'0') - 1
		if pos >= argCount {
			return fmt.Errorf(
				"alias expects an argument at position %d ($%s)",
				pos+1, name)
		}
	}
	return nil
}

// recordReferences inserts each 0-based positional index referenced
// by token (and within argCount bounds) into out.
func recordReferences(token string, argCount int, out map[int]struct{}) {
	for _, name := range scanDollarRefs(token) {
		if len(name) != 1 || name[0] < '1' || name[0] > '9' {
			continue
		}
		pos := int(name[0]-'0') - 1
		if pos < argCount {
			out[pos] = struct{}{}
		}
	}
}

// scanDollarRefs returns names referenced via $NAME / ${NAME} / $N
// inside s. Escaped \$ is skipped, and $$ is treated as a literal $.
// Duplicates are not deduplicated.
func scanDollarRefs(s string) []string {
	var names []string
	i := 0
	n := len(s)
	for i < n {
		c := s[i]
		if c == '\\' && i+1 < n {
			i += 2
			continue
		}
		if c != '$' {
			i++
			continue
		}
		if i+1 < n && s[i+1] == '$' {
			i += 2
			continue
		}
		i++
		if i >= n {
			break
		}
		if s[i] == '{' {
			i++
			start := i
			for i < n && s[i] != '}' {
				i++
			}
			if i > start {
				names = append(names, s[start:i])
			}
			if i < n {
				i++
			}
			continue
		}
		start := i
		if !isDollarHead(s[i]) {
			continue
		}
		i++
		if s[start] >= '0' && s[start] <= '9' {
			names = append(names, s[start:i])
			continue
		}
		for i < n && isDollarTail(s[i]) {
			i++
		}
		names = append(names, s[start:i])
	}
	return names
}

func isDollarHead(b byte) bool {
	return (b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		b == '_' ||
		(b >= '0' && b <= '9')
}

func isDollarTail(b byte) bool {
	return (b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9') ||
		b == '_'
}
