// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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

package text

import (
	"math"
	"sort"

	"unstable.build/go-tui/api/textapi"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/term"
)

// LocationStore manages LocationLists and priorities and provides
// convenience methods to set, retrieve and sort locations.
type LocationStore struct {
	locs          map[string]LocationList          // used by cursor moves
	drawLocations map[string]*priorityLocationList // used to draw
	locsSliceTemp []*priorityLocationList
	locsSlice     []textapi.Location
	messages      map[term.Coordinates][]message
}

// NewLocationStore allocates storage for a new LocationStore and initializes it.
func NewLocationStore() *LocationStore {
	ret := new(LocationStore)
	ret.Init()
	return ret
}

// Init initializes this LocationStore.
func (c *LocationStore) Init() {
	c.locs = make(map[string]LocationList)
	c.drawLocations = make(map[string]*priorityLocationList)
	c.messages = make(map[term.Coordinates][]message)
}

// LocationList returns the location list identified by ID, set previously via SetLocationList,
// or nil and false, if no location list is currently set with the given ID.
func (c *LocationStore) LocationList(ID string) (LocationList, bool) {
	l, ok := c.locs[ID]
	return l, ok
}

// SortedLocations returns all the locations sorted by priority level. If two
// location lists have the same priority level, then the location list ID is used
// to disambiguate order.
func (c *LocationStore) SortedLocations() []textapi.Location {
	// re-use previous allocation
	c.locsSlice = c.locsSlice[:0]
	c.locsSliceTemp = c.locsSliceTemp[:0]
	for _, list := range c.drawLocations {
		c.locsSliceTemp = append(c.locsSliceTemp, list)
	}

	sort.Slice(c.locsSliceTemp, func(i, j int) bool {
		return c.locsSliceTemp[i].priority <
			c.locsSliceTemp[j].priority ||
			(c.locsSliceTemp[i].priority == c.locsSliceTemp[j].priority &&
				c.locsSliceTemp[i].ID < c.locsSliceTemp[j].ID)
	})

	for _, list := range c.locsSliceTemp {
		c.locsSlice = append(c.locsSlice, list.locations...)
	}
	return c.locsSlice
}

// SetLocationList sets a location list on this cursor. It substitutes and returns
// the previous location list with the same ID, if there was any.
// Any calls to Insert on the underlying Writer will reset all location lists.
func (c *LocationStore) SetLocationList(
	pri textapi.LocationPriority, ID string, l LocationList,
) LocationList {
	prev, ok := c.locs[ID]
	if ok {
		c.clearMessages(ID)
		c.drawLocations[ID].locations = c.drawLocations[ID].locations[:0]
		c.drawLocations[ID].priority = pri
	} else {
		c.drawLocations[ID] = &priorityLocationList{ID: ID, priority: pri}
	}

	if l == nil {
		delete(c.locs, ID)
	} else {
		c.locs[ID] = l
		c.processList(ID, l)
	}

	return prev
}

// LocationsAtCoordinates returns the set of locations by location list ID set by SetLocationList,
// at the given coordinates, if there's any.
func (c *LocationStore) LocationsAtCoordinates(pos term.Coordinates) (
	map[string]textapi.Location, bool,
) {
	msgs, ok := c.messages[pos]
	if !ok {
		return nil, false
	}
	if len(msgs) == 0 {
		return nil, false
	}
	ret := make(map[string]textapi.Location, len(msgs))
	for _, msg := range msgs {
		ret[msg.listID] = msg.location
	}
	return ret, true
}

// DrawLocations draws the given locations using the given writer.
// This is intended to be used alongside Scroll's Draw method.
func DrawLocations(locations []textapi.Location, scroll *component.Scroll, w term.Writer) {
	if scroll.Width() == 0 {
		return // avoid division by 0 in ScrollToWindowCoordinates
	}

	offset := scroll.Offset()
	height := scroll.SizeHeight()
	buffer := scroll.Buffer()

	// this might add some extra calls to UnionAttributes that are noop but
	// it's way more efficient that calculating screen coordinates on every
	// location.
	minY := offset.Y
	maxY := offset.Y + height + scroll.HiddenLineCount()
	for _, loc := range locations {
		fromAtScroll := loc.From
		toAtScroll := loc.To
		if fromAtScroll.Y >= maxY || fromAtScroll.Y < minY {
			continue
		}

		fromAtScrollX := fromAtScroll.X
		toAtScroll.Y = int(math.Min(float64(buffer.Rows()-1), float64(toAtScroll.Y)))
		for y := fromAtScroll.Y; y < toAtScroll.Y; y++ {
			toX := buffer.Columns(y)
			for x := fromAtScrollX; x < toX; x++ {
				posAtScreen, ok := scroll.ScrollToWindowCoordinates(term.Coordinates{Y: y, X: x})
				if posAtScreen.X >= scroll.Width() {
					break
				}
				if !ok || posAtScreen.X < 0 {
					continue
				}
				w.UnionAttributes(posAtScreen, loc.Attr)
			}
			fromAtScrollX = 0
		}

		toX := toAtScroll.X
		for x := fromAtScrollX; x < toX; x++ {
			posAtScreen, ok := scroll.ScrollToWindowCoordinates(term.Coordinates{Y: toAtScroll.Y, X: x})
			if posAtScreen.X >= scroll.Width() {
				break
			}
			if !ok || posAtScreen.X < 0 {
				continue
			}
			w.UnionAttributes(posAtScreen, loc.Attr)
		}
	}
}

func (c *LocationStore) clearMessages(ID string) {
	for from, msgs := range c.messages {
		var stay []message
		for _, msg := range msgs {
			if msg.listID == ID {
				continue
			}
			stay = append(stay, msg)
		}
		c.messages[from] = stay
	}
}

func (c *LocationStore) processList(ID string, l LocationList) {
	scrollStartList(l)
	for n, ok := l.Current(); ok; n, ok = l.Next() {
		c.drawLocations[ID].locations = append(c.drawLocations[ID].locations, n)
		if n.Message == "" {
			continue
		}
		from, to := term.CoordinatesSort(n.From, n.To)
		for {
			msgs, ok := c.messages[from]
			if !ok {
				msgs = make([]message, 0, 1)
				c.messages[from] = msgs
			}
			c.messages[from] = append(msgs, message{
				listID:   ID,
				location: n,
			})
			if from.Y == to.Y && from.X == to.X {
				break
			}
			if from.Y == to.Y {
				from.X++
				continue
			}

			from.Y++
			from.X = 0
		}
	}
}

type priorityLocationList struct {
	locations []textapi.Location
	ID        string
	priority  textapi.LocationPriority
}

func scrollStartList(l LocationList) {
	for ok := true; ok; _, ok = l.Prev() {
	}
}
