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
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"

	"cloud.google.com/go/storage"
	"google.golang.org/api/googleapi"
)

// GCSReportStore stores reports as objects in a GCS bucket.
type GCSReportStore struct {
	bucket *storage.BucketHandle
}

// NewGCSReportStore creates a new GCS-backed report store.
func NewGCSReportStore(bucket *storage.BucketHandle) *GCSReportStore {
	return &GCSReportStore{bucket: bucket}
}

// Store writes the report data to GCS under the given object name,
// attaching any metadata to the object.
func (s *GCSReportStore) Store(
	ctx context.Context, objectName string, data []byte, contentType string, metadata map[string]string,
) error {
	obj := s.bucket.Object(objectName).If(storage.Conditions{DoesNotExist: true})
	w := obj.NewWriter(ctx)
	w.ContentType = contentType
	w.Metadata = metadata

	if _, err := bytes.NewReader(data).WriteTo(w); err != nil {
		_ = w.Close()
		return fmt.Errorf("write report to GCS %q: %w", objectName, err)
	}
	if err := w.Close(); err != nil {
		if isAlreadyExistsError(err) {
			return ErrReportAlreadyExists
		}
		return fmt.Errorf("close GCS writer %q: %w", objectName, err)
	}
	return nil
}

func isAlreadyExistsError(err error) bool {
	var googleErr *googleapi.Error
	return errors.As(err, &googleErr) && googleErr.Code == http.StatusPreconditionFailed
}

// Verify interface compliance at compile time.
var _ ReportStore = (*GCSReportStore)(nil)
