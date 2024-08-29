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

package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/unstablebuild/tcell/v3"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/term"
)

// ErrNotFound is returned when config property is not in config.
var (
	ErrNotFound              = errors.New("key not found")
	ErrInvalidType           = errors.New("type is invalid")
	ErrInvalidAttributeValue = errors.New("attribute value is invalid")
)

// Config is the interface implemented by a configuration provider.
type Config interface {
	GetInt(string) (int, error)
	GetFloat(string) (float64, error)
	GetString(string) (string, error)
	GetBool(string) (bool, error)
	GetConfig(string) (Config, error)
	GetMap(string) (map[string]interface{}, error)
	GetAttribute(string) (tcell.AttrMask, error)
	GetColor(string) (tcell.Color, error)
	GetRune(string) (rune, error)
	GetSlice(string) ([]interface{}, error)
	Iterate(fn func(k string, value interface{}))
}

// JSON satisfies Config, encoding.TextMarshaler
// and encoding.TextUnmarshaler.
type JSON struct {
	mapConfig
}

type mapConfig map[string]interface{}

// JSONFromMap returns a JSON Config from the given map of key
// value pairs.
func JSONFromMap(m map[string]interface{}) JSON {
	return JSON{mapConfig: MapConfig(m).(mapConfig)}
}

// JSONFromConfig returns a JSON Config from the given Config.
func JSONFromConfig(c Config) JSON {
	m := make(map[string]interface{})
	c.Iterate(func(k string, v interface{}) {
		m[k] = v
	})
	return JSONFromMap(m)
}

// MapConfig returns a Config that wraps m.
func MapConfig(m map[string]interface{}) Config {
	if m == nil {
		panic("invalid argument: map cannot be nil")
	}
	return mapConfig(m)
}

func clone(m map[string]interface{}) map[string]interface{} {
	ret := make(map[string]interface{})
	for k, v := range m {
		if ifc, ok := v.(map[string]interface{}); ok {
			ret[k] = clone(ifc)
		} else {
			ret[k] = v
		}
	}
	return ret
}

func cloneSlice(s []interface{}) []interface{} {
	ret := make([]interface{}, len(s))
	for i, v := range s {
		if vslice, ok := v.([]interface{}); ok {
			ret[i] = cloneSlice(vslice)
		} else {
			ret[i] = v
		}
	}
	return ret
}

// Clone returns a deep clone of the given config.
func Clone(cfg Config) Config {
	ret := make(map[string]interface{})
	cfg.Iterate(func(k string, v interface{}) {
		if vmap, ok := v.(map[string]interface{}); ok {
			ret[k] = clone(vmap)
		} else if vslice, ok := v.([]interface{}); ok {
			ret[k] = cloneSlice(vslice)
		} else {
			ret[k] = v
		}
	})
	return MapConfig(ret)
}

// GetAttributes is a helper which extracts and parses a term.Attributes as a map
// of fg, bg string keys to an attribute. See GetAttribute for more details.
func GetAttributes(c Config, key string) (term.Attributes, error) {
	cfg, err := c.GetConfig(key)
	if err != nil {
		return term.Attributes{}, err
	}

	var attr term.Attributes
	attr.Attrs, err = cfg.GetAttribute("flag")
	if err != nil && err != ErrNotFound {
		return term.Attributes{}, fmt.Errorf("get 'flag': %w", err)
	} else if err == ErrNotFound {
		// try with 'flags' too
		attr.Attrs, err = cfg.GetAttribute("flags")
		if err != nil && err != ErrNotFound {
			return term.Attributes{}, fmt.Errorf("get 'flags': %w", err)
		}
	}

	if fg, err := cfg.GetColor("fg"); err == nil {
		attr.Fg = fg
	} else if err != ErrNotFound && err != nil {
		return term.Attributes{}, fmt.Errorf("get 'fg' color: %w", err)
	}

	if bg, err := cfg.GetColor("bg"); err == nil {
		attr.Bg = bg
	} else if err != ErrNotFound && err != nil {
		return term.Attributes{}, fmt.Errorf("get 'bg' color: %w", err)
	}

	return attr, nil
}

