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
	"context"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/document"
	"unstable.build/go-tui/term/gui"
)

const (
	sizeKey     = "lastSize"
	positionKey = "lastPosition"
)

type size struct {
	Width  int
	Height int
}

type position struct {
	X int
	Y int
}

func getLastSize(storage document.Service) (width, height int, ok bool) {
	var s size
	err := storage.Get(context.Background(), sizeKey, &s)
	if err != nil && err != document.ErrNotFound {
		log.Errorf("get last size: %v", err)
	}
	// less than a minimum would be hard to operate
	if err == nil && s.Width > 300 && s.Height > 300 {
		width = s.Width
		height = s.Height
		ok = true
	}
	log.Debugf("got last size of %d %d", s.Width, s.Height)
	return
}

func getLastPosition(storage document.Service) (x, y int, ok bool) {
	var s position
	err := storage.Get(context.Background(), positionKey, &s)
	if err != nil && err != document.ErrNotFound {
		log.Errorf("get last position: %v", err)
	}
	// less than a minimum would be hard to operate
	if err == nil && s.X >= 0 && s.Y >= 0 {
		x = s.X
		y = s.Y
		ok = true
	}
	log.Debugf("got last position of %d %d", s.X, s.Y)
	return
}

func saveLastSize(storage document.Service, g *gui.GUI) {
	width, height := g.Size()
	log.Debugf("saving last size of %d %d", width, height)
	err := storage.Set(context.Background(), sizeKey, size{Width: width, Height: height})
	if err != nil {
		log.Errorf("store last size: %v", err)
	}
}

func saveLastPosition(storage document.Service, g *gui.GUI) {
	x, y := g.LastPosition()
	log.Debugf("saving last position of %d %d", x, y)
	err := storage.Set(context.Background(), positionKey, position{X: x, Y: y})
	if err != nil {
		log.Errorf("store last position: %v", err)
	}
}
