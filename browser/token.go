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

package browser

import (
	"unstable.build/go-tui"
	"unstable.build/go-tui/term"
)

const errMsg = "this Handler is a token handler that cannot be used directly"

// Token is a token handler used to indicate which of the remote handlers
// to set as content to a remote server. It satisfies tui.Handler so that clients
// can take the result of an browser.Open type of requests and pass it to Split* or SetContent
// type of responses.
type Token struct {
	ID string
}

// Handle panics if called. This tui.Handler implementation is symbolic.
func (h Token) Handle(term.Event) (exit, handled bool) {
	panic(errMsg)
}

// Cursor panics if called. This tui.Handler implementation is symbolic.
func (h Token) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	panic(errMsg)
}

// Man panics if called. This tui.Handler implementation is symbolic.
func (h Token) Man() tui.Manual {
	panic(errMsg)
}

// Resize panics if called. This tui.Handler implementation is symbolic.
func (h Token) Resize(width, height int) {
	panic(errMsg)
}

// Draw panics if called. This tui.Handler implementation is symbolic.
func (h Token) Draw(w term.Writer) {
	panic(errMsg)
}

// Close does nothing if called.
func (h Token) Close() error {
	return nil
}
