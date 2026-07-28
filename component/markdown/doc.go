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

// Package markdown provides a terminal-based markdown rendering component.
//
// The component parses markdown content using goldmark and renders it
// with proper formatting for terminal display, supporting headers, paragraphs,
// lists (ordered, unordered, task), code blocks, blockquotes, tables,
// and horizontal rules.
//
// # Basic Usage
//
//	md := markdown.New("# Hello World\n\nThis is a **bold** statement.")
//	md.Resize(80, 24)
//	md.Draw(writer)
//
// # Scrolling
//
// The component implements component.ScrollableFloating, allowing it to be
// scrolled when content exceeds the viewport:
//
//	md.SeekDown() // scroll down one line
//	md.SeekUp()   // scroll up one line
//
// # Customization
//
// Use NewWithConfig to customize colors and styling:
//
//	cfg := markdown.DefaultConfig()
//	cfg.H1 = term.Attributes{Fg: term.ColorRed, Attrs: term.AttrBold}
//	md := markdown.NewWithConfig(content, cfg)
package markdown
