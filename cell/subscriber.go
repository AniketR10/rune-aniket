// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package cell

import (
	"context"

	"github.com/unstablebuild/rune-go-sdk/term"
)

// Subscriber is the interface that wraps methods to receive to updates to
// an underlying cell.Editor.
//
// Subscribers MUST NOT have mutable access to the underlying cell.Editor
// they're subscribing to, as updates are published synchronously so
// program could enter in an infinite loop.
//
// Subscribers constructors SHOULD subscribe to a Publisher upon initialization.
type Subscriber interface {
	// OnWillEdit start, end and str correspond the input values
	// to an imminent call to Edit.
	OnWillEdit(ctx context.Context, start, end term.Coordinates, str string)
	// OnDidEdit from, to and old correspond to the return values
	// of a call to Edit. See cell.Editor.Edit for more details.
	OnDidEdit(ctx context.Context, from, to term.Coordinates, old string)
}

// Publisher is the interface that wraps the Subscribe method.
//
// Subscribe enables subscription of insert/delete events. See Subscriber.
type Publisher interface {
	Subscribe(Subscriber)
	Unsubscribe(Subscriber)
}

// PublisherView is the interface that groups Publisher and View.
//
// This interface should be used within Subscribers which need to read from
// a View when handling updates. See Subscriber.
type PublisherView interface {
	Publisher
	View
}
