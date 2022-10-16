package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/term"
)

func TestConfigOk(t *testing.T) {
	assert.Panics(t, func() {
		MapConfig(nil)
	})

	c := MapConfig(make(map[string]interface{}))

	_, err := c.GetString("mk")
	assert.Equal(t, ErrNotFound, err)

	_, err = c.GetBool("mk")
	assert.Equal(t, ErrNotFound, err)

	_, err = c.GetConfig("mk")
	assert.Equal(t, ErrNotFound, err)

	_, err = c.GetFloat("mk")
	assert.Equal(t, ErrNotFound, err)

	_, err = c.GetInt("mk")
	assert.Equal(t, ErrNotFound, err)

	_, err = c.GetAttribute("mk")
	assert.Equal(t, ErrNotFound, err)

	_, err = GetAttributes(c, "mk")
	assert.Equal(t, ErrNotFound, err)

	_, err = c.GetRune("mk")
	assert.Equal(t, ErrNotFound, err)

	_, err = GetFrameCharset(c, "mk", component.FrameCharSetDefault())
	assert.Equal(t, ErrNotFound, err)
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
		"attr_1": map[string]interface{}{"fg": "blue", "bg": 7},
		"attr_2": map[string]interface{}{"fg": []interface{}{"red", "bold"}},
		"charset_1": map[string]interface{}{"topleft": "a",
			"topright": "b", "bottomleft": "c",
			"bottomright": "d", "horizontalbottom": "e", "verticalleft": "f"},
		"charset_2":  map[string]interface{}{"topleft": 'a'},
		"duration_1": "2s",
	}

	tsuite := []struct {
		desc string
		cfg  Config
	}{
		{"MapConfig", MapConfig(m)},
		{"Clone", Clone(MapConfig(m))},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.desc, func(t *testing.T) {
			m := make(map[string]interface{})
			tcase.cfg.Iterate(func(k string, v interface{}) {
				m[k] = v
			})
			c1 := MapConfig(m)
			c2 := MapConfig(map[string]interface{}{
				"nested": m,
			})

			cnested, err := c2.GetConfig("nested")
			require.Nil(t, err)

			// exceptions for unmarshaled
			m["a1"] = "a"
			m["a2"] = "a"
			m["a4"] = "a"
			m["a5"] = "a"
			j1 := JSON{mapConfig(m)}
			data, err := j1.MarshalText()
			require.NoError(t, err)

			var j2 JSON
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
						out, err := c.GetString(key)
						assert.Nil(t, err)
						assert.Equal(t, "a", out, key)
					}

					ints := []string{"11", "12", "13", "14", "15", "16"}
					for _, key := range ints {
						out, err := c.GetInt(key)
						require.NoError(t, err)
						assert.Equal(t, 1, out)
					}

					floats := []string{"21", "22", "23"}
					for _, key := range floats {
						out, err := c.GetFloat(key)
						require.NoError(t, err)
						assert.True(t, 2.0 < out)
					}

					out, err := c.GetBool("true")
					require.NoError(t, err)
					assert.Equal(t, true, out)

					out, err = c.GetBool("false")
					require.NoError(t, err)
					assert.Equal(t, false, out)

					attrs, err := GetAttributes(c, "attr_1")
					require.NoError(t, err)
					assert.True(t, attrs.Fg&term.ColorBlue != 0)
					assert.True(t, attrs.Bg&term.ColorCyan != 0)

					attrs, err = GetAttributes(c, "attr_2")
					require.NoError(t, err)
					assert.True(t, attrs.Fg&term.ColorRed != 0)
					assert.True(t, attrs.Fg&term.AttrBold != 0)
					assert.Equal(t, term.ColorDefault, attrs.Bg)

					charset, err := GetFrameCharset(c, "charset_1", component.FrameCharSetDefault())
					require.NoError(t, err)
					assert.Equal(t, 'a', charset.TopLeft)
					assert.Equal(t, 'b', charset.TopRight)
					assert.Equal(t, 'c', charset.BottomLeft)
					assert.Equal(t, 'd', charset.BottomRight)
					assert.Equal(t, 'e', charset.HorizontalBottom)
					assert.Equal(t, 'f', charset.VerticalLeft)
					assert.Equal(t, '─', charset.HorizontalTop)
					assert.Equal(t, '│', charset.VerticalRight)

					charset, err = GetFrameCharset(c, "charset_2", component.FrameCharSetDefault())
					require.NoError(t, err)
					assert.Equal(t, 'a', charset.TopLeft)
					charset.TopLeft = '┌'
					assert.Equal(t, charset, component.FrameCharSetDefault())

					duration, err := GetDuration(c, "duration_1", 1*time.Second)
					require.NoError(t, err)
					assert.Equal(t, 2*time.Second, duration)

					duration, err = GetDuration(c, "duration_2", 1*time.Second)
					require.NoError(t, err)
					assert.Equal(t, 1*time.Second, duration)
				})
			}
		})
	}
}