// GetDuration is a helper which extracts and parses a time.Duration as a string
// from a Config.
func GetDuration(
	pconfig Config, key string, def time.Duration,
) (time.Duration, error) {
	durStr, err := pconfig.GetString(key)
	if err != nil {
		if err != ErrNotFound {
			err = fmt.Errorf("Error getting '%s' from config: %w", key, err)
			return 0, err
		}
		return def, nil
	}

	duration, err := time.ParseDuration(durStr)
	if err != nil {
		err = fmt.Errorf("Error parsing duration '%s' from config: %w", key, err)
		return 0, err
	}

	return duration, nil
}

// GetMapInt returns pconfig 'key' value as a map of string to integers.
func GetMapInt(pconfig Config, key string) (map[string]int, error) {
	mapOfIfc, err := pconfig.GetMap(key)
	if err != nil {
		if err != ErrNotFound {
			err = fmt.Errorf("get '%s' from config: %w. ", key, err)
		}
		return nil, err
	}
	ret := make(map[string]int, len(mapOfIfc))
	cfg := mapConfig(mapOfIfc)
	for k := range mapOfIfc {
		ret[k], err = cfg.GetInt(k)
		if err != nil { // ErrNotFound is not possible
			return nil, fmt.Errorf("expected %s to be map of string to integer: %w", key, err)
		}
	}
	return ret, nil
}

// GetFrameCharset is a helper which extracts and parses a component.FrameCharSet.
func GetFrameCharset(c Config, key string, def component.FrameCharSet) (
	component.FrameCharSet, error,
) {
	cfg, err := c.GetConfig(key)
	if err != nil {
		return def, err
	}

	cs := def
	r, err := cfg.GetRune("topleft")
	if err == nil {
		cs.TopLeft = r
	}
	r, err = cfg.GetRune("topright")
	if err == nil {
		cs.TopRight = r
	}
	r, err = cfg.GetRune("bottomleft")
	if err == nil {
		cs.BottomLeft = r
	}
	r, err = cfg.GetRune("bottomright")
	if err == nil {
		cs.BottomRight = r
	}
	r, err = cfg.GetRune("horizontaltop")
	if err == nil {
		cs.HorizontalTop = r
	}
	r, err = cfg.GetRune("horizontalbottom")
	if err == nil {
		cs.HorizontalBottom = r
	}
	r, err = cfg.GetRune("verticalleft")
	if err == nil {
		cs.VerticalLeft = r
	}
	r, err = cfg.GetRune("verticalright")
	if err == nil {
		cs.VerticalRight = r
	}
	return cs, nil
}

// GetKey is a helper which extracts and parses a term.KeyComb as a string
// from a Config.
func GetKey(c Config, key string) (term.KeyComb, error) {
	s, err := c.GetString(key)
	if err != nil {
		return term.KeyComb{}, err
	}
	return term.ParseKey(s)
}

// MarshalText satisfies encoding.TextMarshaler.
func (c *JSON) MarshalText() ([]byte, error) {
	text, err := json.Marshal(c.mapConfig)
	return text, err
}

// UnmarshalText satisfies encoding.TextUnmarshaler.
func (c *JSON) UnmarshalText(text []byte) error {
	c.mapConfig = make(map[string]interface{})
	err := json.Unmarshal(text, &c.mapConfig)
	return err
}

func (c mapConfig) GetInt(key string) (int, error) {
	v, ok := c[key]
	if !ok {
		return 0, ErrNotFound
	}

	switch vt := v.(type) {
	case int:
		return vt, nil
	case float64:
		return int(vt), nil
	case float32:
		return int(vt), nil
	case int64:
		return int(vt), nil
	case int32:
		return int(vt), nil
	default:
		return 0, ErrInvalidType
	}
}

func (c mapConfig) GetFloat(key string) (float64, error) {
	v, ok := c[key]
	if !ok {
		return 0, ErrNotFound
	}

	switch vt := v.(type) {
	case float64:
		return vt, nil
	case int:
		return float64(vt), nil
	case float32:
		return float64(vt), nil
	case int64:
		return float64(vt), nil
	case int32:
		return float64(vt), nil
	default:
		return 0, ErrInvalidType
	}
}

