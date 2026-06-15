// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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

package text

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/clipboard"
	"github.com/unstablebuild/rune-go-sdk/clipboard/sysclip"
)

// NewSystemClipboard returns a clipboard.Register backed by the OS
// clipboard.
func NewSystemClipboard() clipboard.Register {
	return newSystemClipboard(sysclip.NewRegister())
}

type systemClipboard struct {
	sys     clipboard.Register
	mem     clipboard.Register
	openErr error
}

func newSystemClipboard(sys clipboard.Register, err error) clipboard.Register {
	if err != nil {
		msg := err.Error()
		if runtime.GOOS == "linux" && strings.Contains(msg, "unsupported") {
			msg += "; install one of the following clipboard utilities: " +
				"xclip, xsel, or wl-clipboard (Wayland)"
		}
		err = fmt.Errorf("system clipboard: %s", msg)
	}
	return &systemClipboard{sys: sys, mem: clipboard.NewInMemory(), openErr: err}
}

func (c *systemClipboard) Copy(registerID string, data clipboard.Data) error {
	_ = c.mem.Copy(registerID, data)
	if registerID != clipboard.DefaultRegisterID {
		return nil
	}
	if c.openErr != nil {
		return c.openErr
	}
	if err := c.sys.Copy(registerID, data); err != nil {
		return fmt.Errorf("system clipboard: %s", err.Error())
	}
	return nil
}

func (c *systemClipboard) Paste(registerID string) (clipboard.Data, error) {
	if c.openErr != nil {
		data, _ := c.mem.Paste(registerID)
		return data, c.openErr
	}
	data, err := c.sys.Paste(registerID)
	if err != nil {
		memData, _ := c.mem.Paste(registerID)
		return memData, fmt.Errorf("system clipboard: %s", err.Error())
	}
	return data, nil
}
