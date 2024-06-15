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
package command

import (
	"context"

	"github.com/unstablebuild/blue/iterator"
)

// Completer abstracts the ability to complete command arguments.
type Completer interface {
	// Complete takes the given command and arguments and returns an iterator
	// over an expanded list of options for the last argument. It also returns
	// an expanded version of the last argument, if there is one, or an empty
	// string if the last argument could/should not be automatically expanded.
	Complete(ctx context.Context, cmd string, args ...string) (
		iterator.Iterator[string], string,
	)
}

// FuncCompleter returns a Completer that calls fn every time Complete is called.
func FuncCompleter(
	fn func(context.Context, string, ...string) (iterator.Iterator[string], string),
) Completer {
	return fnCompleter{fn: fn}
}

type fnCompleter struct {
	fn func(context.Context, string, ...string) (iterator.Iterator[string], string)
}

func (d fnCompleter) Complete(
	ctx context.Context, cmd string, args ...string,
) (iterator.Iterator[string], string) {
	return d.fn(ctx, cmd, args...)
}
