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

package font

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/blue/logging"
	"go.uber.org/multierr"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	"unstable.build/go-tui/api/config"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/term/gui/font/builtinfont"
	"unstable.build/go-tui/workspace"
)

// Manager manages the underlying font.Face used to render
// characters on screen. It also exposes methods to help calculate
// the width and height in cells and pixels of the given screen.
//
// If SetFontByFamilyName is not called, a builtin font is used.
type Manager struct {
	findfont     findFont
	brailleFont  *sfnt.Font
	fallbackFont *sfnt.Font
	// acts as an IR to have all fonts preloaded upon
	// size, DPI and device scale changes.
	preloaded      []*sfnt.Font
	family         string
	regularFace    font.Face
	boldFace       font.Face
	italicFace     font.Face
	boldItalicFace font.Face
	size           float64
	staticDPI      float64
	staticDevScale float64
	charSize       CharSize
	offset         fixed.Point26_6
	cellOffsetY    float64
}

// CharSize represent a character dimensions in pixels.
type CharSize struct {
	X float64
	Y float64
}

// NewManager allocates storage for a new Manager and initializes it
// with the default font and dpi.
func NewManager() (*Manager, error) {
	ret := &Manager{
		size: 16,
	}
	cwdURI, _ := workspaceapi.CurrentUserHostURI(".")
	fs, err := workspace.NewFileScheme(context.Background(), config.NopConfig(), cwdURI)
	if err != nil {
		return nil, fmt.Errorf("new file scheme: %v", err)
	}
	ret.findfont = systemFindFont{reader: fs}
	ret.brailleFont, err = opentype.Parse(builtinfont.BrailleTTF)
	if err != nil {
		return nil, fmt.Errorf("parse braille font: %w", err)
	}
	ret.fallbackFont, err = opentype.Parse(builtinfont.FallbackTTF)
	if err != nil {
		return nil, fmt.Errorf("parse braille font: %w", err)
	}
	return ret, nil
}

// IncreaseSize increases the size of the font by 1.
func (m *Manager) IncreaseSize() error {
	return m.SetSize(m.size + 1)
}

// DecreaseSize decreases the size of the font by 1.
func (m *Manager) DecreaseSize() error {
	if m.size <= 1 {
		return nil
	}
	return m.SetSize(m.size - 1)
}

// DPI returns the configured DPI for the underlying font.
func (m *Manager) DPI() float64 {
	return m.dpi()
}

func (m *Manager) dpi() float64 {
	if m.staticDPI != 0 {
		return m.staticDPI
	}
	return 72.0 * m.DeviceScale()
}

// SetDPI sets the DPI of the configured font. It will
// reload the font with the new DPI and return
// an error if there was a problem reloading font.
//
// Note that after this method is called, the DPI
// won't be adjusted dynamically. This is only meant to be run
// on systems where the device scale detector is not working properly.
func (m *Manager) SetDPI(dpi float64) error {
	if dpi < 0 {
		panic(errors.New("DPI must be >0"))
	}
	if m.staticDPI == dpi {
		return nil
	}
	m.staticDPI = dpi
	return m.ReloadFont()
}

// SetSize sets the size of the configured font.
// It will reload the font with the new DPI and return
// an error if there was a problem reloading the font.
func (m *Manager) SetSize(size float64) error {
	if m.size == size {
		return nil
	}
	m.size = size
	return m.ReloadFont()
}

// SetOffset sets the x and y offset of the configured font.
// It will reload the font with the new DPI and return
// an error if there was a problem reloading the font.
func (m *Manager) SetOffset(x, y float64) error {
	fixedX := float64ToFixed(x)
	fixedY := float64ToFixed(y)
	if m.offset.Y == fixedY && m.offset.X == fixedX {
		return nil
	}
	m.offset.X = fixedX
	m.offset.Y = fixedY
	return m.ReloadFont()
}

// IncreaseLineHeight increases the line height of the font by 1 pixel.
func (m *Manager) IncreaseLineHeight() error {
	return m.SetOffset(fixedToFloat64(m.offset.X), fixedToFloat64(m.offset.Y)+1)
}

