package config

import "github.com/ernestrc/tcell/v3"

type nopConfig struct{}

// NopConfig returns a Config that always returns ErrNotFound.
func NopConfig() Config {
	return nopConfig{}
}

func (n nopConfig) GetInt(string) (int, error) {
	return 0, ErrNotFound
}

func (n nopConfig) GetFloat(string) (float64, error) {
	return 0, ErrNotFound
}

func (n nopConfig) GetString(string) (string, error) {
	return "", ErrNotFound
}

func (n nopConfig) GetBool(string) (bool, error) {
	return false, ErrNotFound
}

func (n nopConfig) GetConfig(string) (Config, error) {
	return nil, ErrNotFound
}

func (n nopConfig) GetMap(string) (map[string]interface{}, error) {
	return nil, ErrNotFound
}

func (n nopConfig) GetAttribute(string) (tcell.AttrMask, error) {
	return 0, ErrNotFound
}

func (n nopConfig) GetColor(string) (tcell.Color, error) {
	return 0, ErrNotFound
}

func (n nopConfig) GetRune(string) (rune, error) {
	return 0, ErrNotFound
}

func (n nopConfig) GetSlice(string) ([]interface{}, error) {
	return nil, ErrNotFound
}

func (n nopConfig) Iterate(fn func(k string, value interface{})) {
}
