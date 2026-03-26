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

package user

import (
	"context"
	"fmt"
)

// Store abstracts user persistence to durable storage.
type Store interface {
	Create(context.Context, ID, User) error
	// Update updates the given user with id, with the given document updates. This method
	// should panic if updates is nil or of 0 length.
	//
	// The updates field names must conform to
	// https://auth0.com/docs/api/management/v2/users/patch-users-by-id
	Update(context.Context, ID, map[string]any) error
	Get(context.Context, ID) (User, error)
	Health(context.Context) error
}

var _ error = (*ErrAlreadyExists)(nil)

// ErrAlreadyExists is returned by Store.Create if a user
// already exists.
type ErrAlreadyExists struct {
	Account string
	Err     error
}

func (e *ErrAlreadyExists) Error() string {
	return fmt.Sprintf("user already exists: %v", e.Err)
}

func (e *ErrAlreadyExists) Unwrap() error {
	return e.Err
}
