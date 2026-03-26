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

package drawrect

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

func abs(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

type point struct {
	x float32
	y float32
}

type subpath struct {
	points []point
}

func (s *subpath) pointCount() int {
	return len(s.points)
}

func (s *subpath) lastPoint() point {
	return s.points[len(s.points)-1]
}

func (s *subpath) appendPoint(pt point) {
	// Do not add a too close point to the last point.
	// This can cause unexpected rendering results.
	if lp := s.lastPoint(); abs(lp.x-pt.x) < 1e-2 && abs(lp.y-pt.y) < 1e-2 {
		return
	}

	s.points = append(s.points, pt)
}

// Path represents a collection of path subpathments.
type Path struct {
	subpath
}

// Reset resets this path.
func (p *Path) Reset() {
	p.points = p.points[:0]
}

// MoveTo starts a new subpath with the given position (x, y) without adding a subpath,
func (p *Path) MoveTo(x, y float32) {
	p.points = append(p.points, point{x: x, y: y})
}

// LineTo adds a line segment to the path, which starts from the last position of the current subpath
// and ends to the given position (x, y).
// If p doesn't have any subpaths or the last subpath is closed, LineTo sets (x, y) as the start position of a new subpath.
func (p *Path) LineTo(x, y float32) {
	p.appendPoint(point{x: x, y: y})
}

// lineForTwoPoints returns parameters for a line passing through p0 and p1.
func lineForTwoPoints(p0, p1 point) (a, b, c float32) {
	// Line passing through p0 and p1 in the form of ax + by + c = 0
	a = p1.y - p0.y
	b = -(p1.x - p0.x)
	c = (p1.x-p0.x)*p0.y - (p1.y-p0.y)*p0.x
	return
}

// crossingPointForTwoLines returns a crossing point for two lines.
func crossingPointForTwoLines(p00, p01, p10, p11 point) point {
	a0, b0, c0 := lineForTwoPoints(p00, p01)
	a1, b1, c1 := lineForTwoPoints(p10, p11)
	det := a0*b1 - a1*b0
	return point{
		x: (b0*c1 - b1*c0) / det,
		y: (a1*c0 - a0*c1) / det,
	}
}

// AppendVerticesAndIndicesForFilling appends vertices and indices to fill this path and returns them.
// AppendVerticesAndIndicesForFilling works in a similar way to the built-in append function.
// If the arguments are nils, AppendVerticesAndIndicesForFilling returns new slices.
//
// The returned vertice's SrcX and SrcY are 0, and ColorR, ColorG, ColorB, and ColorA are 1.
//
// The returned values are intended to be passed to DrawTriangles or DrawTrianglesShader with the fill rule NonZero or EvenOdd
// in order to render a complex polygon like a concave polygon, a polygon with holes, or a self-intersecting polygon.
//
// The returned vertices and indices should be rendered with a solid (non-transparent) color with the default Blend (source-over).
// Otherwise, there is no guarantee about the rendering result.
func (p *Path) AppendVerticesAndIndicesForFilling(vertices []ebiten.Vertex, indices []uint16) ([]ebiten.Vertex, []uint16) {
	base := uint16(len(vertices))
	if p.subpath.pointCount() < 3 {
		return vertices, indices
	}
	for i, pt := range p.subpath.points {
		vertices = append(vertices, ebiten.Vertex{
			DstX:   pt.x,
			DstY:   pt.y,
			SrcX:   0,
			SrcY:   0,
			ColorR: 1,
			ColorG: 1,
			ColorB: 1,
			ColorA: 1,
		})
		if i < 2 {
			continue
		}
		indices = append(indices, base, base+uint16(i-1), base+uint16(i))
	}
	return vertices, indices
}

// LineCap represents the way in which how the ends of the stroke are rendered.
type LineCap int

// List of LineCap.
const (
	LineCapButt LineCap = iota
	LineCapSquare
)

// LineJoin represents the way in which how two segments are joined.
type LineJoin int

// List of LineJoin.
const (
	LineJoinMiter LineJoin = iota
	LineJoinBevel
)

// StrokeOptions is options to render a stroke.
type StrokeOptions struct {
	// Width is the stroke width in pixels.
	//
	// The default (zero) value is 0.
	Width float32

	// LineCap is the way in which how the ends of the stroke are rendered.
	// Line caps are not rendered when the subpath is marked as closed.
	//
	// The default (zero) value is LineCapButt.
	LineCap LineCap

	// LineJoin is the way in which how two segments are joined.
	//
	// The default (zero) value is LineJoiMiter.
	LineJoin LineJoin

	// MiterLimit is the miter limit for LineJoinMiter.
	// For details, see https://developer.mozilla.org/en-US/docs/Web/SVG/Attribute/stroke-miterlimit.
	//
	// The default (zero) value is 0.
	MiterLimit float32
}

// AppendVerticesAndIndicesForStroke appends vertices and indices to render a stroke of this path and returns them.
// AppendVerticesAndIndicesForStroke works in a similar way to the built-in append function.
// If the arguments are nils, AppendVerticesAndIndicesForStroke returns new slices.
//
// The returned vertice's SrcX and SrcY are 0, and ColorR, ColorG, ColorB, and ColorA are 1.
//
// The returned values are intended to be passed to DrawTriangles or DrawTrianglesShader with a solid (non-transparent) color
// with FillAll or NonZero fill rule, not EvenOdd fill rule.
func (p *Path) AppendVerticesAndIndicesForStroke(vertices []ebiten.Vertex, indices []uint16, op *StrokeOptions) ([]ebiten.Vertex, []uint16) {
	if p.subpath.pointCount() < 2 {
		return vertices, indices
	}

	var rects [][4]point
	for i := 0; i < p.subpath.pointCount()-1; i++ {
		pt := p.subpath.points[i]

		nextPt := p.subpath.points[i+1]
		dx := nextPt.x - pt.x
		dy := nextPt.y - pt.y
		dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))
		extX := (dy) * op.Width / 2 / dist
		extY := (-dx) * op.Width / 2 / dist

		rects = append(rects, [4]point{
			{
				x: pt.x + extX,
				y: pt.y + extY,
			},
			{
				x: nextPt.x + extX,
				y: nextPt.y + extY,
			},
			{
				x: pt.x - extX,
				y: pt.y - extY,
			},
			{
				x: nextPt.x - extX,
				y: nextPt.y - extY,
			},
		})
	}

	for i, rect := range rects {
		idx := uint16(len(vertices))
		for _, pt := range rect {
			vertices = append(vertices, ebiten.Vertex{
				DstX:   pt.x,
				DstY:   pt.y,
				SrcX:   0,
				SrcY:   0,
				ColorR: 1,
				ColorG: 1,
				ColorB: 1,
				ColorA: 1,
			})
		}
		// All the triangles are rendered in clockwise order to enable NonZero filling rule (#2833).
		indices = append(indices, idx, idx+1, idx+2, idx+1, idx+3, idx+2)

		// Add line joints.
		var nextRect [4]point
		if i < len(rects)-1 {
			nextRect = rects[i+1]
		} else {
			continue
		}

		// c is the center of the 'end' edge of the current rect (= the second point of the segment).
		c := point{
			x: (rect[1].x + rect[3].x) / 2,
			y: (rect[1].y + rect[3].y) / 2,
		}

		// Note that the Y direction and the angle direction are opposite from math's.
		a0 := float32(math.Atan2(float64(rect[1].y-c.y), float64(rect[1].x-c.x)))
		a1 := float32(math.Atan2(float64(nextRect[0].y-c.y), float64(nextRect[0].x-c.x)))
		da := a1 - a0
		for da < 0 {
			da += 2 * math.Pi
		}
		if da == 0 {
			continue
		}

		switch op.LineJoin {
		case LineJoinMiter:
			delta := math.Pi - da
			exceed := float32(math.Abs(1/math.Sin(float64(delta/2)))) > op.MiterLimit

			var quad Path
			quad.MoveTo(c.x, c.y)
			if da < math.Pi {
				quad.LineTo(rect[1].x, rect[1].y)
				if !exceed {
					pt := crossingPointForTwoLines(rect[0], rect[1], nextRect[0], nextRect[1])
					quad.LineTo(pt.x, pt.y)
				}
				quad.LineTo(nextRect[0].x, nextRect[0].y)
			} else {
				quad.LineTo(rect[3].x, rect[3].y)
				if !exceed {
					pt := crossingPointForTwoLines(rect[2], rect[3], nextRect[2], nextRect[3])
					quad.LineTo(pt.x, pt.y)
				}
				quad.LineTo(nextRect[2].x, nextRect[2].y)
			}
			vertices, indices = quad.AppendVerticesAndIndicesForFilling(vertices, indices)

		case LineJoinBevel:
			var tri Path
			tri.MoveTo(c.x, c.y)
			if da < math.Pi {
				tri.LineTo(rect[1].x, rect[1].y)
				tri.LineTo(nextRect[0].x, nextRect[0].y)
			} else {
				tri.LineTo(rect[3].x, rect[3].y)
				tri.LineTo(nextRect[2].x, nextRect[2].y)
			}
			vertices, indices = tri.AppendVerticesAndIndicesForFilling(vertices, indices)
		}
	}

	if len(rects) == 0 {
		return vertices, indices
	}

	// If the subpath is closed, do not render line caps.
	switch op.LineCap {
	case LineCapButt:
		// Do nothing.

	case LineCapSquare:
		startR, endR := rects[0], rects[len(rects)-1]
		{
			a := math.Atan2(float64(startR[0].y-startR[1].y), float64(startR[0].x-startR[1].x))
			s, c := math.Sincos(a)
			dx, dy := float32(c)*op.Width/2, float32(s)*op.Width/2

			var quad Path
			quad.MoveTo(startR[0].x, startR[0].y)
			quad.LineTo(startR[0].x+dx, startR[0].y+dy)
			quad.LineTo(startR[2].x+dx, startR[2].y+dy)
			quad.LineTo(startR[2].x, startR[2].y)
			vertices, indices = quad.AppendVerticesAndIndicesForFilling(vertices, indices)
		}
		{
			a := math.Atan2(float64(endR[1].y-endR[0].y), float64(endR[1].x-endR[0].x))
			s, c := math.Sincos(a)
			dx, dy := float32(c)*op.Width/2, float32(s)*op.Width/2

			var quad Path
			quad.MoveTo(endR[1].x, endR[1].y)
			quad.LineTo(endR[1].x+dx, endR[1].y+dy)
			quad.LineTo(endR[3].x+dx, endR[3].y+dy)
			quad.LineTo(endR[3].x, endR[3].y)
			vertices, indices = quad.AppendVerticesAndIndicesForFilling(vertices, indices)
		}
	}

	return vertices, indices
}
