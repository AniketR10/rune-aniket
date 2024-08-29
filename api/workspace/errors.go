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

package api

import (
	"errors"
	"os"
)

var (
	// ErrFileIsNotRegular is returned when file type was not expected to be a directory.
	ErrFileIsNotRegular = errors.New("file type is not regular")

	// ErrFileIsNotWritable is returned when file is opened in read-only.
	ErrFileIsNotWritable = errors.New("file is not writable")

	// ErrFileAlreadyOpen is returned when a file is not expected to be opened already.
	ErrFileAlreadyOpen = errors.New("file open by another process or previous process was closed abruptly")

	// ErrStaleData is returned when a file was modified by some other application.
	ErrStaleData = errors.New("file was modified by another process since reading it")
)

// Error is used to abstract os.Is(.*) functions
type Error struct {
	Err          error
	IsPermission bool
	IsExist      bool
	IsNotExist   bool
}

// String returns the string representation of the underlying error.
func (e *Error) String() string {
	if e == nil {
		return "<nil>"
	}

	return e.ToError().Error()
}

// ToError returns a os error or the underlying
// error.
func (e Error) ToError() error {
	if e.IsPermission {
		return os.ErrPermission
	}
	if e.IsNotExist {
		return os.ErrNotExist
	}
	if e.IsExist {
		return os.ErrExist
	}
	if e.Err != nil {
		return e.Err
	}
	panic("workspaceapi.Error with nil Error")
}

// NopError returns an Error that simply wraps err.
func NopError(err error) *Error {
	return &Error{Err: err}
}
