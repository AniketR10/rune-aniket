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

package oxapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
	blueauth "github.com/unstablebuild/blue/auth"
	"github.com/unstablebuild/blue/logging"
	"unstable.build/go-tui/cmd/ox-api/auth"
)

const (
	telemetryClass    = "telemetryHandler"
	telemetryCallType = "telemetry"
	dateTimeFormat    = time.RFC3339
)

// NewTelemetryHandler returns the telemetry endpoint handler with optional auth enrichment.
func NewTelemetryHandler(
	logger *log.Logger,
	keys blueauth.Keys,
) http.Handler {
	authHandler := telemetryAuthenticatedHandler{logger: logger}
	config := blueauth.MiddlewareConfig[auth.RPCUser]{
		VerifyKeys:   keys,
		Authorizer:   alwaysAuthorize{},
		SuccessLevel: log.TraceLevel,
		FailureLevel: log.DebugLevel,
	}
	middleware := blueauth.WithMiddleware(authHandler, config)
	return &telemetryHandler{
		logger:     logger,
		middleware: middleware,
	}
}

// telemetryHandler attempts to use the auth middleware to authenticate a telemetry
// payload, and if it succeeds, logs payload along with user claim IDs. Otherwise it fallbacks
// to log the payload as-is for unauthenticated users.
type telemetryHandler struct {
	logger     *log.Logger
	middleware http.Handler
}

func (t *telemetryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// NOTE real-ip header would be PII, leave it out for now
	// clone body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Errorf("read telemetry payload: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	wi := interceptHttpWriter{}
	t.middleware.ServeHTTP(&wi, r)

	if wi.code == http.StatusOK {
		// telemetryAuthenticatedHandler logged successfully
		w.WriteHeader(http.StatusOK)
		return
	}

	// fallback to log without auth fields
	var fields log.Fields
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&fields); err != nil {
		log.Errorf("json decode telemetry payload: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	fields["Auth"] = false
	fields[logging.KeyCallType] = telemetryCallType
	fields[logging.KeyClass] = telemetryClass

	t.logger.WithFields(fields).Info()

	w.WriteHeader(http.StatusOK)
}

type telemetryAuthenticatedHandler struct {
	logger *log.Logger
}

func (t telemetryAuthenticatedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	claims, ok := blueauth.ClaimsFromContext[auth.RPCUser](r.Context())
	if !ok {
		err := "claims not found in user request context"
		log.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var fields log.Fields
	err := json.NewDecoder(r.Body).Decode(&fields)
	if err != nil {
		log.Errorf("json decode telemetry payload: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	fields["Auth.UserID"] = claims.UserID
	fields["Auth.Role"] = claims.Extra.Role.String()
	fields["Auth.Expiry"] = claims.Expiry.Time().Format(dateTimeFormat)
	fields["Auth.IssuedAt"] = claims.IssuedAt.Time().Format(dateTimeFormat)
	fields["Auth.Subject"] = claims.Subject
	fields["Auth.ID"] = claims.ID
	fields["Auth"] = true
	fields[logging.KeyCallType] = telemetryCallType
	fields[logging.KeyClass] = telemetryClass

	// no PII in telemetry logs
	// m["Auth.Email"] = claims.Email
	// m["Auth.Extra.Email"] = claims.Extra.Email

	// not needed for now
	// m["Auth.Issuer"] = claims.Issuer
	// m["Auth.Audience"] = claims.Audience

	t.logger.WithFields(fields).Info()

	w.WriteHeader(http.StatusOK)
}

type interceptHttpWriter struct {
	code   int
	header http.Header
}

func (w *interceptHttpWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *interceptHttpWriter) Write(data []byte) (int, error) {
	if w.code == 0 {
		w.code = http.StatusOK
	}
	return len(data), nil
}

func (w *interceptHttpWriter) WriteHeader(statusCode int) {
	w.code = statusCode
}

type alwaysAuthorize struct {
}

func (a alwaysAuthorize) Authorize(
	ctx context.Context, claims blueauth.UserClaims[auth.RPCUser], resource string,
) error {
	return nil
}
