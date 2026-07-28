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

package gui

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/term/graphemecluster"
	imagefont "golang.org/x/image/font"
	"unstable.build/go-tui/term/gui/drawrect"
	"unstable.build/go-tui/term/gui/drawtext"
	"unstable.build/go-tui/term/gui/font"
)

const (
	dimAlphaPerc    float32 = 0.7
	longestLigature int     = 2
)

var (
	// NOTE: if you change this, you must manually test that white/light themes
	// look good, that the initial shader looks good, and that when opacity
	// is 0.4 or less things look ok.
	defaultDrawTextOptions = ebiten.DrawImageOptions{
		Blend: ebiten.Blend{
			BlendFactorSourceRGB:      ebiten.BlendFactorOne,
			BlendFactorDestinationRGB: ebiten.BlendFactorOneMinusSourceAlpha,
			BlendOperationRGB:         ebiten.BlendOperationAdd,
			BlendOperationAlpha:       ebiten.BlendOperationMax,
		},
	}
	backgroundRuneDrawTextOptions = ebiten.DrawImageOptions{
		Blend: ebiten.Blend{
			BlendFactorSourceRGB:        ebiten.BlendFactorOne,
			BlendFactorSourceAlpha:      ebiten.BlendFactorOne,
			BlendFactorDestinationRGB:   ebiten.BlendFactorOneMinusSourceAlpha,
			BlendFactorDestinationAlpha: ebiten.BlendFactorOne,
			BlendOperationRGB:           ebiten.BlendOperationAdd,
			BlendOperationAlpha:         ebiten.BlendOperationAdd,
		},
	}
	frameToScreenOptions = ebiten.DrawImageOptions{
		Blend: ebiten.Blend{
			BlendFactorSourceRGB:        ebiten.BlendFactorOne,
			BlendFactorSourceAlpha:      ebiten.BlendFactorOne,
			BlendFactorDestinationRGB:   ebiten.BlendFactorZero,
			BlendFactorDestinationAlpha: ebiten.BlendFactorZero,
			BlendOperationRGB:           ebiten.BlendOperationAdd,
			BlendOperationAlpha:         ebiten.BlendOperationAdd,
		},
	}
)

var ligatures = map[string]rune{
	":=": '≔',
	"!=": '≠',
	"<=": '≤',
	">=": '≥',
	"=>": '⇒',
	"->": '→',
	"<-": '←',
	"<>": '≷',
}

type renderer struct {
	fontManager      *font.Manager
	drawer           *drawtext.Drawer
	font             fontFace
	bgOpacity        float64
	fgOpacity        float64
	bgColor          color.RGBA
	fgColor          color.RGBA
	frame            *ebiten.Image
	enableLigatures  bool
	cursorBackground color.RGBA
	cursorForeground color.RGBA

	bufPath     drawrect.Path
	bufVertices []ebiten.Vertex
	bufIndices  []uint16
}

type fontFace struct {
	Regular    imagefont.Face
	Bold       imagefont.Face
	Italic     imagefont.Face
	BoldItalic imagefont.Face
	CellSize   font.CharSize
	OffsetY    float64
}

func newFontFace(fontManager *font.Manager) fontFace {
	return fontFace{
		Regular:    fontManager.RegularFontFace(),
		Bold:       fontManager.BoldFontFace(),
		Italic:     fontManager.ItalicFontFace(),
		BoldItalic: fontManager.BoldItalicFontFace(),
		CellSize:   fontManager.CharSize(),
		OffsetY:    fontManager.OffsetY(),
	}
}

