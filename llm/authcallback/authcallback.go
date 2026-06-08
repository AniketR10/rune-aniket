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

// Package authcallback renders the branded HTML page shown in the user's
// browser after an OAuth login callback completes.
package authcallback

import (
	_ "embed"
	"html/template"
	"io"
	"net/http"
	"strings"
)

//go:embed callback_page.html
var callbackPageHTML string

var callbackPageTmpl = template.Must(
	template.New("callback_page.html").Parse(callbackPageHTML))

// Page describes the branded copy shown on the OAuth success page.
type Page struct {
	Label   string
	Title   string
	Heading string
	Message string
}

// Render returns the fully rendered HTML document for the success page.
func Render(p Page) string {
	var buf strings.Builder
	if err := callbackPageTmpl.Execute(&buf, p); err != nil {
		return ""
	}
	return buf.String()
}

// WriteSuccess writes the branded success page to w as an HTML response.
func WriteSuccess(w http.ResponseWriter, p Page) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, Render(p))
}
