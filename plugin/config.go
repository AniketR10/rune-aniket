package plugin

import (
	"encoding"
	"encoding/json"
)

// Config is the interface implemented by a plugin configuration provider.
type Config interface {
	GetInt(string) (int, bool)
	GetFloat(string) (float64, bool)
	GetString(string) (string, bool)
	GetBool(string) (bool, bool)
	GetConfig(string) (Config, bool)
}

type internalConfig interface {
	encoding.TextMarshaler
	encoding.TextUnmarshaler
	Config
}

type jsonMap struct {
	mapConfig
}

// satisfies Config but not internalConfig
type mapConfig map[string]interface{}

func (c *jsonMap) MarshalText() ([]byte, error) {
	text, err := json.Marshal(c.mapConfig)
	return text, err
}

func (c *jsonMap) UnmarshalText(text []byte) error {
	c.mapConfig = make(map[string]interface{})
	err := json.Unmarshal(text, &c.mapConfig)
	return err
}

func (c mapConfig) GetInt(key string) (int, bool) {
	v, ok := c[key]
	if !ok {
		return 0, ok
	}

	switch vt := v.(type) {
	case int:
		return vt, true
	case float64:
		return int(vt), true
	case float32:
		return int(vt), true
	case int64:
		return int(vt), true
	case int32:
		return int(vt), true
	default:
		return 0, false
	}
}

func (c mapConfig) GetFloat(key string) (float64, bool) {
	v, ok := c[key]
	if !ok {
		return 0, ok
	}

	switch vt := v.(type) {
	case float64:
		return vt, true
	case int:
		return float64(vt), true
	case float32:
		return float64(vt), true
	case int64:
		return float64(vt), true
	case int32:
		return float64(vt), true
	default:
		return 0, false
	}
}

func (c mapConfig) GetString(key string) (string, bool) {
	v, ok := c[key]
	if !ok {
		return "", ok
	}
	switch vt := v.(type) {
	case string:
		return vt, true
	case []byte:
		return string(vt), true
	case []rune:
		return string(vt), true
	case rune:
		return string(vt), true
	case byte:
		return string(vt), true
	default:
		return "", false
	}
}

func (c mapConfig) GetBool(key string) (bool, bool) {
	v, ok := c[key]
	if !ok {
		return false, ok
	}
	vt, ok := v.(bool)
	return vt, ok
}

func (c mapConfig) GetConfig(key string) (Config, bool) {
	v, ok := c[key]
	if !ok {
		return nil, ok
	}
	vt, ok := v.(map[string]interface{})
	if !ok {
		return nil, false
	}

	return mapConfig(vt), ok
}

// NewConfig converts a map into a plugin.Config.
func NewConfig(m map[string]interface{}) Config {
	if m == nil {
		panic("invalid argument: map cannot be nil")
	}
	return mapConfig(m)
}

// NOTE(ernestrc): whatever impl we return in NewConfig,
// should be the same one cast here. There should not
// be any other Config castings in the codebase.
func toInternalConfig(c Config) mapConfig {
	return c.(mapConfig)
}