func newRenderer(
	width, height int, deviceScale float64,
	fontManager *font.Manager, bgOpacity, fgOpacity float64, enableLigatures bool,
	cursorAttributes, defaultAttr term.Attributes,
) *renderer {
	bgBlack := applyOpacity(0, 0, 0, bgOpacity)
	fgWhite := applyOpacity(255, 255, 255, fgOpacity)

	imageWidth, imageHeight := fontManager.ImageWidth(width), fontManager.ImageHeight(height)
	var cursorForeground, cursorBackground color.RGBA
	if cursorAttributes.Bg.Valid() {
		rr, g, b := cursorAttributes.Bg.TrueColor().RGB()
		cursorBackground = color.RGBA{R: uint8(rr), G: uint8(g), B: uint8(b), A: 255}
	} else {
		cursorBackground = applyOpacity(0, 0, 0, bgOpacity)
	}
	if cursorAttributes.Fg.Valid() {
		rr, g, b := cursorAttributes.Fg.TrueColor().RGB()
		cursorForeground = color.RGBA{R: uint8(rr), G: uint8(g), B: uint8(b), A: 255}
	} else {
		cursorForeground = applyOpacity(0, 0, 0, fgOpacity)
	}

	bgColor := tcellToColor(defaultAttr.Bg, bgBlack, bgOpacity)
	return &renderer{
		fontManager:      fontManager,
		drawer:           drawtext.New(),
		bgColor:          bgColor,
		frame:            ebiten.NewImage(imageWidth, imageHeight),
		fgColor:          tcellToColor(defaultAttr.Fg, fgWhite, fgOpacity),
		font:             newFontFace(fontManager),
		bgOpacity:        bgOpacity,
		fgOpacity:        fgOpacity,
		enableLigatures:  enableLigatures,
		cursorForeground: cursorForeground,
		cursorBackground: cursorBackground,
	}
}

func (r *renderer) Draw(
	screen *ebiten.Image, cells [][]term.Cell,
	drawCursor bool, cursorPos term.Coordinates,
	cursorStyle term.CursorStyle,
	offsetX, offsetY float64,
) {
	// fill default background so we can skip drawing individual
	// cells with default background.
	r.frame.Fill(r.bgColor)
	r.renderContent(r.frame, cells)
	if drawCursor {
		r.renderCursor(r.frame, cells, cursorPos, cursorStyle)
	}
	screen.DrawImage(r.frame, &frameToScreenOptions)
}

func (r *renderer) renderContent(screen *ebiten.Image, cells [][]term.Cell) {
	// draw base content for each row
	for viewY := len(cells) - 1; viewY >= 0; viewY-- {
		r.renderRow(screen, cells, viewY)
	}
}

