// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"context"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/docmarshal/docbson"
	"unstable.build/rune/localstorage"
	"unstable.build/rune/term/gui"
)

const (
	sizeKey     = "lastSize"
	positionKey = "lastPosition"
)

func newRuneStorage(dataDir string) storageapi.Service {
	return localstorage.New(context.Background(), dataDir, docbson.Marshaler())
}

type size struct {
	Width  int
	Height int
}

type position struct {
	X int
	Y int
}

func getLastSize(storage storageapi.Service) (width, height int, ok bool) {
	var s size
	err := storage.Get(context.Background(), sizeKey, &s)
	if err != nil && err != storageapi.ErrNotFound {
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

func getLastPosition(storage storageapi.Service) (x, y int, ok bool) {
	var s position
	err := storage.Get(context.Background(), positionKey, &s)
	if err != nil && err != storageapi.ErrNotFound {
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

func saveLastSize(storage storageapi.Service, g *gui.GUI) {
	width, height := g.Size()
	log.Debugf("saving last size of %d %d", width, height)
	err := storage.Set(context.Background(), sizeKey, size{Width: width, Height: height})
	if err != nil {
		log.Errorf("store last size: %v", err)
	}
}

func saveLastPosition(storage storageapi.Service, g *gui.GUI) {
	x, y := g.LastPosition()
	log.Debugf("saving last position of %d %d", x, y)
	err := storage.Set(context.Background(), positionKey, position{X: x, Y: y})
	if err != nil {
		log.Errorf("store last position: %v", err)
	}
}
