// Copyright (C) 2017-2026 The Rune Authors
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

	// ErrNotAuthenticated indicates that no valid credentials are available
	// and the caller must explicitly initiate the login flow.
	// Background callers should treat this as a no-op; interactive callers
	// should surface a notification asking the user to run the `login` command.
	ErrNotAuthenticated = errors.New("not authenticated; run the `login` command")
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
