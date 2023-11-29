package component

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRowHeight(t *testing.T) {
	t.Run("takes advantage of the full width", func(t *testing.T) {
		row := NewRow()
		row.AddComponent(StringResponsive("123456789", StringResponsiveConfig{}), MaxCols)
		assert.Equal(t, 1, row.Height(9))
	})
	t.Run("passes the correct width to components", func(t *testing.T) {
		row := NewRow()
		row.AddComponent(StringResponsive("123456789", StringResponsiveConfig{}), MaxCols/2)
		assert.Equal(t, 3, row.Height(9))
	})
	t.Run("returns the max height", func(t *testing.T) {
		row := NewRow()
		row.AddComponent(StringResponsive("12", StringResponsiveConfig{}), MaxCols/2)
		row.AddComponent(StringResponsive("123456789", StringResponsiveConfig{}), MaxCols/2)
		assert.Equal(t, 3, row.Height(9))
	})
}