func (r *renderer) renderRow(
	screen *ebiten.Image, cells [][]term.Cell, viewY int,
) {
	row := cells[viewY]
	pixelY := r.fontManager.PixelY(viewY)
	textPixelY := pixelY + r.font.OffsetY
	halfCell := math.Floor(r.font.CellSize.Y/2) - 2

	var useFace imagefont.Face
	useFace = r.font.Regular

	var temp color.RGBA
	var skipRunes int
	// draw text content of each cell in row
	for viewX := range len(row) {
		if skipRunes > 0 {
			skipRunes--
			continue
		}
		cell := row[viewX]
		isBackground := graphemecluster.IsBackground(cell.Ch)

		var fg color.RGBA
		if isBackground {
			fg = tcellToColor(cell.Fg, r.fgColor, r.bgOpacity)
		} else {
			fg = tcellToColor(cell.Fg, r.fgColor, r.fgOpacity)
		}
		bg := tcellToColor(cell.Bg, r.bgColor, r.bgOpacity)
		pixelX := r.fontManager.PixelX(viewX)
		pixelY := pixelY
		textPixelY := textPixelY
		verticalOffset := cell.Attrs&term.AttrVerticalRenderOffset != 0
		negativeVerticalOffset := cell.Attrs&term.AttrNegativeVerticalRenderOffset != 0
		if verticalOffset {
			pixelY = math.Floor(pixelY + halfCell)
			textPixelY = math.Floor(textPixelY + halfCell)
		} else if negativeVerticalOffset {
			pixelY = max(0, math.Ceil(pixelY-halfCell-1))
			textPixelY = max(0, math.Ceil(textPixelY-halfCell-1))
		}

		// reverse attr if AttrReverse
		if cell.Attrs&term.AttrReverse != 0 {
			temp = fg
			fg = bg
			bg = temp
		}

		// we don't need to draw empty cells, just draw background
		if cell.Ch == 0 || cell.Ch == '\t' {
			// do not draw default background as a rect, since it's already
			// been instructed via frame.Fill above.
			if bg == r.bgColor {
				continue
			}
			r.bufVertices, r.bufIndices = drawrect.DrawRect(
				&r.bufPath, r.bufVertices, r.bufIndices,
				screen, float32(pixelX), float32(pixelY),
				float32(r.font.CellSize.X), float32(r.font.CellSize.Y), bg)
			continue
		}

		isBold := cell.Attrs&term.AttrBold != 0
		isItalic := cell.Attrs&term.AttrItalic != 0

		drawTextOptions := defaultDrawTextOptions
		if isBackground {
			drawTextOptions = backgroundRuneDrawTextOptions
		}
		drawTextOptions.GeoM.Translate(pixelX, textPixelY)

		// pick a font face for the cell
		if !isBold && !isItalic {
			useFace = r.font.Regular
		} else if isBold && isItalic {
			useFace = r.font.BoldItalic
		} else if isBold {
			useFace = r.font.Bold
		} else if isItalic {
			useFace = r.font.Italic
		}
		cr, cg, cb, ca := fg.RGBA()
		drawTextOptions.ColorScale.Scale(
			float32(cr)/0xffff,
			float32(cg)/0xffff,
			float32(cb)/0xffff,
			float32(ca)/0xffff,
		)

		// dim fg text if AttrDim
		if cell.Attrs&term.AttrDim != 0 {
			drawTextOptions.ColorScale.ScaleAlpha(dimAlphaPerc)
		}

		if cell.Attrs&term.AttrUnderline != 0 {
			underlinePixelY := pixelY + r.font.CellSize.Y - 1
			r.bufVertices, r.bufIndices = drawrect.DrawStroke(&r.bufPath, r.bufVertices, r.bufIndices,
				screen, float32(pixelX), float32(underlinePixelY),
				float32(pixelX+r.font.CellSize.X),
				float32(underlinePixelY), 2, fg)
		}

		if r.enableLigatures && skipRunes == 0 {
			skipRunes = r.handleLigatures(screen, cells, viewX, viewY, useFace, fg)
		}

		if skipRunes > 0 {
			skipRunes--
			continue
		}

		cellWidth := math.Max(1, float64(cell.Width))
		// do not draw default background as a rect, since it's already
		// been instructed via frame.Fill above.
		if bg != r.bgColor {
			r.bufVertices, r.bufIndices = drawrect.DrawRect(
				&r.bufPath, r.bufVertices, r.bufIndices,
				screen, float32(pixelX), float32(pixelY),
				float32(r.font.CellSize.X*cellWidth), float32(r.font.CellSize.Y), bg)
		}

		// draw text
		r.drawer.DrawWithOptions(screen, cell.Ch, cell.CombiningRunes(), useFace, &drawTextOptions)
		if cell.Width > 1 {
			skipRunes += int(cell.Width) - 1
		}
	}
}

func (r *renderer) handleLigatures(
	screen *ebiten.Image, cells [][]term.Cell, sx, sy int,
	face imagefont.Face, color color.RGBA,
) (length int) {
	return handleLigatures(r.drawer, cells, sx, sy, face, color, r.font, screen)
}