// DecreaseLineHeight decreases the line height of the font by 1 pixel.
func (m *Manager) DecreaseLineHeight() error {
	return m.SetOffset(fixedToFloat64(m.offset.X), fixedToFloat64(m.offset.Y)-1)
}

// ReloadFont reloads the font. This can be used
// if a change in DeviceScale is detected to re-adjust
// calcultions and font rendering for the new device scale.
func (m *Manager) ReloadFont() error {
	if len(m.preloaded) == 0 {
		return m.loadFallbackFont()
	}
	if err := m.setPreloaded(m.preloaded); err != nil {
		return fmt.Errorf("reload font: %w", err)
	}
	return nil
}

// SetFontByFamilyName finds the given font installed on the system and
// sets it as the configured font, or returns an error if there was
// a problem loading the given font.
// If name is set to an empty string, the default builtin font is used.
func (m *Manager) SetFontByFamilyName(name string) error {
	if name == m.family {
		return nil
	}
	m.resetFonts()
	if name == "" {
		return m.loadFallbackFont()
	}

	fonts, err := m.findAndLoadFont(name)
	if err == nil {
		m.preloaded = fonts
		m.family = name
	}
	return err
}

// SetDeviceScale forces the device scale to the given value.
//
// Note that after this method is called, the device scale
// won't be adjusted dynamically. This is only meant to be run
// on systems where the device scale detector is not working properly.
func (m *Manager) SetDeviceScale(value float64) {
	m.staticDevScale = value
}

// DeviceScale returns the device scale factor of the current screen.
func (m *Manager) DeviceScale() float64 {
	if m.staticDevScale != 0 {
		return m.staticDevScale
	}
	// this cannot be cached otherwise moving window across screens with
	// different DPIs wouldn't adjust the device scale factor.
	return deviceScale()
}

// CharSize returns the character dimensions in pixels
// of the configured font.
func (m *Manager) CharSize() CharSize {
	m.ensureFontLoaded()
	return m.charSize
}

// OffsetY returns the y-offset for rendering a grid of character cells.
func (m *Manager) OffsetY() float64 {
	m.ensureFontLoaded()
	return m.cellOffsetY
}

// CellsWidth returns the total width in cells of the current screen.
func (m *Manager) CellsWidth(width int) int {
	m.ensureFontLoaded()
	return int(math.Max(1, math.Floor(m.cellsWidth(width))))
}

// CellsHeight returns the total height in cells of the current screen.
func (m *Manager) CellsHeight(height int) int {
	m.ensureFontLoaded()
	return int(math.Max(1, math.Floor(m.cellsHeight(height))))
}

// ImageWidth returns the total width in pixels of the current screen.
func (m *Manager) ImageWidth(width int) int {
	m.ensureFontLoaded()
	return int(math.Max(1, math.Floor(m.cellsWidth(width)*m.CharSize().X)))
}

// ImageHeight returns the total height in pixels of the current screen.
func (m *Manager) ImageHeight(height int) int {
	m.ensureFontLoaded()
	return int(math.Max(1, math.Floor(m.cellsHeight(height)*m.CharSize().Y)))
}

// RegularFontFace returns the configured regular font.Face.
func (m *Manager) RegularFontFace() font.Face {
	m.ensureFontLoaded()
	return m.regularFace
}

// BoldFontFace returns the configured bold font.Face or the fallback
// if no bold font face was found when loading the font.
func (m *Manager) BoldFontFace() font.Face {
	if m.boldFace == nil {
		return m.RegularFontFace()
	}
	return m.boldFace
}

// ItalicFontFace returns the configured italic font.Face or the fallback
// if no italic font face was found when loading the font.
func (m *Manager) ItalicFontFace() font.Face {
	if m.italicFace == nil {
		return m.RegularFontFace()
	}
	return m.italicFace
}

// BoldItalicFontFace returns the configured bold and italic font.Face or the fallback
// if no bold and italic font face was found when loading the font.
func (m *Manager) BoldItalicFontFace() font.Face {
	if m.boldItalicFace == nil {
		if m.boldFace == nil {
			return m.ItalicFontFace()
		}
		return m.BoldFontFace()
	}
	return m.boldItalicFace
}

