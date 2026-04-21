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

import "context"

type nopPager struct{}

func (nopPager) Page(context.Context, Page) error { return nil }

const (
	// PageSeverityInfo indicates an informational page event.
	PageSeverityInfo = "info"
	// PageSeverityWarning indicates a warning page event.
	PageSeverityWarning = "warning"
	// PageSeverityError indicates an error page event.
	PageSeverityError = "error"
	// PageSeverityCritical indicates a critical page event.
	PageSeverityCritical = "critical"
)

// Pager abstracts paging providers so report handling can be tested without
// calling external services, and so other paging implementations can be added
// later without changing the report handler.
type Pager interface {
	Page(ctx context.Context, page Page) error
}

// Page describes a provider-agnostic paging event.
type Page struct {
	Summary   string
	Source    string
	Severity  string
	Component string
	Group     string
	Class     string
	DedupKey  string
	Details   map[string]any
	Links     []PageLink
}

// PageLink is an optional link attached to a page event.
type PageLink struct {
	Href string
	Text string
}
