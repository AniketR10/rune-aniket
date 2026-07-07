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


package apiclient

import (
	"strings"
	"testing"
)

func TestCallbackPageHTML(t *testing.T) {
	if !strings.HasPrefix(callbackPageHTML, "<!doctype html") {
		t.Fatalf("callbackPageHTML must start with <!doctype html, got %q",
			callbackPageHTML[:min(40, len(callbackPageHTML))])
	}
	for _, needle := range []string{
		"You're in",
		"return to Rune",
	} {
		if !strings.Contains(callbackPageHTML, needle) {
			t.Errorf("callbackPageHTML missing %q", needle)
		}
	}
	for _, absent := range []string{
		"{{.CheckoutURL}}",
		"/checkout",
		"window.location",
		"http-equiv=\"refresh\"",
	} {
		if strings.Contains(callbackPageHTML, absent) {
			t.Errorf("callbackPageHTML must not redirect, found %q", absent)
		}
	}
}
