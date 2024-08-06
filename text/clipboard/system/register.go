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

package system

import (
	"errors"
	"sync/atomic"

	sysclip "github.com/atotto/clipboard"
	"unstable.build/go-tui/text/clipboard"
)

type register struct {
	data atomic.Value
}

type registerData struct {
	metadata interface{}
	data     string
}

// NewRegister allocates initializes a new system clipboard.
func NewRegister() (clipboard.Register, error) {
	if sysclip.Unsupported {
		return nil, errors.New("system clipboard unsupported")
	}
	ret := new(register)
	ret.data.Store(registerData{})
	return ret, nil
}

// Paste satisfies clipboard.Register.
func (r *register) Paste(id string) (ret clipboard.Data, err error) {
	text, err := sysclip.ReadAll()
	if err != nil {
		return ret, err
	}
	// only return metadata if it matches last copy
	ret.Text = text
	data := r.data.Load().(registerData)
	if data.data != text {
		return
	}
	ret.Metadata = data.metadata
	return
}

// Copy satisfies clipboard.Register.
func (r *register) Copy(id string, data clipboard.Data) error {
	r.data.Store(registerData{data: data.Text, metadata: data.Metadata})
	return sysclip.WriteAll(data.Text)
}
