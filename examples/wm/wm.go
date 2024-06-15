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
package main

import (
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"

	"unstable.build/go-tui"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/term"
)

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	if len(os.Args) < 2 {
		fmt.Printf("usage: %s <filename>\n", os.Args[0])
		return
	}

	filename := os.Args[1]
	input, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}

	if err := tui.Init(); err != nil {
		log.Fatal(err)
	}

	defer tui.Close()

	var wm *handler.WindowManager
	var less [4]handler.Less

	for i := range less {
		less[i].Init(handler.DefaultLessConfig())
		less[i].Buffer().ReadFrom(input)
	}

	wm = handler.NewWindowManager(&less[0], handler.DefaultWindowManagerConfig())

	wm.SplitHorizontal(wm.Focus(), &less[1])
	wm.FocusUp()
	wm.FocusLeft()
	wm.SplitVertical(wm.Focus(), &less[2])
	wm.SplitVertical(wm.Focus(), &less[3])

	term.SetInputMode(term.InputAlt | term.InputMouse)

	if err := tui.Run(wm); err != nil {
		log.Fatal(err)
	}
}
