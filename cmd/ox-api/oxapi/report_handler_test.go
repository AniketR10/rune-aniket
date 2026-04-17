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
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// inMemoryReportStore is a test double for ReportStore.
type inMemoryReportStore struct {
	objects      map[string][]byte
	contentTypes map[string]string
	metadata     map[string]map[string]string
	err          error
}

type recordingPager struct {
	pages []Page
	err   error
}

func (p *recordingPager) Page(_ context.Context, page Page) error {
	p.pages = append(p.pages, page)
	return p.err
}

func newInMemoryReportStore() *inMemoryReportStore {
	return &inMemoryReportStore{
		objects:      make(map[string][]byte),
		contentTypes: make(map[string]string),
		metadata:     make(map[string]map[string]string),
	}
}

func (s *inMemoryReportStore) Store(
	_ context.Context, objectName string, data []byte, contentType string, metadata map[string]string,
) error {
	if s.err != nil {
		return s.err
	}
	if _, ok := s.objects[objectName]; ok {
		return ErrReportAlreadyExists
	}
	s.objects[objectName] = data
	s.contentTypes[objectName] = contentType
	s.metadata[objectName] = metadata
	return nil
}

func validReportYAML(subject string) string {
	quotedSubject := strconv.Quote(subject)
	return `package: rune
version: 1.0.0
author: debug.CapturePanic
subject: ` + quotedSubject + `
metadata:
    error: ` + quotedSubject + `
    stack: |
        goroutine 1 [running]:
        runtime/debug.Stack()
            /src/runtime/debug/stack.go:26 +0x64
        unstable.build/go-tui/ide.(*ex).panic(0x1234?, {0xabcd?, 0xef?})
            /workspace/ide/ex.go:1569 +0x2c
`
}

func testReportHandler(store ReportStore, cfg ReportConfig) http.Handler {
	logger := log.New()
	logger.SetLevel(log.TraceLevel)
	return newReportHandler(logger, store, cfg)
}