// AvailableFontFamilies returns a list of available font families, by family name.
func (m *Manager) AvailableFontFamilies() (iterator.Iterator[string], error) {
	fonts, err := m.findfont.list()
	if err != nil {
		return nil, fmt.Errorf("list fonts: %w", err)
	}
	seen := make(map[string]struct{})
	return iterator.Filter(iterator.Map[metadata, string](fonts, func(f metadata) string {
		return f.family
	}), func(family string) bool {
		_, ok := seen[family]
		if ok {
			return false
		}
		seen[family] = struct{}{}
		return true
	}), nil
}

func (m *Manager) ensureFontLoaded() {
	if m.regularFace == nil {
		err := m.loadFallbackFont()
		if err != nil {
			panic(fmt.Sprintf("could not load the default fonts: %v", err))
		}
	}
}

func (m *Manager) cellsWidth(width int) float64 {
	return float64(width) * m.DeviceScale() / m.charSize.X
}

func (m *Manager) cellsHeight(height int) float64 {
	return float64(height) * m.DeviceScale() / m.charSize.Y
}

func (m *Manager) loadFallbackFont() error {
	regular, err := opentype.Parse(builtinfont.RegularTTF)
	if err != nil {
		return err
	}
	m.regularFace, err = m.createFace(regular, false)
	if err != nil {
		return err
	}

	bold, err := opentype.Parse(builtinfont.BoldTTF)
	if err != nil {
		return err
	}
	m.boldFace, err = m.createFace(bold, true)
	if err != nil {
		return err
	}

	italic, err := opentype.Parse(builtinfont.ItalicTTF)
	if err != nil {
		return err
	}
	m.italicFace, err = m.createFace(italic, false)
	if err != nil {
		return err
	}

	boldItalic, err := opentype.Parse(builtinfont.BoldItalicTTF)
	if err != nil {
		return err
	}
	m.boldItalicFace, err = m.createFace(boldItalic, true)
	if err != nil {
		return err
	}

	return m.setFaceMetrics()
}

func (m *Manager) loadFontAtPath(path string) (fonts []*sfnt.Font, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", path, err)
	}

	switch filepath.Ext(path) {
	case ".ttc", ".otc":
		col, err := opentype.ParseCollectionReaderAt(f)
		if err != nil {
			return nil, fmt.Errorf("opentype parse collection: %w", err)
		}
		for i := 0; i < col.NumFonts(); i++ {
			font, err := col.Font(i)
			if err != nil {
				return nil, fmt.Errorf("font %d: %w", i, err)
			}
			fonts = append(fonts, font)
		}
	case ".ttf", ".otf":
		font, serr := opentype.ParseReaderAt(f)
		if serr != nil {
			return nil, multierr.Append(err, fmt.Errorf("opentype parse font: %w", serr))
		}
		fonts = append(fonts, font)
	}
	if err != nil {
		return nil, err
	}

	var buf sfnt.Buffer
	for _, font := range fonts {
		if lerr := m.setFont(&buf, font); lerr != nil {
			return nil, multierr.Append(err, lerr)
		}
	}
	return
}

func (m *Manager) setFont(buf *sfnt.Buffer, font *sfnt.Font) error {
	subfamily, err := font.Name(buf, sfnt.NameIDSubfamily)
	if err != nil {
		return fmt.Errorf("read font subfamily: %w", err)
	}
	switch subfamily {
	case "Regular":
		face, err := m.createFace(font, false)
		if err != nil {
			return fmt.Errorf("create opentype face: %w", err)
		}
		m.regularFace = face
	case "Bold":
		face, err := m.createFace(font, true)
		if err != nil {
			return fmt.Errorf("create opentype face: %w", err)
		}
		m.boldFace = face
	case "Italic", "Oblique":
		face, err := m.createFace(font, false)
		if err != nil {
			return fmt.Errorf("create opentype face: %w", err)
		}
		m.italicFace = face
	case "Bold Italic", "Bold Oblique":
		face, err := m.createFace(font, true)
		if err != nil {
			return fmt.Errorf("create opentype face: %w", err)
		}
		m.boldItalicFace = face
	default:
		m.log(log.DebugLevel, "skipping subfamily: %q", subfamily)
	}
	return nil
}

