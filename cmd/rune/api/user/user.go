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
	"encoding/base64"
	"fmt"
	"time"

	"unstable.build/go-tui/cmd/rune/auth"
)

// ID represents a user identifier.
type ID string

// IDFromClaimsSubject extracts a user ID from authentication claims.
func IDFromClaimsSubject(subject string) ID {
	return ID(subject)
}

// IDFromURIPath decodes the given user ID from a URI resource path.
func IDFromURIPath(userIDPath string) (ret ID, err error) {
	var decoded []byte
	decoded, err = base64.StdEncoding.DecodeString(userIDPath)
	if err != nil {
		err = fmt.Errorf("base64 decode: %w", err)
	} else {
		ret = ID(decoded)
	}
	return
}

// User holds the user data stored in Store.
type User struct {
	PictureURL string
	FirstName  string
	LastName   string
	Issuer     string
	Verified   bool
	CreatedAt  time.Time

	// the following need to match auth.RPCUser,
	// but cannot embed to not screw up with marshalers.
	ID      ID
	Email   string
	Role    auth.Role
	Account string
}

// LoggingID returns an ID suitable for logging.
func (u User) LoggingID() string {
	return string(u.ID)
}
