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

package texttest

import (
	"context"

	"github.com/unstablebuild/rune-go-sdk/api/textapi"

	"unstable.build/go-tui/ide/idecmd"
	"unstable.build/go-tui/text"
)

// dispatchWithAliases is a texttest-local helper that mirrors what
// ide/ex does in production: it wires a text.Component to an
// idecmd.Expander built from the alias map installed on the
// Component's config, then iterates the expanded command stream and
// dispatches each yielded command.
//
// Tests use it in place of comp.DispatchCommand so they continue to
// exercise alias resolution after the loop moved out of text/.
func dispatchWithAliases(
	ctx context.Context, c *text.Component, cmd textapi.Command,
) (bool, error) {
	exp := idecmd.NewExpander(c.CommandAliases(), c.DispatchEnv())
	ctx = idecmd.WithChain(ctx, cmd.Name, idecmd.NewChain())
	it, err := exp.Expand(ctx, cmd)
	if err != nil {
		return false, err
	}
	defer func() { _ = it.Close() }()
	var handled bool
	for {
		next, ok := it.Next(ctx)
		if !ok {
			break
		}
		h, derr := c.DispatchCommand(ctx, next)
		if derr != nil {
			return h, derr
		}
		handled = handled || h
	}
	return handled, it.Err()
}
