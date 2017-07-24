package tiled

// TODO
// func TestWindowManager(t *testing.T) {
// 	width, height := 8, 8
// 	w := writer.String(width, height)
// 	m, _, err := New(width, height, &noopWindow{fill: 'A'})
//
// 	if err != nil {
// 		t.Fatal(err)
// 	}
//
// 	expected := "AAAAAAAA\nAAAAAAAA\nAAAAAAAA\nAAAAAAAA"
//
// 	if err := m.Draw(w); err != nil {
// 		t.Fatal(err)
// 	}
//
// 	if err := w.Flush(); err != nil {
// 		t.Fatal(err)
// 	}
//
// 	if expected != w.String() {
// 		t.Errorf("expected %q found %q", expected, w.String())
// 	}
// }
