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

package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"golang.org/x/oauth2"
	"unstable.build/go-tui/debug"
)

const telemetryPath = "/telemetry"

// flushTimeout bounds the final usage post performed on Close so that
// shutting down Rune while offline is not delayed by a hanging request.
const flushTimeout = 300 * time.Millisecond

// TelemetryEvents returns the events that telemetry's text.EventHandler needs
// to collects user statistics.
func TelemetryEvents() []textapi.EventType {
	return []textapi.EventType{
		textapi.EventTypeOpen,
		textapi.EventTypeClose,
		textapi.EventTypeFlush,
		textapi.EventTypeEdit,
	}
}

type telemetry struct {
	auth      oauth2.TokenSource
	url       string
	period    time.Duration
	quitCtx   context.Context
	cancelCtx func()

	installID  string
	tampered   bool
	installErr string
	sessionID  string
	sysinfo    sysinfo
	version    string
	editorMode string

	opened         atomic.Int32
	closed         atomic.Int32
	flushed        atomic.Int32
	edited         atomic.Int32
	watchedChanges atomic.Int32
	commands       atomic.Int32
}

func newTelemetry(
	auth oauth2.TokenSource,
	url *url.URL,
	period time.Duration,
	version string,
	editorMode string,
	store storageapi.Service,
	installBackupDir string,
) *telemetry {
	if period <= 0 {
		panic(fmt.Sprintf("apiclient: TelemetryPeriod must be positive, got %v", period))
	}
	if installBackupDir == "" {
		panic("apiclient: InstallBackupDir is required")
	}
	ret := new(telemetry)
	ret.auth = auth

	ret.url = url.JoinPath(telemetryPath).String()
	ret.sessionID = uuid.New().String()
	ret.sysinfo, _ = uname()
	ret.version = version
	ret.editorMode = editorMode
	ret.period = period

	ret.quitCtx, ret.cancelCtx = context.WithCancel(context.Background())

	ret.installID, ret.tampered, ret.installErr = getInstallID(ret.quitCtx, store, installBackupDir)

	return ret
}

func (t *telemetry) start() {
	go debug.CapturePanicReport(func() {
		t.periodicPostData(t.period)
	})
}

func (t *telemetry) periodicPostData(period time.Duration) {
	log.Tracef("Starting period posting of telemetry data every %v", period)

	timer := time.NewTimer(period)
	defer timer.Stop()

	buf := new(bytes.Buffer)

	// start with a declaration of the client's system event
	data := t.getSystemData()
	err := t.postData(buf, period, data)
	if err != nil {
		log.Tracef("Could not post telemetry system data: %v", err)
	}

	for {
		select {
		case <-timer.C:
			timer.Reset(period)
		case <-t.quitCtx.Done():
			return
		}

		data := t.getUsage()
		err := t.postData(buf, period, data)
		if err != nil {
			log.Tracef("Could not post telemetry usage data: %v", err)
			continue
		}
		t.resetUsage(data)
	}
}

func (t *telemetry) getUsage() telemetryUsagePayload {
	var data telemetryUsagePayload
	data.Opened = int(t.opened.Load())
	data.Closed = int(t.closed.Load())
	data.Flushed = int(t.flushed.Load())
	data.Edited = int(t.edited.Load())
	data.WatchedChanges = int(t.watchedChanges.Load())
	data.Commands = int(t.commands.Load())
	data.SID = t.sessionID
	data.Type = "ClientUsage"
	data.EditorMode = t.editorMode
	return data
}

func (t *telemetry) getSystemData() telemetrySystemPayload {
	var data telemetrySystemPayload
	data.InstallID = t.installID
	data.Tampered = t.tampered
	data.InstallIDErr = t.installErr
	data.SID = t.sessionID
	data.Type = "ClientSystem"
	data.SystemArquitecture = t.sysinfo.Machine
	data.SystemOS = t.sysinfo.OS
	data.SystemName = t.sysinfo.Node
	data.SystemRelease = t.sysinfo.Release
	data.SystemVersion = t.sysinfo.Version
	data.Version = t.version
	data.EditorMode = t.editorMode
	return data
}

func (t *telemetry) resetUsage(data telemetryUsagePayload) {
	// reset exactly the counts that were posted
	t.opened.Add(-int32(data.Opened))
	t.closed.Add(-int32(data.Closed))
	t.flushed.Add(-int32(data.Flushed))
	t.edited.Add(-int32(data.Edited))
	t.watchedChanges.Add(-int32(data.WatchedChanges))
	t.commands.Add(-int32(data.Commands))
}

func (t *telemetry) postData(buf *bytes.Buffer, period time.Duration, data any) error {
	ctx, cancel := context.WithTimeout(t.quitCtx, period)
	defer cancel()
	return t.postDataCtx(ctx, buf, data)
}

func (t *telemetry) postDataCtx(ctx context.Context, buf *bytes.Buffer, data any) error {
	buf.Reset()
	err := json.NewEncoder(buf).Encode(data)
	if err != nil {
		return fmt.Errorf("json encode: %v", err)
	}

	r, err := http.NewRequestWithContext(ctx, "POST", t.url, buf)
	if err != nil {
		return fmt.Errorf("new request: %v", err)
	}

	token, err := t.auth.Token()
	if err == nil && token.Valid() {
		r.Header["Authorization"] = []string{fmt.Sprintf("%s %s", token.TokenType, token.AccessToken)}
	}

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return fmt.Errorf("post request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("response status code non-200")
	}
	return nil
}

func (t *telemetry) Handle(ctx context.Context, ev textapi.Event) bool {
	switch ev.Type {
	case textapi.EventTypeOpen:
		t.opened.Add(1)
	case textapi.EventTypeClose:
		t.closed.Add(1)
	case textapi.EventTypeFlush:
		t.flushed.Add(1)
	case textapi.EventTypeEdit:
		t.edited.Add(1)
	}
	return false
}

func (t *telemetry) recordWatchedFilesChange(n int) {
	t.watchedChanges.Add(int32(n))
}

func (t *telemetry) recordCommand() {
	t.commands.Add(1)
}

func (t *telemetry) Close() error {
	t.flushFinalUsage()
	t.cancelCtx()
	return nil
}

func (t *telemetry) flushFinalUsage() {
	data := t.getUsage()
	ctx, cancel := context.WithTimeout(context.Background(), flushTimeout)
	defer cancel()
	_ = t.postDataCtx(ctx, new(bytes.Buffer), data)
}

type telemetryUsagePayload struct {
	Type           string
	SID            string
	EditorMode     string
	Opened         int
	Closed         int
	Edited         int
	Flushed        int
	WatchedChanges int
	Commands       int
}

type telemetrySystemPayload struct {
	Type               string
	InstallID          string
	Tampered           bool
	InstallIDErr       string
	SID                string
	Version            string
	EditorMode         string
	SystemArquitecture string
	SystemOS           string
	SystemName         string
	SystemRelease      string
	SystemVersion      string
}
