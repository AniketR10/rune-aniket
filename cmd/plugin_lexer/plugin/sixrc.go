package plugin

import (
	"errors"

	"github.com/alecthomas/chroma/lexers"
)

func setupConfigLexer() error {
	lexer := lexers.Match(".yaml")
	if lexer == nil {
		return errors.New("could not find yaml lexer")
	}
	cfg := lexer.Config()
	cfg.Name = "sixrc"
	cfg.Filenames = []string{".sixrc", ".sixdevrc", ".oxrc", ".oxdevrc"}
	lexers.Register(lexer)
	return nil
}