func (m *Manager) resetFonts() {
	m.regularFace = nil
	m.boldFace = nil
	m.italicFace = nil
	m.boldItalicFace = nil
}

func (m *Manager) setPreloaded(preloaded []*sfnt.Font) (ret error) {
	m.resetFonts()
	var buf sfnt.Buffer
	for _, font := range preloaded {
		if err := m.setFont(&buf, font); err != nil {
			ret = multierr.Append(ret, err)
		}
	}
	if ret != nil {
		return
	}
	if err := m.setFaceMetrics(); err != nil {
		ret = multierr.Append(ret, err)
	}
	return
}

func (m *Manager) findAndLoadFont(name string) (ret []*sfnt.Font, err error) {
	fonts, err := m.findfont.findByFamily(name)
	if err != nil {
		return nil, fmt.Errorf("find font with family '%s': %w", name, err)
	}

	defer fonts.Close()
	for {
		meta, ok := fonts.Next()
		if !ok {
			if err := fonts.Err(); err != nil {
				return nil, fmt.Errorf("fonts iterator: %v", err)
			}
			break
		}
		fonts, err := m.loadFontAtPath(meta.path)
		if err != nil {
			return nil, fmt.Errorf("load font at path '%s': %w", meta.path, err)
		}
		ret = append(ret, fonts...)
	}

	if m.regularFace == nil {
		return nil, fmt.Errorf("could not find regular style for font family '%s'", name)
	}

	err = m.setFaceMetrics()
	return
}

func (m *Manager) createFace(f *sfnt.Font, bold bool) (font.Face, error) {
	face, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size:    m.size,
		DPI:     m.dpi(),
		Hinting: font.HintingNone,
	})
	if err != nil {
		return nil, fmt.Errorf("opentype new face: %w", err)
	}
	brailleFace, err := opentype.NewFace(m.brailleFont, &opentype.FaceOptions{
		Size:    m.size,
		DPI:     m.dpi(),
		Hinting: font.HintingNone,
	})
	if err != nil {
		return nil, fmt.Errorf("opentype new braille face: %w", err)
	}
	fallbackFace, err := opentype.NewFace(m.fallbackFont, &opentype.FaceOptions{
		Size:    m.size,
		DPI:     m.dpi(),
		Hinting: font.HintingNone,
	})
	if err != nil {
		return nil, fmt.Errorf("opentype new fallback face: %w", err)
	}
	charSizeX, charSizeY, offsetY := m.calcFaceMetrics(face)
	customFace := newCustomFace(charSizeX, charSizeY, offsetY, face, bold)
	face = newMultiFace(1, customFace, face, brailleFace, fallbackFace)
	face = newCacheFace(face)
	return face, nil
}

func (m *Manager) setFaceMetrics() error {
	// use user/system face for calculating metrics, rather than
	// auxiliary or fallback faces.
	multi := m.regularFace.(*cacheFace).f.(*multi)
	faceForMetrics := multi.faces[multi.preferred]

	m.charSize.X, m.charSize.Y, m.cellOffsetY = m.calcFaceMetrics(faceForMetrics)
	m.log(log.DebugLevel, "calculated font char size: %+v and offset: %f",
		m.charSize, m.cellOffsetY)
	return nil
}

func (m *Manager) calcFaceMetrics(face font.Face) (float64, float64, float64) {
	bounds, advance, _ := face.GlyphBounds('█')

	charSizeX := math.Max(0, float64((advance-bounds.Min.X+m.offset.X)/(1<<6)))
	charSizeY := math.Max(0, float64((bounds.Max.Sub(bounds.Min).Y+m.offset.Y)/(1<<6)))
	cellOffsetY := float64(-bounds.Min.Y / (1 << 6))

	return charSizeX, charSizeY, cellOffsetY
}

func (p *Manager) log(level log.Level, msg string, args ...any) {
	if !log.IsLevelEnabled(level) {
		return
	}
	log.WithFields(log.Fields{
		logging.KeyClass: "font.Manager",
	}).Logf(level, msg, args...)
}
