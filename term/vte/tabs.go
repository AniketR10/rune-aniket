package vte

const initialTabstops int = 8

type tabstops struct {
	tabs []bool
}

func (t *tabstops) init(columns int) {
	t.tabs = make([]bool, columns)
	for i := range t.tabs {
		t.tabs[i] = i%initialTabstops == 0
	}
}

func (t *tabstops) clearAll() {
	for i := range t.tabs {
		t.tabs[i] = false
	}
}

func (t *tabstops) resize(columns int) {
	if columns < len(t.tabs) {
		t.tabs = t.tabs[:columns]
		return
	}

	i := len(t.tabs)
	for i < columns {
		t.tabs = append(t.tabs, i%initialTabstops == 0)
		i++
	}
}

func (t *tabstops) get(i int) bool {
	return t.tabs[i]
}

func (t *tabstops) set(i int, value bool) {
	t.tabs[i] = value
}
