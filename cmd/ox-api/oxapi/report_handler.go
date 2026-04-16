// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2023-2025 Unstable Build, All Rights Reserved.
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

package oxapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	blueauth "github.com/unstablebuild/blue/auth"
	"gopkg.in/yaml.v3"
	"unstable.build/go-tui/cmd/rune/auth"
)

const (
	reportClass    = "reportHandler"
	reportCallType = "report"

	// DefaultReportMaxBytes is the default maximum size of a report
	// payload in bytes (5 MB).
	DefaultReportMaxBytes int64 = 5 * 1024 * 1024

	reportContentType = "application/yaml"

	// DefaultReportRateLimitWindow is the default interval over which reports
	// with the same fingerprint are bucketed into a single object.
	DefaultReportRateLimitWindow = time.Hour
)

// ReportStore is the interface for persisting report payloads.
type ReportStore interface {
	// Store persists a report under the given object name.
	// The data is the raw YAML payload. The metadata map may contain
	// additional information to store alongside the object (e.g. user ID).
	// The implementation should return ErrReportAlreadyExists if objectName
	// already exists so duplicate reports can be rate-limited.
	Store(ctx context.Context, objectName string, data []byte, contentType string, metadata map[string]string) error
}

// ErrReportAlreadyExists indicates that a report bucket object already exists.
var ErrReportAlreadyExists = errors.New("report already exists")

// ReportConfig holds configuration for the report handler.
type ReportConfig struct {
	// MaxBytes is the maximum allowed request body size.
	// If zero, DefaultReportMaxBytes is used.
	MaxBytes int64

	// Prefix is a path prefix for GCS object names (e.g. "reports/").
	Prefix string

	// RateLimitWindow buckets duplicate reports with the same fingerprint into
	// a single object for this duration. If zero, DefaultReportRateLimitWindow
	// is used.
	RateLimitWindow time.Duration
}

// newReportHandler returns the report endpoint handler.
// Authentication is applied externally via RPC middleware in http.go.
func newReportHandler(
	logger *log.Logger,
	store ReportStore,
	cfg ReportConfig,
) http.Handler {
	if cfg.MaxBytes == 0 {
		cfg.MaxBytes = DefaultReportMaxBytes
	}
	if cfg.RateLimitWindow == 0 {
		cfg.RateLimitWindow = DefaultReportRateLimitWindow
	}

	return &reportHandler{
		logger: logger,
		store:  store,
		cfg:    cfg,
	}
}

type reportHandler struct {
	logger *log.Logger
	store  ReportStore
	cfg    ReportConfig
}

type reportDocument struct {
	Package  string            `yaml:"package"`
	Version  string            `yaml:"version"`
	Subject  string            `yaml:"subject"`
	Metadata map[string]string `yaml:"metadata"`
}

func (h *reportHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, h.cfg.MaxBytes+1))
	if err != nil {
		log.Errorf("read report payload: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if int64(len(body)) > h.cfg.MaxBytes {
		http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
		return
	}
	if len(body) == 0 {
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	var report reportDocument
	if err := yaml.Unmarshal(body, &report); err != nil {
		log.Errorf("yaml decode report: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if report.Package == "" || report.Version == "" || report.Subject == "" {
		http.Error(w, "package, version and subject are required", http.StatusBadRequest)
		return
	}

	fingerprint := reportFingerprint(report)
	if fingerprint == "" {
		http.Error(w, "report could not be fingerprinted: missing fields", http.StatusBadRequest)
		return
	}

	// Extract authenticated user info from the auth middleware.
	metadata := map[string]string{
		"fingerprint": fingerprint,
		"package":     report.Package,
		"version":     report.Version,
	}
	if claims, ok := blueauth.ClaimsFromContext[auth.RPCUser](r.Context()); ok {
		metadata["user_id"] = claims.UserID
		metadata["subject"] = claims.Subject
	}

	// Build the object name under a date-based path.
	now := time.Now().UTC()
	bucketTime := now.Truncate(h.cfg.RateLimitWindow)
	metadata["bucket_start"] = bucketTime.Format(time.RFC3339)
	metadata["rate_limit_window"] = h.cfg.RateLimitWindow.String()
	objectName := fmt.Sprintf("%s%s/%s_%s.yaml",
		h.cfg.Prefix,
		bucketTime.Format("2006/01/02"),
		bucketTime.Format("1504"),
		fingerprint,
	)
	h.logger.WithFields(log.Fields{
		"class":             reportClass,
		"call_type":         reportCallType,
		"fingerprint":       fingerprint,
		"package":           report.Package,
		"version":           report.Version,
		"object":            objectName,
		"bucket_start":      metadata["bucket_start"],
		"rate_limit_window": metadata["rate_limit_window"],
	}).Info("report create attempted")

	err = h.store.Store(r.Context(), objectName, body, reportContentType, metadata)
	if err != nil {
		if errors.Is(err, ErrReportAlreadyExists) {
			h.logger.WithFields(log.Fields{
				"class":       reportClass,
				"call_type":   reportCallType,
				"fingerprint": fingerprint,
				"package":     report.Package,
				"version":     report.Version,
				"object":      objectName,
			}).Info("duplicate report rate-limited")

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"status":      "duplicate",
				"object":      objectName,
				"fingerprint": fingerprint,
			})
			return
		}

		log.Errorf("store report %q: %v", fingerprint, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	h.logger.WithFields(log.Fields{
		"class":       reportClass,
		"call_type":   reportCallType,
		"fingerprint": fingerprint,
		"package":     report.Package,
		"version":     report.Version,
		"object":      objectName,
	}).Info("report stored")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":      "created",
		"object":      objectName,
		"fingerprint": fingerprint,
	})
}

func reportFingerprint(report reportDocument) string {
	stack := report.Metadata["stack"]
	errMsg := report.Metadata["error"]
	if errMsg == "" {
		errMsg = report.Subject
	}

	parts := make([]string, 0, 2)
	if errMsg != "" {
		parts = append(parts, strings.TrimSpace(errMsg))
	}
	if stack != "" {
		normalized := normalizeStackForFingerprint(stack)
		if normalized == "" {
			normalized = strings.TrimSpace(stack)
		}
		if normalized != "" {
			parts = append(parts, normalized)
		}
	}
	if len(parts) == 0 {
		return ""
	}

	h := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(h[:])[:16]
}

var stackPCRe = regexp.MustCompile(`\s\+0x[0-9a-fA-F]+$`)

func normalizeStackForFingerprint(stack string) string {
	lines := strings.Split(stack, "\n")
	frames := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "goroutine ") || strings.HasPrefix(line, "created by ") {
			continue
		}
		if strings.HasPrefix(line, "/") || strings.Contains(line, ":") && !strings.Contains(line, "(") {
			continue
		}
		line = normalizeFunctionLine(line)
		if line == "" {
			continue
		}
		frames = append(frames, line)
	}
	return strings.Join(frames, "\n")
}

func normalizeFunctionLine(line string) string {
	line = stackPCRe.ReplaceAllString(line, "")
	fn, _, ok := strings.Cut(line, "(")
	if !ok {
		return ""
	}
	return fn
}
