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

package ideplan

import (
	"context"
	"fmt"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"unstable.build/go-tui/debug"
)

// DefaultMonitorInterval is the cadence at which the running-IDE
// monitor re-evaluates gating against the JWT.
const DefaultMonitorInterval = 24 * time.Hour

// Locker abstracts the ability to lock the IDE.
type Locker interface {
	SetLocked(locked bool)
}

// MonitorConfig collects the dependencies of Monitor. All fields
// except Interval and Now are required; NewMonitor panics on missing
// dependencies.
type MonitorConfig struct {
	Source        Source
	Notifications browserapi.Notifications
	Locker        Locker
	Interval      time.Duration
	Now           func() time.Time
}

// Monitor periodically refreshes the gating decision and reacts:
//
//   - StatusActive: clear the pinned warning, ensure the lockdown
//     wrapper is unlocked.
//   - StatusGracePeriod: emit one notification per day, pinned via
//     UpdateNotificationProgress so the user cannot dismiss it.
//   - StatusExpired: signal the lockdown wrapper.
type Monitor struct {
	cfg MonitorConfig

	mu              sync.Mutex
	lastNotifiedDay time.Time
	warningID       string
	stopCh          chan struct{}
	started         bool
}

// NewMonitor constructs a Monitor with the given configuration,
// applying defaults for unset Interval and Now fields. Panics when a
// required dependency is missing.
func NewMonitor(cfg MonitorConfig) *Monitor {
	if cfg.Source == nil {
		panic("ideplan: MonitorConfig.Source is required")
	}
	if cfg.Notifications == nil {
		panic("ideplan: MonitorConfig.Notifications is required")
	}
	if cfg.Locker == nil {
		panic("ideplan: MonitorConfig.Locker is required")
	}
	if cfg.Interval <= 0 {
		cfg.Interval = DefaultMonitorInterval
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Monitor{cfg: cfg, stopCh: make(chan struct{})}
}

// Start launches the background ticker. Safe to call once.
func (m *Monitor) Start(ctx context.Context) {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return
	}
	m.started = true
	m.mu.Unlock()
	go debug.CapturePanicReport(func() { m.run(ctx) })
}

// Stop halts the ticker. Safe to call multiple times.
func (m *Monitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	select {
	case <-m.stopCh:
		return
	default:
	}
	close(m.stopCh)
}

// Tick runs the daily evaluation cycle, rotating the access token
// via the refresh_token grant. Exposed for tests.
func (m *Monitor) Tick(ctx context.Context) {
	dec, err := m.cfg.Source.Refresh(ctx)
	if err != nil {
		log.WithError(err).Debug("ideplan monitor: refresh")
	}
	m.react(dec, err)
}

// Reevaluate re-derives gating from the cached token without
// touching the refresh_token grant. Called by bootstrap and
// re-signin after a fresh browser login so the prior session is
// never reused.
func (m *Monitor) Reevaluate(ctx context.Context) {
	dec, err := m.cfg.Source.Decision(ctx)
	if err != nil {
		log.WithError(err).Debug("ideplan monitor: decision")
	}
	m.react(dec, err)
}

func (m *Monitor) react(dec Decision, err error) {
	log.WithFields(log.Fields{
		"status":      dec.Status.String(),
		"signed_in":   dec.SignedIn.String(),
		"plan_ends":   dec.PlanEnds.Format(time.RFC3339),
		"grace_until": dec.GraceUntil.Format(time.RFC3339),
		"err":         err,
	}).Debug("ideplan monitor: tick")
	switch dec.Status {
	case StatusActive:
		m.mu.Lock()
		warningID := m.warningID
		m.warningID = ""
		m.mu.Unlock()
		if warningID != "" {
			if err := m.cfg.Notifications.UpdateNotificationProgress(warningID, "", 100, 100); err != nil {
				log.WithError(err).Debug("ideplan monitor: clear warning")
			}
		}
		m.cfg.Locker.SetLocked(false)
	case StatusGracePeriod:
		m.cfg.Locker.SetLocked(false)
		m.maybeNotifyGrace(dec)
	case StatusExpired, StatusNeverSubscribed:
		m.cfg.Locker.SetLocked(true)
	}
}

func (m *Monitor) run(ctx context.Context) {
	m.Tick(ctx)
	ticker := time.NewTicker(m.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.Tick(ctx)
		}
	}
}

func (m *Monitor) maybeNotifyGrace(dec Decision) {
	now := m.cfg.Now()
	utc := now.UTC()
	day := time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
	m.mu.Lock()
	if m.lastNotifiedDay.Equal(day) {
		m.mu.Unlock()
		return
	}
	m.lastNotifiedDay = day
	m.mu.Unlock()

	daysLeft := int(dec.GraceUntil.Sub(now).Hours()/24) + 1
	if daysLeft < 1 {
		daysLeft = 1
	}
	msg := fmt.Sprintf(
		"Rune Pro subscription expired. %d day(s) remaining before access is locked.",
		daysLeft,
	)
	id, err := m.cfg.Notifications.Notify(browserapi.LevelWarn, "Rune Pro subscription expired.")
	if err != nil {
		log.WithError(err).Debug("ideplan monitor: notify")
		return
	}
	if id == "" {
		return
	}
	m.mu.Lock()
	m.warningID = id
	m.mu.Unlock()
	if err := m.cfg.Notifications.UpdateNotificationProgress(id, msg, 99, 100); err != nil {
		log.WithError(err).Debug("ideplan monitor: update progress")
	}
}
