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

package rpc

import (
	"context"
	"runtime"

	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/term"
	termpb "unstable.build/go-tui/term/rpc"
)

type clientWriter struct {
	uri    workspaceapi.URI
	client *Client
}

func (w clientWriter) Edit(
	ctx context.Context, start, end term.Coordinates, str string,
) (from, to term.Coordinates, old string, err error) {
	var protoStart, protoEnd termpb.Coordinates
	protoStart.FromModel(start)
	protoEnd.FromModel(end)
	req := EditCellRequest{
		ResourceName: NewURI(w.uri),
		Start:        &protoStart,
		End:          &protoEnd,
		Str:          str,
	}
	res, err := w.client.ed.EditCell(ctx, &req)
	runtime.KeepAlive(w.client)
	if err != nil {
		return from, to, "", err
	}

	from = res.GetFrom().ToModel()
	to = res.GetTo().ToModel()
	old = res.GetOld()
	return
}
