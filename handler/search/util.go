package search

import (
	"strings"

	fzf "github.com/junegunn/fzf/src/algo"
	"github.com/junegunn/fzf/src/util"
)

const (
	slab16Size int = 100 * 1024 // 200KB * 32 = 12.8MB
	slab32Size int = 2048       // 8KB * 32 = 256KB
)

// Match represents a search match.
type Match struct {
	idx    int
	data   []byte
	tokens *[]int
	res    fzf.Result
}

// Data returns the matched data.
func (m Match) Data() []byte {
	return m.data
}

// Score returns the match score.
func (m Match) Score() int {
	return m.res.Score
}

func makeSlab() *util.Slab {
	return util.MakeSlab(slab16Size, slab32Size)
}

func search(
	algo fzf.Algo, input [][]byte, searchQuery string,
	slab *util.Slab, caseSensitive bool,
	onMatch func(Match) bool,
) {
	const (
		forward   = false
		normalize = false
		withPos   = true // return matched tokens
	)

	if !caseSensitive {
		// if caseSensitive is disabled, search query is expected
		// to be lowercase.
		searchQuery = strings.ToLower(searchQuery)
	}

	searchRunes := []rune(searchQuery)
	for i, data := range input {
		chars := util.ToChars(data)
		res, tokens := algo(caseSensitive, normalize, forward, &chars,
			searchRunes, withPos, slab)
		if res.Start < 0 {
			continue
		}
		match := Match{data: data, res: res, tokens: tokens, idx: i}
		if !onMatch(match) {
			return
		}
	}
}

func simpleSearch(
	algo fzf.Algo, input [][]byte, query string, caseSensitive bool,
) []Match {
	res := make([]Match, 0)
	slab := makeSlab()
	search(algo, input, query, slab, caseSensitive,
		func(m Match) bool {
			res = append(res, m)
			return true
		})
	return res
}

// Fuzzy performs an approximate string matching search on input with query and
// returns a list of matching results.
func Fuzzy(input [][]byte, query string, caseSensitive bool) []Match {
	return simpleSearch(fzf.FuzzyMatchV2, input, query, caseSensitive)
}

// Equal performs a string matching search on input with query and
// returns a list of matching results.
func Equal(input [][]byte, query string, caseSensitive bool) []Match {
	return simpleSearch(fzf.EqualMatch, input, query, caseSensitive)
}

// Contains performs a string contains search on input with query and
// returns a list of matching results.
func Contains(input [][]byte, query string, caseSensitive bool) []Match {
	return simpleSearch(containsMatch, input, query, caseSensitive)
}

func containsMatch(
	caseSensitive bool, normalize bool,
	forward bool, text *util.Chars, pattern []rune, withPos bool, slab *util.Slab,
) (fzf.Result, *[]int) {
	lenPattern := len(pattern)
	if lenPattern == 0 {
		return fzf.Result{Start: 0, End: 0, Score: 1}, nil
	}

	runesStr := string(text.ToRunes())
	if !caseSensitive {
		runesStr = strings.ToLower(runesStr)
	}

	idx := strings.Index(runesStr, string(pattern))
	if idx == -1 {
		return fzf.Result{Start: -1, End: -1, Score: 0}, nil
	}

	tokens := make([]int, 0, len(pattern))
	for i := idx; i < idx+len(pattern); i++ {
		tokens = append(tokens, i)
	}

	return fzf.Result{
		Start: idx,
		End:   idx + lenPattern,
		Score: lenPattern,
	}, &tokens
}
