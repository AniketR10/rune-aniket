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

package config

import "github.com/unstablebuild/tcell/v3"

// ErrConfig returns a Config that returns err to all methods of Config.
func ErrConfig(err error) Config {
	return errConfig{err: err}
}

type errConfig struct {
	err error
}

func (e errConfig) GetInt(string) (int, error) {
	return 0, e.err
}

func (e errConfig) GetFloat(string) (float64, error) {
	return 0, e.err
}

func (e errConfig) GetString(string) (string, error) {
	return "", e.err
}

func (e errConfig) GetBool(string) (bool, error) {
	return false, e.err
}

func (e errConfig) GetConfig(string) (Config, error) {
	return nil, e.err
}

func (e errConfig) GetMap(string) (map[string]interface{}, error) {
	return nil, e.err
}

func (e errConfig) GetAttribute(string) (tcell.AttrMask, error) {
	return 0, e.err
}

func (e errConfig) GetColor(string) (tcell.Color, error) {
	return 0, e.err
}

func (e errConfig) GetRune(string) (rune, error) {
	return 0, e.err
}

func (e errConfig) GetSlice(string) ([]interface{}, error) {
	return nil, e.err
}

func (e errConfig) Iterate(fn func(k string, value interface{})) {
}
