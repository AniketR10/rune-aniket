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
	"fmt"
	"log/slog"
	"net/url"
	"os"

	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"unstable.build/rune/debug"
	htmlhandler "unstable.build/rune/handler/html"
)

func main() {
	debug.StartPProfOnSignal()
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: runefox <url>")
		os.Exit(1)
	}

	u, err := url.Parse(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "usage: runefox <url>")
		os.Exit(1)
	}

	interrupter := term.FuncInterrupter(func(context.Context) error {
		term.ScheduleNextTick(func() {})
		return nil
	})

	h := htmlhandler.New(interrupter, u,
		htmlhandler.WithNavigationBar(htmlhandler.BarTop))
	defer func() { _ = h.Close() }()

	if err := tui.Run(h, tui.WithInputMode(term.InputEsc|term.InputMouse)); err != nil {
		slog.Error("run error", "error", err)
	}
}