func (c mapConfig) GetString(key string) (string, error) {
	v, ok := c[key]
	if !ok {
		return "", ErrNotFound
	}
	switch vt := v.(type) {
	case string:
		return vt, nil
	case []byte:
		return string(vt), nil
	case []rune:
		return string(vt), nil
	case rune:
		return string(vt), nil
	case byte:
		return string(vt), nil
	default:
		return "", ErrInvalidType
	}
}

func (c mapConfig) GetBool(key string) (bool, error) {
	v, ok := c[key]
	if !ok {
		return false, ErrNotFound
	}
	vt, ok := v.(bool)
	if !ok {
		return false, ErrInvalidType
	}
	return vt, nil
}

func (c mapConfig) GetConfig(key string) (Config, error) {
	m, err := c.GetMap(key)
	if err != nil {
		return nil, err
	}
	return mapConfig(m), nil
}

func (c mapConfig) GetMap(key string) (map[string]interface{}, error) {
	v, ok := c[key]
	if !ok {
		return nil, ErrNotFound
	}
	vt, ok := v.(map[string]interface{})
	if !ok {
		return nil, ErrInvalidType
	}

	return vt, nil
}

func (c mapConfig) GetSlice(key string) ([]interface{}, error) {
	v, ok := c[key]
	if !ok {
		return nil, ErrNotFound
	}
	vt, ok := v.([]interface{})
	if !ok {
		return nil, ErrInvalidType
	}

	return vt, nil
}

func strToAttr(str string) (attr tcell.AttrMask, err error) {
	switch str {
	case "bold":
		attr = tcell.AttrBold
	case "underline":
		attr = tcell.AttrUnderline
	case "reverse":
		attr = tcell.AttrReverse
	case "default":
		attr = tcell.AttrNone
	case "none":
		attr = tcell.AttrNone
	case "blink":
		attr = tcell.AttrBlink
	case "dim":
		attr = tcell.AttrDim
	case "italic":
		attr = tcell.AttrItalic
	case "strikethrough":
		attr = tcell.AttrStrikeThrough
	default:
		err = ErrInvalidAttributeValue
	}
	return
}

func (c mapConfig) GetAttribute(key string) (tcell.AttrMask, error) {
	v, ok := c[key]
	if !ok {
		return 0, ErrNotFound
	}
	vt, ok := v.(string)
	if ok {
		return strToAttr(vt)
	}
	i, err := c.GetInt(key)
	if err == nil {
		return tcell.AttrMask(i), nil
	}

	avt, ok := v.([]interface{})
	if !ok {
		return 0, ErrInvalidType
	}

	var attr tcell.AttrMask
	for _, vtv := range avt {
		if vtvs, ok := vtv.(string); ok {
			vtvattr, err := strToAttr(vtvs)
			if err != nil {
				return 0, err
			}
			attr |= vtvattr
		}
	}

	return attr, nil
}

func (c mapConfig) GetColor(key string) (tcell.Color, error) {
	v, ok := c[key]
	if !ok {
		return 0, ErrNotFound
	}
	vs, ok := v.(string)
	if ok {
		if vs == "default" {
			return tcell.ColorDefault, nil
		}
		return tcell.GetColor(vs), nil
	}
	i, err := c.GetInt(key)
	if err != nil {
		return 0, err
	}
	if i > math.MaxInt32 {
		return 0, ErrInvalidType
	}
	return tcell.NewHexColor(int32(i)), nil
}

func (c mapConfig) GetRune(key string) (rune, error) {
	v, ok := c[key]
	if !ok {
		return 0, ErrNotFound
	}
	vt, ok := v.(string)
	if ok {
		return []rune(vt)[0], nil
	}

	vtr, ok := v.(rune)
	if ok {
		return vtr, nil
	}

	vtb, ok := v.(byte)
	if ok {
		return rune(vtb), nil
	}

	i, err := c.GetInt(key)
	if err != nil {
		return 0, err
	}

	return rune(i), nil
}

func (c mapConfig) Iterate(fn func(string, interface{})) {
	for k, v := range c {
		fn(k, v)
	}
}
