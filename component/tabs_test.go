package component

import (
	"testing"

	"github.com/ernestrc/go-tui/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTabsDraw(t *testing.T) {
	l := NewTabs()
	l.Resize(20, 4)
	fb := DefaultFrameBorders()
	fb.BottomLeft.Ch, fb.BottomRight.Ch = '├', '┤'
	l.SetFrameBorders(fb)

	w := term.NewStringWriter(20, 9)

	tests := []testCase{
		{
			nil, `
┌──────────────────┐
│                  │
│                  │
├──────────────────┤
                    
                    
                    
                    
                    `,
		}, {
			func() { l.Add("Atzari") }, `
┌──────────────────┐
│Atzari            │
│                  │
├──────────────────┤
                    
                    
                    
                    
                    `,
		}, {
			func() { l.Resize(20, 9) }, `
┌──────────────────┐
│                  │
│                  │
│                  │
│Atzari            │
│                  │
│                  │
│                  │
├──────────────────┤`,
		}, {
			func() {
				l.Add("Saturn")
				l.Resize(20, 4)
			}, `
┌──────────────────┐
│Atzari  Saturn    │
│                  │
├──────────────────┤
                    
                    
                    
                    
                    `,
		}, {
			func() { l.Add("Other") }, `
┌──────────────────┐
│Atzari  Saturn  ..│
│                  │
├──────────────────┤
                    
                    
                    
                    
                    `,
		}, {
			func() { l.Add("Things") }, `
┌──────────────────┐
│Atzari  Saturn  ..│
│                  │
├──────────────────┤
                    
                    
                    
                    
                    `,
		}, {
			func() { l.SetFocus(2) }, `
┌──────────────────┐
│..  Saturn  Othe..│
│                  │
├──────────────────┤
                    
                    
                    
                    
                    `,
		}, {
			func() { l.Add("Morsins"); l.Add("Morsillonins") }, `
┌──────────────────┐
│..  Saturn  Othe..│
│                  │
├──────────────────┤
                    
                    
                    
                    
                    `,
		}, {
			func() { l.SetFocus(5) }, `
┌──────────────────┐
│..  Morsillonins  │
│                  │
├──────────────────┤
                    
                    
                    
                    
                    `,
		}, {
			func() { l.SetBorder(false); l.Resize(20, 1) }, `
..  Morsillonins    
                    
                    
                    
                    
                    
                    
                    
                    `,
		}, {
			func() { l.SetFocus(3) }, `
..  Other  Things ..
                    
                    
                    
                    
                    
                    
                    
                    `,
		}, {
			func() { l.SetBorder(false); l.Resize(4, 0) }, `
                    
                    
                    
                    
                    
                    
                    
                    
                    `,
		},
	}

	testWorkflow(t, l, w, tests)
}

func setupOneTab(width, height int) *Tabs {
	l := NewTabs()
	l.Resize(width, height)

	const name = "blah"
	l.Add(name)

	return l
}

func TestTabsTabAt(t *testing.T) {
	t.Run("should return false if no tab in list", func(t *testing.T) {
		l := NewTabs()
		_, ok := l.TabAt(term.Coordinates{})
		assert.False(t, ok)
	})

	t.Run("should return first tab if coordinates is zero", func(t *testing.T) {
		l := setupOneTab(4, 4)

		idx, ok := l.TabAt(term.Coordinates{})
		require.True(t, ok)
		assert.Equal(t, "blah", l.tabs[idx].name)
	})

	t.Run("should return first tab if size is 0", func(t *testing.T) {
		l := setupOneTab(0, 0)

		l.TabAt(term.Coordinates{})
		idx, ok := l.TabAt(term.Coordinates{})
		require.True(t, ok)
		assert.Equal(t, "blah", l.tabs[idx].name)
	})

	t.Run("should take offset into consideration", func(t *testing.T) {
		l := setupOneTab(4, 4)

		l.Add("111111")
		l.Add("222222222222222222222")
		l.Add("3")
		l.SetFocus(l.Add("4"))

		// force calculating offsets
		w := term.NewStringWriter(4, 4)
		l.Draw(w)
		require.NoError(t, w.Flush())
		t.Log(w.String())

		idx, ok := l.TabAt(term.Coordinates{})
		require.True(t, ok)
		assert.Equal(t, "4", l.tabs[idx].name)
	})
}

func TestTabsFocus(t *testing.T) {
	l := setupOneTab(10, 10)
	idx, ok := l.Focus()
	require.True(t, ok)
	assert.Equal(t, 0, idx)

	assert.Panics(t, func() {
		l.SetFocus(2)
	})

	assert.Equal(t, 1, l.Add("fjkewl"))
	l.SetFocus(1)

	idx, ok = l.Focus()
	require.True(t, ok)
	assert.Equal(t, 1, idx)

	assert.Equal(t, 2, l.Add("jkflejwkl"))
	assert.Equal(t, 3, l.Add("jkflejwkl"))
	assert.Equal(t, 4, l.Add("jkflejwkl"))
	l.SetFocus(4)

	idx, ok = l.Focus()
	require.True(t, ok)
	assert.Equal(t, 4, idx)
}
