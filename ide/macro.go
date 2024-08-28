package ide

import (
	"errors"
	"fmt"
	"strings"

	"unstable.build/go-tui/term"
)

type macroKey struct {
	term.KeyComb
	instructWait bool
}

// parseMacroKeys recursively parses key combinations combined with instructions,
// encoded between {} characters.
func parseMacroKeys(sequence string) (ret []macroKey, err error) {
	idxOpen := strings.IndexRune(sequence, '{')
	if idxOpen < 0 {
		idxOpen = len(sequence)
	}

	// parse up until first instruction
	var keys []term.KeyComb
	keys, err = term.ParseKeys(sequence[0:idxOpen])
	for _, key := range keys {
		ret = append(ret, macroKey{KeyComb: key})
	}
	if err != nil {
		ret = nil
		return
	}

	remainder := sequence[idxOpen:]
	if remainder == "" {
		return
	}

	idxClose := strings.IndexRune(remainder, '}')
	if idxClose < 0 {
		err = errors.New("unterminated key: '{' found but no matching '}' found")
		ret = nil
		return
	}
	instruction := remainder[0 : idxClose+1]
	switch instruction {
	case "{wait}":
		ret = append(ret, macroKey{instructWait: true})
	default:
		err = fmt.Errorf("invalid instruction: %s", instruction)
	}
	if err != nil {
		ret = nil
		return
	}

	var recKeys []macroKey
	recKeys, err = parseMacroKeys(remainder[idxClose+1:])
	if err != nil {
		ret = nil
		return
	}
	ret = append(ret, recKeys...)
	return
}
