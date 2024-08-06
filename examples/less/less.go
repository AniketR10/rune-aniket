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
	"flag"
	"io"
	
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"runtime/pprof"

	"unstable.build/go-tui"
	"unstable.build/go-tui/handler"
)

var (
	less *handler.Less
)

var wrap = flag.Bool("w", false, "wrap text")

func startCPUProfile() func() {
	f, err := os.CreateTemp("", "less_cpuprofile")
	if err != nil {
		log.Fatal("could not create CPU profile: ", err)
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		log.Fatal("could not start CPU profile: ", err)
	}
	return func() {
		pprof.StopCPUProfile()
		f.Close()
	}
}

func writeMemProfile() {
	f, err := os.CreateTemp("", "less_memprofile")
	if err != nil {
		log.Fatal("could not create memory profile: ", err)
	}
	defer f.Close()
	runtime.GC() // get up-to-date statistics
	if err := pprof.WriteHeapProfile(f); err != nil {
		log.Fatal("could not write memory profile: ", err)
	}
}

func handleLessEvent(ev handler.LessEvent) {
	switch ev.Type {
	case handler.EOF:
		less.SetMessage("EOF")
	case handler.Search:
		less.SetMessage("search pattern: %s..", ev.Data)
	}
}

func main() {
	go func() {
		// for net/pprof
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	var input io.Reader
	var err error

	if len(os.Args) > 1 {
		filename := os.Args[1]
		if input, err = os.Open(filename); err != nil {
			log.Fatal(err)
		}
		if err = flag.CommandLine.Parse(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
	} else {
		input = os.Stdin
		flag.Parse()

	}

	config := handler.DefaultLessConfig()
	config.Wrap = *wrap
	config.Handler = handleLessEvent

	// profile initialization
	stopCPUProfile := startCPUProfile()

	less = handler.NewLess(config)
	_, err = less.Buffer().ReadFrom(input)
	if err != nil {
		log.Fatal(err)
	}

	stopCPUProfile()

	if err = tui.Init(); err != nil {
		log.Fatal(err)
	}

	writeMemProfile()

	defer tui.Close()

	if err = tui.Run(less); err != nil {
		log.Fatal(err)
	}

}