func (r *renderer) renderCursor(
	screen *ebiten.Image, cells [][]term.Cell,
	pos term.Coordinates, style term.CursorStyle,
) {
	cell := r.getCell(cells, pos)
	width := math.Max(1, float64(cell.Width))

	useFace := r.font.Regular
	isBold := cell.Attrs&term.AttrBold != 0
	isItalic := cell.Attrs&term.AttrItalic != 0
	if isBold && isItalic {
		useFace = r.font.BoldItalic
	} else if isBold {
		useFace = r.font.Bold
	} else if isItalic {
		useFace = r.font.Italic
	}

	pixelX := r.fontManager.PixelX(pos.X)
	pixelY := r.fontManager.PixelY(pos.Y)
	textPixelY := pixelY + r.font.OffsetY
	pixelW, pixelH := r.font.CellSize.X*width, r.font.CellSize.Y

	// empty rect without focus
	if !ebiten.IsFocused() {
		r.bufVertices, r.bufIndices = drawrect.DrawRect(
			&r.bufPath, r.bufVertices, r.bufIndices,
			screen, float32(pixelX), float32(pixelY),
			float32(pixelW), float32(pixelH), r.cursorBackground)
		r.bufVertices, r.bufIndices = drawrect.DrawRect(
			&r.bufPath, r.bufVertices, r.bufIndices,
			screen, float32(pixelX+1), float32(pixelY+1),
			float32(pixelW-2), float32(pixelH-2), r.cursorForeground)
		return
	}

	// draw the cursor shape
	switch style {
	case term.CursorStyleBlinkingBar, term.CursorStyleSteadyBar:
		r.bufVertices, r.bufIndices = drawrect.DrawRect(
			&r.bufPath, r.bufVertices, r.bufIndices,
			screen, float32(pixelX), float32(pixelY), 2,
			float32(pixelH), r.cursorBackground)
	case term.CursorStyleBlinkingUnderline, term.CursorStyleSteadyUnderline:
		r.bufVertices, r.bufIndices = drawrect.DrawRect(
			&r.bufPath, r.bufVertices, r.bufIndices,
			screen, float32(pixelX), float32(pixelY+pixelH-2),
			float32(pixelW), 2, r.cursorBackground)
	default:
		r.bufVertices, r.bufIndices = drawrect.DrawRect(
			&r.bufPath, r.bufVertices, r.bufIndices,
			screen, float32(pixelX), float32(pixelY),
			float32(pixelW), float32(pixelH), r.cursorBackground)
		if cell.Ch != 0 {
			var opts ebiten.DrawImageOptions
			opts.GeoM.Translate(pixelX, textPixelY)
			cr, cg, cb, ca := r.cursorForeground.RGBA()
			opts.ColorScale.Scale(
				float32(cr)/0xffff,
				float32(cg)/0xffff,
				float32(cb)/0xffff,
				float32(ca)/0xffff,
			)
			r.drawer.DrawWithOptions(screen, cell.Ch, cell.CombiningRunes(), useFace, &opts)
		}
	}
}

func (r *renderer) getCell(cells [][]term.Cell, pos term.Coordinates) (ret term.Cell) {
	if pos.Y >= len(cells) || pos.X >= len(cells[pos.Y]) {
		return
	}
	return cells[pos.Y][pos.X]
}

func tcellToColor(tcolor term.Color, def color.RGBA, opacity float64) color.RGBA {
	if !tcolor.Valid() || tcolor == term.ColorDefault {
		return def
	}
	r, g, b := tcolor.TrueColor().RGB()
	return applyOpacity(r, g, b, opacity)
}

func handleLigatures(
	drawer *drawtext.Drawer, cells [][]term.Cell, sx, sy int, face imagefont.Face, color color.RGBA,
	font fontFace, frame *ebiten.Image,
) (length int) {
	var c [longestLigature]rune
	candidate := c[:0]
	for i := 0; i < longestLigature; i++ {
		x := sx + i
		if sy >= len(cells) || x >= len(cells[sy]) || cells[sy][x].Ch == 0 {
			break
		}
		candidate = append(candidate, cells[sy][x].Ch)
	}

	for len(candidate) > 1 {
		if ru, ok := ligatures[string(candidate)]; ok {
			// draw ligature
			ligX := (float64(sx) * font.CellSize.X) + ((float64(len(candidate)-1) * font.CellSize.X) / 2)
			ligY := float64(sy)*font.CellSize.Y + font.OffsetY
			var opts ebiten.DrawImageOptions
			opts.GeoM.Translate(ligX, ligY)
			cr, cg, cb, ca := color.RGBA()
			opts.ColorScale.Scale(
				float32(cr)/0xffff,
				float32(cg)/0xffff,
				float32(cb)/0xffff,
				float32(ca)/0xffff,
			)
			drawer.DrawWithOptions(frame, ru, nil, face, &opts)
			return len(candidate)
		}
		candidate = candidate[:len(candidate)-1]
	}

	return 0
}

func applyOpacity(r, g, b int32, opacity float64) color.RGBA {
	alpha := uint8(float64(255) * opacity)
	return color.RGBA{
		R: uint8(float64(r) * opacity),
		G: uint8(float64(g) * opacity),
		B: uint8(float64(b) * opacity),
		A: alpha,
	}
}
