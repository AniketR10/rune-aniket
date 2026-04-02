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

package main

import (
	"fmt"

	"github.com/ernestrc/go-multierror"
	"github.com/unstablebuild/blue/document/doclog"
	"github.com/unstablebuild/blue/document/docmarshal/doctoml"
	"github.com/unstablebuild/blue/document/docrpc"
	"github.com/unstablebuild/blue/document/firestore"
	"google.golang.org/grpc"
)

func registerDocumentService(
	projectID, credsFile string,
	gsrv *grpc.Server, firestoreCollection string,
) error {
	db, err := firestore.New(projectID, firestoreCollection, credsFile)
	if err != nil {
		return fmt.Errorf("firestore new: %v", err)
	}
	dbWithLogs := doclog.WithLogging(db, fmt.Sprintf("/Firestore/%s", firestoreCollection))
	srv := new(docrpc.Server)
	// toml is the only binary-compatible encoding.Marshaler with firestore's marshaling scheme.
	// It doesn't fiddle with field upper/lower case so it allows both access to the data
	// via firestore's driver or through an RPC api.
	srv.Init(dbWithLogs, doctoml.Marshaler())
	docrpc.RegisterCollectionDocumentService(gsrv, srv, firestoreCollection)
	return nil
}

func registerGRPCApi(
	projectID, credsFile string,
	srv *grpc.Server,
	issuesCollection string, releaseCollections []string,
) (ret error) {
	if err := registerDocumentService(projectID, credsFile, srv, issuesCollection); err != nil {
		ret = multierror.Append(ret, err)
	}
	for _, rc := range releaseCollections {
		if err := registerDocumentService(projectID, credsFile, srv, rc); err != nil {
			ret = multierror.Append(ret, err)
		}
	}
	return
}