func TestReportHandler_MethodNotAllowed(t *testing.T) {
	store := newInMemoryReportStore()
	h := testReportHandler(store, ReportConfig{})

	req := httptest.NewRequest(http.MethodGet, "/api/reports", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	assert.Empty(t, store.objects)
}

func TestReportHandler_EmptyBody(t *testing.T) {
	store := newInMemoryReportStore()
	h := testReportHandler(store, ReportConfig{})

	req := httptest.NewRequest(http.MethodPost, "/api/reports",
		strings.NewReader(""))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Empty(t, store.objects)
}

func TestReportHandler_PayloadTooLarge(t *testing.T) {
	store := newInMemoryReportStore()
	cfg := ReportConfig{MaxBytes: 100}
	h := testReportHandler(store, cfg)

	bigPayload := strings.Repeat("x", 200)
	req := httptest.NewRequest(http.MethodPost, "/api/reports",
		strings.NewReader(bigPayload))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	assert.Empty(t, store.objects)
}

func TestReportHandler_InvalidYAML(t *testing.T) {
	store := newInMemoryReportStore()
	h := testReportHandler(store, ReportConfig{})

	req := httptest.NewRequest(http.MethodPost, "/api/reports",
		strings.NewReader("not: [yaml"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Empty(t, store.objects)
}

func TestReportHandler_MissingRequiredFields(t *testing.T) {
	store := newInMemoryReportStore()
	h := testReportHandler(store, ReportConfig{})

	payload := `report: stack trace`
	req := httptest.NewRequest(http.MethodPost, "/api/reports",
		strings.NewReader(payload))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Empty(t, store.objects)
}

func TestReportHandler_Success(t *testing.T) {
	store := newInMemoryReportStore()
	cfg := ReportConfig{Prefix: "reports/", RateLimitWindow: time.Minute}
	h := testReportHandler(store, cfg)

	body := []byte(validReportYAML("panic: runtime error"))

	req := httptest.NewRequest(http.MethodPost, "/api/reports",
		strings.NewReader(string(body)))
	req.Header.Set("Content-Type", reportContentType)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	// Verify response body.
	var resp map[string]string
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "created", resp["status"])
	assert.Contains(t, resp["object"], "reports/")
	assert.Contains(t, resp["object"], resp["fingerprint"])
	assert.True(t, strings.HasSuffix(resp["object"], ".yaml"))

	// Verify store received the object.
	require.Len(t, store.objects, 1)
	for objName, data := range store.objects {
		assert.Contains(t, objName, "reports/")
		assert.Contains(t, objName, resp["fingerprint"])
		assert.True(t, strings.HasSuffix(objName, ".yaml"))
		assert.Equal(t, body, data)
		assert.Equal(t, reportContentType, store.contentTypes[objName])

		meta := store.metadata[objName]
		assert.Equal(t, resp["fingerprint"], meta["fingerprint"])
		assert.Equal(t, "rune", meta["package"])
		assert.Equal(t, "1.0.0", meta["version"])
		assert.Equal(t, time.Minute.String(), meta["rate_limit_window"])
	}
}

func TestReportHandler_DuplicateReportIsRateLimited(t *testing.T) {
	store := newInMemoryReportStore()
	h := testReportHandler(store, ReportConfig{Prefix: "reports/", RateLimitWindow: time.Hour})
	body := validReportYAML("panic: same crash")

	firstReq := httptest.NewRequest(http.MethodPost, "/api/reports", strings.NewReader(body))
	first := httptest.NewRecorder()
	h.ServeHTTP(first, firstReq)
	require.Equal(t, http.StatusCreated, first.Code)

	secondReq := httptest.NewRequest(http.MethodPost, "/api/reports", strings.NewReader(body))
	second := httptest.NewRecorder()
	h.ServeHTTP(second, secondReq)
	require.Equal(t, http.StatusOK, second.Code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(second.Body).Decode(&resp))
	assert.Equal(t, "duplicate", resp["status"])
	require.Len(t, store.objects, 1)
}

func TestReportHandler_DuplicateReportDoesNotPage(t *testing.T) {
	store := newInMemoryReportStore()
	store.err = ErrReportAlreadyExists
	pager := &recordingPager{}
	h := testReportHandler(store, ReportConfig{
		Prefix:          "reports/",
		RateLimitWindow: time.Hour,
		Pager:           pager,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/reports",
		strings.NewReader(validReportYAML("panic: already stored")))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "duplicate", resp["status"])
	assert.Empty(t, pager.pages)
}

func TestReportHandler_PagesAfterNewReportIsStored(t *testing.T) {
	store := newInMemoryReportStore()
	pager := &recordingPager{}
	h := testReportHandler(store, ReportConfig{
		Bucket:          "rune-reports",
		Prefix:          "reports/",
		RateLimitWindow: time.Hour,
		Pager:           pager,
	})
	body := validReportYAML("panic: report me")

	firstReq := httptest.NewRequest(http.MethodPost, "/api/reports", strings.NewReader(body))
	first := httptest.NewRecorder()
	h.ServeHTTP(first, firstReq)
	require.Equal(t, http.StatusCreated, first.Code)

	require.Len(t, pager.pages, 1)
	page := pager.pages[0]
	assert.Equal(t, "Rune crash report: panic: report me", page.Summary)
	assert.Equal(t, PageSeverityError, page.Severity)
	assert.Equal(t, "ox-api", page.Component)
	assert.Equal(t, "rune", page.Group)
	assert.Equal(t, "crash-report", page.Class)
	assert.Contains(t, page.Source, "gs://rune-reports/reports/")
	assert.Contains(t, page.DedupKey, "rune-crash:reports/")
	assert.Equal(t, "rune-reports", page.Details["bucket"])
	assert.Equal(t, "rune", page.Details["package"])
	assert.Equal(t, "1.0.0", page.Details["version"])
	assert.Equal(t, "panic: report me", page.Details["subject"])

	reportMetadata, ok := page.Details["report_metadata"].(map[string]string)
	require.True(t, ok)
	assert.Equal(t, "panic: report me", reportMetadata["error"])
	assert.NotContains(t, reportMetadata, "stack")

	secondReq := httptest.NewRequest(http.MethodPost, "/api/reports", strings.NewReader(body))
	second := httptest.NewRecorder()
	h.ServeHTTP(second, secondReq)
	require.Equal(t, http.StatusOK, second.Code)
	require.Len(t, pager.pages, 1, "duplicate reports should not page")
}

func TestReportHandler_PagerFailureDoesNotFailUpload(t *testing.T) {
	store := newInMemoryReportStore()
	pager := &recordingPager{err: errors.New("pager unavailable")}
	h := testReportHandler(store, ReportConfig{
		Prefix:          "reports/",
		RateLimitWindow: time.Hour,
		Pager:           pager,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/reports", strings.NewReader(validReportYAML("panic: still store")))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	require.Len(t, store.objects, 1)
	require.Len(t, pager.pages, 1)
}

func TestReportHandler_LogsCreateAttemptForEachFingerprintAttempt(t *testing.T) {
	store := newInMemoryReportStore()
	logger := log.New()
	var logBuf bytes.Buffer
	logger.SetOutput(&logBuf)
	logger.SetFormatter(&log.JSONFormatter{})
	h := newReportHandler(logger, store, ReportConfig{
		Prefix:          "reports/",
		RateLimitWindow: time.Hour,
	})
	body := validReportYAML("panic: analytics crash")

	firstReq := httptest.NewRequest(http.MethodPost, "/api/reports", strings.NewReader(body))
	first := httptest.NewRecorder()
	h.ServeHTTP(first, firstReq)
	require.Equal(t, http.StatusCreated, first.Code)

	secondReq := httptest.NewRequest(http.MethodPost, "/api/reports", strings.NewReader(body))
	second := httptest.NewRecorder()
	h.ServeHTTP(second, secondReq)
	require.Equal(t, http.StatusOK, second.Code)

	attempts := 0
	var fingerprint string
	scanner := bufio.NewScanner(strings.NewReader(logBuf.String()))
	for scanner.Scan() {
		var entry map[string]any
		require.NoError(t, json.Unmarshal(scanner.Bytes(), &entry))
		if entry["msg"] != "report create attempted" {
			continue
		}
		attempts++
		assert.Equal(t, reportClass, entry["class"])
		assert.Equal(t, reportCallType, entry["call_type"])
		assert.Equal(t, "rune", entry["package"])
		assert.Equal(t, "1.0.0", entry["version"])

		entryFingerprint, _ := entry["fingerprint"].(string)
		if fingerprint == "" {
			fingerprint = entryFingerprint
		} else {
			assert.Equal(t, fingerprint, entryFingerprint)
		}
	}
	require.NoError(t, scanner.Err())
	assert.Equal(t, 2, attempts)
	assert.NotEmpty(t, fingerprint)
}

func TestReportFingerprint_IgnoresStackAddressesAndLocations(t *testing.T) {
	stackA := `goroutine 1 [running]:
runtime/debug.Stack()
    /tmp/go/src/runtime/debug/stack.go:26 +0x64
unstable.build/go-tui/ide.(*ex).panic(0x1234?, {0xabcd?, 0xef?})
    /work/a/ide/ex.go:1569 +0x2c
panic({0x1000?, 0x2000?})
    /tmp/go/src/runtime/panic.go:860 +0x12c
`
	stackB := `goroutine 99 [running]:
runtime/debug.Stack()
    /different/go/src/runtime/debug/stack.go:28 +0x99
unstable.build/go-tui/ide.(*ex).panic(0x9876?, {0x1111?, 0x2222?})
    /work/b/ide/ex.go:42 +0xbeef
panic({0x3000?, 0x4000?})
    /different/go/src/runtime/panic.go:1000 +0xff
`

	fingerprintA := reportFingerprint(reportDocument{
		Subject:  "panic",
		Metadata: map[string]string{"stack": stackA},
	})
	fingerprintB := reportFingerprint(reportDocument{
		Subject:  "panic",
		Metadata: map[string]string{"stack": stackB},
	})

	assert.NotEmpty(t, fingerprintA)
	assert.Equal(t, fingerprintA, fingerprintB)
}

func TestReportFingerprint_IncludesErrorMessage(t *testing.T) {
	stack := `goroutine 1 [running]:
runtime/debug.Stack()
    /tmp/go/src/runtime/debug/stack.go:26 +0x64
unstable.build/go-tui/ide.(*ex).panic(0x1234?)
    /work/ide/ex.go:1569 +0x2c
`

	fingerprintA := reportFingerprint(reportDocument{
		Subject:  "panic A",
		Metadata: map[string]string{"error": "panic A", "stack": stack},
	})
	fingerprintB := reportFingerprint(reportDocument{
		Subject:  "panic B",
		Metadata: map[string]string{"error": "panic B", "stack": stack},
	})

	assert.NotEqual(t, fingerprintA, fingerprintB)
}

func TestReportHandler_StorageFailure(t *testing.T) {
	store := newInMemoryReportStore()
	store.err = errors.New("GCS unavailable")
	h := testReportHandler(store, ReportConfig{})

	payload := validReportYAML("crash")
	req := httptest.NewRequest(http.MethodPost, "/api/reports",
		strings.NewReader(payload))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
