// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2023-2024 Unstable Build, All Rights Reserved.
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

package auth

import (
	"context"
	"errors"

	"golang.org/x/oauth2"
)

var (
	// ErrUnavailable can be used by clients and implementors of TokenSource
	// to signal that an expected error in the flow.
	ErrUnavailable = errors.New("sourcer is currently not available")
)

// TokenSourcer abstracts an oauth2 token client acquisition flow.
type TokenSourcer interface {
	// TokenSource gets a new TokenSource from the gien token. If token is
	// nil, this indicates that the oauth2 flow needs to be re-started.
	TokenSource(context.Context, *oauth2.Token) (oauth2.TokenSource, error)
}

// FuncTokenSourcer wraps fn to satisfy TokenSourcer.
func FuncTokenSourcer(
	fn func(context.Context, *oauth2.Token) (oauth2.TokenSource, error),
) TokenSourcer {
	return funcTokenSourcer(fn)
}

type funcTokenSourcer func(context.Context, *oauth2.Token) (oauth2.TokenSource, error)

func (f funcTokenSourcer) TokenSource(
	ctx context.Context, t *oauth2.Token,
) (oauth2.TokenSource, error) {
	return f(ctx, t)
}
