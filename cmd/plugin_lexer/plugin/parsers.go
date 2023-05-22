package plugin

import (
	"path/filepath"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/bash"
	"github.com/smacker/go-tree-sitter/c"
	"github.com/smacker/go-tree-sitter/cpp"
	"github.com/smacker/go-tree-sitter/csharp"
	"github.com/smacker/go-tree-sitter/css"
	"github.com/smacker/go-tree-sitter/cue"
	"github.com/smacker/go-tree-sitter/dockerfile"
	"github.com/smacker/go-tree-sitter/elixir"
	"github.com/smacker/go-tree-sitter/elm"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/hcl"
	"github.com/smacker/go-tree-sitter/html"
	"github.com/smacker/go-tree-sitter/java"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/kotlin"
	"github.com/smacker/go-tree-sitter/lua"
	"github.com/smacker/go-tree-sitter/ocaml"
	"github.com/smacker/go-tree-sitter/php"
	"github.com/smacker/go-tree-sitter/protobuf"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/ruby"
	"github.com/smacker/go-tree-sitter/rust"
	"github.com/smacker/go-tree-sitter/scala"
	"github.com/smacker/go-tree-sitter/svelte"
	"github.com/smacker/go-tree-sitter/toml"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
	"github.com/smacker/go-tree-sitter/yaml"
)

// NOTE: to add supported language from here
// https://tree-sitter.github.io/tree-sitter/ simply
// follow instructions here: https://github.com/smacker/go-tree-sitter/issues/57
var extensionToLanguage = map[string]*sitter.Language{
	".sh":           bash.GetLanguage(),
	".bash":         bash.GetLanguage(),
	".zsh":          bash.GetLanguage(),
	".bashrc":       bash.GetLanguage(),
	".profile":      bash.GetLanguage(),
	".localrc":      bash.GetLanguage(),
	".bash_profile": bash.GetLanguage(),
	".c":            c.GetLanguage(),
	".h":            c.GetLanguage(),
	".cpp":          cpp.GetLanguage(),
	".cc":           cpp.GetLanguage(),
	".hh":           cpp.GetLanguage(),
	".cs":           csharp.GetLanguage(),
	".css":          css.GetLanguage(),
	".cue":          cue.GetLanguage(),
	".dockerfile":   dockerfile.GetLanguage(),
	".ex":           elixir.GetLanguage(),
	".exs":          elixir.GetLanguage(),
	".elm":          elm.GetLanguage(),
	".go":           golang.GetLanguage(),
	".hcl":          hcl.GetLanguage(),
	".html":         html.GetLanguage(),
	".htm":          html.GetLanguage(),
	".java":         java.GetLanguage(),
	".js":           javascript.GetLanguage(),
	".kt":           kotlin.GetLanguage(),
	".kts":          kotlin.GetLanguage(),
	".lua":          lua.GetLanguage(),
	".ml":           ocaml.GetLanguage(),
	".php":          php.GetLanguage(),
	".proto":        protobuf.GetLanguage(),
	".py":           python.GetLanguage(),
	".rb":           ruby.GetLanguage(),
	".rs":           rust.GetLanguage(),
	".scala":        scala.GetLanguage(),
	".svelte":       svelte.GetLanguage(),
	".toml":         toml.GetLanguage(),
	".ts":           typescript.GetLanguage(),
	".yaml":         yaml.GetLanguage(),
	".yml":          yaml.GetLanguage(),
}

func newParser(file string) (*sitter.Parser, *sitter.Language, bool) {
	ext := filepath.Ext(file)
	lang, ok := extensionToLanguage[ext]
	if !ok {
		return nil, nil, ok
	}

	parser := sitter.NewParser()
	parser.SetLanguage(lang)
	return parser, lang, true
}
