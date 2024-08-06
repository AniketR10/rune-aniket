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

package test

import (
	browserapi "unstable.build/go-tui/api/browser"
	"unstable.build/go-tui/browser"
)

// WindowToAPIWindow is a convenience method to work
// with browserapi.Window and browser.Window in the same package.
type WindowToAPIWindow struct {
	Win browser.Window
}

func (a WindowToAPIWindow) SetContent(h browserapi.Handler) error {
	return a.Win.SetContent(h)
}

func (a WindowToAPIWindow) Focus() (bool, error) {
	return a.Win.Focus()
}

func (a WindowToAPIWindow) Close() error {
	return a.Win.Close()
}

func (a WindowToAPIWindow) Content() (browserapi.Handler, error) {
	return a.Win.Content()
}

func (a WindowToAPIWindow) ID() uint64 {
	return a.Win.ID()
}

// WindowFromAPIWindow is a convenience method to work
// with browserapi.Window and browser.Window in the same package.
type WindowFromAPIWindow struct {
	Win browserapi.Window
}

func (a WindowFromAPIWindow) SetContent(h browserapi.Handler) error {
	return a.Win.SetContent(h)
}

func (a WindowFromAPIWindow) Content() (browserapi.Handler, error) {
	h, err := a.Win.(interface {
		Content() (browserapi.Browser, error)
	}).Content()
	return h.(browserapi.Handler), err
}

func (a WindowFromAPIWindow) ID() uint64 {
	return a.Win.(interface{ ID() uint64 }).ID()
}

func (a WindowFromAPIWindow) Focus() (bool, error) {
	return a.Win.Focus()
}

func (a WindowFromAPIWindow) Close() error {
	return a.Win.Close()
}

func (w WindowFromAPIWindow) Closed() bool {
	return w.Win.(interface{ Closed() bool }).Closed()
}

func (w WindowFromAPIWindow) IsFloating() bool {
	return w.Win.(interface{ IsFloating() bool }).IsFloating()
}
