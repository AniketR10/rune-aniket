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

package crashreport

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"gopkg.in/yaml.v3"
)

// Uploader is the interface for uploading crash reports to the server.
type Uploader interface {
	PostReport(ctx context.Context, payload Payload) error
}

// debugLogTailLines is the number of lines to include from the debug log.
const debugLogTailLines = 50

// Payload is the upload payload for a crash report.
type Payload struct {
	// YAML is a complete YAML crash report document. When debug logs are
	// available, they are embedded as a top-level "logs" field in this
	// document rather than wrapped in a secondary transport format.
	YAML []byte
}

// ReportState is the persisted state for a single crash report.
type ReportState struct {
	ReportID  string    `json:"ReportID"`
	Path      string    `json:"Path"`
	CreatedAt time.Time `json:"CreatedAt"`
	Sent      bool      `json:"Sent"`
	SentAt    time.Time `json:"SentAt"`
	Declined  bool      `json:"Declined"`
	LastError string    `json:"LastError,omitempty"`
}

// Manager scans for crash reports, tracks their state, and
// coordinates uploads.
type Manager struct {
	reportsDir   string
	debugLogPath string
	storage      storageapi.Service
	uploader     Uploader
}

// NewManager creates a crash report manager.
func NewManager(
	reportsDir string,
	debugLogPath string,
	storage storageapi.Service,
	uploader Uploader,
) *Manager {
	return &Manager{
		reportsDir:   reportsDir,
		debugLogPath: debugLogPath,
		storage:      storage,
		uploader:     uploader,
	}
}

// reportID computes a stable ID from the file path and content hash.
func reportID(path string, data []byte) string {
	h := sha256.New()
	h.Write([]byte(path))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// PendingReports scans the reports directory and returns reports that
// have not been sent or declined.
func (m *Manager) PendingReports(ctx context.Context) ([]ReportState, error) {
	entries, err := os.ReadDir(m.reportsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read reports dir %q: %w", m.reportsDir, err)
	}

	var pending []ReportState
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.Contains(entry.Name(), "crash_report") {
			continue
		}

		reportPath := filepath.Join(m.reportsDir, entry.Name())
		data, err := os.ReadFile(reportPath)
		if err != nil {
			log.Warnf("read report %q: %v", reportPath, err)
			continue
		}

		id := reportID(reportPath, data)

		// Check if we already have state for this report.
		var state ReportState
		err = m.storage.Get(ctx, id, &state)
		if err == nil {
			// We have state: skip if already sent or declined.
			if state.Sent || state.Declined {
				continue
			}
			pending = append(pending, state)
			continue
		}
		if !errors.Is(err, storageapi.ErrNotFound) {
			log.Warnf("get report state %q: %v", id, err)
			continue
		}

		// New report: create initial state.
		info, _ := entry.Info()
		created := time.Now()
		if info != nil {
			created = info.ModTime()
		}
		state = ReportState{
			ReportID:  id,
			Path:      reportPath,
			CreatedAt: created,
		}
		if err := m.storage.Create(ctx, id, &state); err != nil &&
			!errors.Is(err, storageapi.ErrAlreadyExists) {
			log.Warnf("create report state %q: %v", id, err)
		}
		pending = append(pending, state)
	}
	return pending, nil
}

// BuildPayload constructs the upload payload for a report, including
// the crash report content and the last 50 lines from the debug log.
func (m *Manager) BuildPayload(_ context.Context, state ReportState) (Payload, error) {
	data, err := os.ReadFile(state.Path)
	if err != nil {
		return Payload{}, fmt.Errorf("read report %q: %w", state.Path, err)
	}

	var logs string
	if m.debugLogPath != "" {
		logs = tailFile(m.debugLogPath, debugLogTailLines)
	}
	data, err = withLogs(data, logs)
	if err != nil {
		return Payload{}, fmt.Errorf("add logs to report %q: %w", state.Path, err)
	}

	return Payload{
		YAML: data,
	}, nil
}

func withLogs(data []byte, logs string) ([]byte, error) {
	if logs == "" {
		return data, nil
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("yaml decode: %w", err)
	}
	mapping := rootMappingNode(&doc)
	if mapping == nil {
		return nil, fmt.Errorf("report root must be a YAML mapping")
	}
	setYAMLString(mapping, "logs", logs)

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(4)
	if err := enc.Encode(&doc); err != nil {
		return nil, fmt.Errorf("yaml encode: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("close yaml encoder: %w", err)
	}
	return buf.Bytes(), nil
}

func rootMappingNode(doc *yaml.Node) *yaml.Node {
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil
	}
	return root
}

func setYAMLString(mapping *yaml.Node, key, value string) {
	valueNode := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: value,
	}
	if strings.Contains(value, "\n") {
		valueNode.Style = yaml.LiteralStyle
	}

	for i := 0; i < len(mapping.Content)-1; i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content[i+1] = valueNode
			return
		}
	}

	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		valueNode,
	)
}

// MarkSent records a report as successfully uploaded.
func (m *Manager) MarkSent(ctx context.Context, id string) error {
	return m.storage.Set(ctx, id, &ReportState{
		ReportID: id,
		Sent:     true,
		SentAt:   time.Now(),
	})
}

// MarkDeclined records that the user declined to send a report.
func (m *Manager) MarkDeclined(ctx context.Context, id string) error {
	return m.storage.Set(ctx, id, &ReportState{
		ReportID: id,
		Declined: true,
	})
}

// SendReports uploads all the given reports and marks them sent.
// Returns the number successfully sent and the first error encountered.
func (m *Manager) SendReports(ctx context.Context, reports []ReportState) (int, error) {
	sent := 0
	for _, r := range reports {
		payload, err := m.BuildPayload(ctx, r)
		if err != nil {
			log.Warnf("build payload for %q: %v", r.ReportID, err)
			_ = m.setLastError(ctx, r.ReportID, err.Error())
			continue
		}
		if err := m.uploader.PostReport(ctx, payload); err != nil {
			log.Warnf("upload report %q: %v", r.ReportID, err)
			_ = m.setLastError(ctx, r.ReportID, err.Error())
			return sent, err
		}
		if err := m.MarkSent(ctx, r.ReportID); err != nil {
			log.Warnf("mark sent %q: %v", r.ReportID, err)
		}
		sent++
	}
	return sent, nil
}

// tailFile returns the last n lines from the file at path.
// If the file cannot be read, it returns an empty string.
func tailFile(path string, n int) string {
	f, err := os.Open(path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Warnf("open debug log %q: %v", path, err)
		}
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// Use a ring buffer to keep the last n lines.
	ring := make([]string, 0, n)
	for scanner.Scan() {
		if len(ring) < n {
			ring = append(ring, scanner.Text())
		} else {
			copy(ring, ring[1:])
			ring[n-1] = scanner.Text()
		}
	}
	if err := scanner.Err(); err != nil {
		log.Warnf("scan debug log %q: %v", path, err)
	}
	if len(ring) == 0 {
		return ""
	}
	return strings.Join(ring, "\n") + "\n"
}

// DeclineReports marks all the given reports as declined.
func (m *Manager) DeclineReports(ctx context.Context, reports []ReportState) {
	for _, r := range reports {
		if err := m.MarkDeclined(ctx, r.ReportID); err != nil {
			log.Warnf("mark declined %q: %v", r.ReportID, err)
		}
	}
}

func (m *Manager) setLastError(ctx context.Context, id, errMsg string) error {
	return m.storage.Update(ctx, id, []storageapi.Update{
		{FieldPath: []string{"LastError"}, Value: errMsg},
	})
}
