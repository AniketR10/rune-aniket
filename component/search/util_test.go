package search

import (
	"testing"

	fzf "github.com/junegunn/fzf/src/algo"
	"github.com/junegunn/fzf/src/util"
	"github.com/stretchr/testify/assert"
)

func TestSearch(t *testing.T) {
	tsuite := []struct {
		input  []string
		search string
		result []string
	}{
		{
			input:  []string{},
			search: "a",
			result: nil,
		},
		{
			input:  []string{"a.go", "b.go"},
			search: "",
			result: []string{"a.go", "b.go"},
		},
		{
			input:  []string{"a.go", "b.go"},
			search: "a",
			result: []string{"a.go"},
		},
		{
			input:  []string{"a.go", "b.go"},
			search: "b",
			result: []string{"b.go"},
		},
		{
			input:  []string{"a.go", "b.go"},
			search: ".",
			result: []string{"a.go", "b.go"},
		},
		{
			input:  []string{"caliu.go", "claudi.go"},
			search: "au",
			result: []string{"caliu.go", "claudi.go"},
		},
	}

	for _, tcase := range tsuite {
		caseSensitive := true

		searchQuery := tcase.search
		var in [][]byte
		for _, f := range tcase.input {
			in = append(in, []byte(f))
		}
		matches := Fuzzy(in, searchQuery, caseSensitive)

		var res []string
		for _, m := range matches {
			res = append(res, string(m.Data()))
		}
		assert.Equal(t, tcase.result, res)
	}
}

func benchSearch(b *testing.B, n int) {
	slab := util.MakeSlab(slab16Size, slab32Size)
	algo := fzf.FuzzyMatchV2
	caseSensitive := true

	searchQuery := "au"
	var in [][]byte
	template := []string{"caliu.go", "claudi.go"}
	for i := 0; i < n; i++ {
		for _, f := range template {
			in = append(in, []byte(f))
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		search(algo, in, searchQuery, slab, caseSensitive,
			func(match Match, tokens *[]int) bool {
				return true
			})
	}
}

func BenchmarkSearch1(b *testing.B) {
	benchSearch(b, 1)
}

func BenchmarkSearch10(b *testing.B) {
	benchSearch(b, 10)
}

func BenchmarkSearch100(b *testing.B) {
	benchSearch(b, 100)
}

func BenchmarkSearch1000(b *testing.B) {
	benchSearch(b, 1000)
}

func BenchmarkSearch100000(b *testing.B) {
	benchSearch(b, 100000)
}
