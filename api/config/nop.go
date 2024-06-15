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

type nopConfig struct{}

// NopConfig returns a Config that always returns ErrNotFound.
func NopConfig() Config {
	return nopConfig{}
}

func (n nopConfig) GetInt(string) (int, error) {
	return 0, ErrNotFound
}

func (n nopConfig) GetFloat(string) (float64, error) {
	return 0, ErrNotFound
}

func (n nopConfig) GetString(string) (string, error) {
	return "", ErrNotFound
}

func (n nopConfig) GetBool(string) (bool, error) {
	return false, ErrNotFound
}

func (n nopConfig) GetConfig(string) (Config, error) {
	return nil, ErrNotFound
}

func (n nopConfig) GetMap(string) (map[string]interface{}, error) {
	return nil, ErrNotFound
}

func (n nopConfig) GetAttribute(string) (tcell.AttrMask, error) {
	return 0, ErrNotFound
}

func (n nopConfig) GetColor(string) (tcell.Color, error) {
	return 0, ErrNotFound
}

func (n nopConfig) GetRune(string) (rune, error) {
	return 0, ErrNotFound
}

func (n nopConfig) GetSlice(string) ([]interface{}, error) {
	return nil, ErrNotFound
}

func (n nopConfig) Iterate(fn func(k string, value interface{})) {
}
