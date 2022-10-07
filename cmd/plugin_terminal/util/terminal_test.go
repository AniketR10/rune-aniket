package termutil

import "testing"

func BenchmarkProcessRunes1(b *testing.B) {
	benchmarkProcessRunes(b, MeasuredRune{Rune: 'a', Width: 1})
}

func BenchmarkProcessRunes10(b *testing.B) {
	b.SkipNow()

	rs := []MeasuredRune{}
	for i := 0; i < 10; i++ {
		rs = append(rs, MeasuredRune{Rune: 'z', Width: 1})
	}
	benchmarkProcessRunes(b, rs...)
}

func benchmarkProcessRunes(b *testing.B, r ...MeasuredRune) {
	t := New(nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = t.processRunes(r...)
	}
}
