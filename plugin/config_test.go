package plugin

import (
	"testing"

	"github.com/ernestrc/go-tui/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigOk(t *testing.T) {
	assert.Panics(t, func() {
		NewConfig(nil)
	})

	c := NewConfig(make(map[string]interface{}))

	_, ok := c.GetString("mk")
	assert.False(t, ok)

	_, ok = c.GetBool("mk")
	assert.False(t, ok)

	_, ok = c.GetConfig("mk")
	assert.False(t, ok)

	_, ok = c.GetFloat("mk")
	assert.False(t, ok)

	_, ok = c.GetInt("mk")
	assert.False(t, ok)

	_, ok = c.GetAttribute("mk")
	assert.False(t, ok)

	_, ok = c.GetAttributes("mk")
	assert.False(t, ok)
}
func TestConfigTypes(t *testing.T) {
	m := map[string]interface{}{
		"a1":     rune('a'),
		"a2":     byte('a'),
		"a3":     string('a'),
		"a4":     []byte{'a'},
		"a5":     []rune{'a'},
		"11":     1,
		"12":     int(1),
		"13":     int32(1),
		"14":     int64(1),
		"15":     float64(1),
		"16":     float32(1),
		"21":     2.1,
		"22":     float64(2.1),
		"23":     float32(2.1),
		"true":   true,
		"false":  false,
		"attr_1": map[string]interface{}{"fg": "blue", "bg": "cyan"},
		"attr_2": map[string]interface{}{"fg": []interface{}{"red", "bold"}},
	}

	c1 := NewConfig(m)
	c2 := NewConfig(map[string]interface{}{
		"nested": m,
	})

	cnested, ok := c2.GetConfig("nested")
	require.True(t, ok)

	// exceptions for unmarshaled
	m["a1"] = "a"
	m["a2"] = "a"
	m["a4"] = "a"
	m["a5"] = "a"
	j1 := jsonMap{mapConfig(m)}
	data, err := j1.MarshalText()
	require.NoError(t, err)

	var j2 jsonMap
	err = j2.UnmarshalText(data)
	require.NoError(t, err)
	cunmarshaled := j2.mapConfig

	suite := []struct {
		description string
		Config
	}{
		{"simple", c1},
		{"nested", cnested},
		{"unmarshaled", cunmarshaled},
	}

	for _, tcase := range suite {
		c := tcase.Config
		t.Run(tcase.description, func(t *testing.T) {
			strings := []string{"a1", "a2", "a3", "a4", "a5"}
			for _, key := range strings {
				out, ok := c.GetString(key)
				assert.True(t, ok)
				assert.Equal(t, "a", out, key)
			}

			ints := []string{"11", "12", "13", "14", "15", "16"}
			for _, key := range ints {
				out, ok := c.GetInt(key)
				assert.True(t, ok)
				assert.Equal(t, 1, out)
			}

			floats := []string{"21", "22", "23"}
			for _, key := range floats {
				out, ok := c.GetFloat(key)
				assert.True(t, ok)
				assert.True(t, 2.0 < out)
			}

			out, ok := c.GetBool("true")
			assert.True(t, ok)
			assert.Equal(t, true, out)

			out, ok = c.GetBool("false")
			assert.True(t, ok)
			assert.Equal(t, false, out)

			attrs, ok := c.GetAttributes("attr_1")
			assert.True(t, ok)
			assert.True(t, attrs.Fg&term.ColorBlue != 0)
			assert.True(t, attrs.Bg&term.ColorCyan != 0)

			attrs, ok = c.GetAttributes("attr_2")
			assert.True(t, ok)
			assert.True(t, attrs.Fg&term.ColorRed != 0)
			assert.True(t, attrs.Fg&term.AttrBold != 0)
			assert.Equal(t, term.ColorDefault, attrs.Bg)
		})
	}
}
