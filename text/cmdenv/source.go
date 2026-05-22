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

// Package cmdenv resolves the named variables that Rune expands inside
// command alias bodies and dispatched argv at dispatch time.
package cmdenv

import (
	"os"
)

// Source resolves variable names to values for command-time expansion.
// Returning ok=false indicates the variable is unknown to this source;
// callers chain to other lookups (typically os.Getenv). A nil Source
// is equivalent to one that returns ok=false for every name.
type Source func(name string) (value string, ok bool)

// Lookup returns the function expected by mvdan.cc/sh/v3/shell
// (Fields / Expand) that resolves variables through src first and
// falls back to os.Getenv. A nil src behaves like os.Getenv alone.
func Lookup(src Source) func(string) string {
	if src == nil {
		return os.Getenv
	}
	return func(name string) string {
		if v, ok := src(name); ok {
			return v
		}
		return os.Getenv(name)
	}
}
