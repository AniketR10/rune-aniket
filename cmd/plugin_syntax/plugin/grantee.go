package plugin

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/ernestrc/blue/iterator"
	browserapi "unstable.build/go-tui/api/browser"
	"unstable.build/go-tui/api/config"
	textapi "unstable.build/go-tui/api/text"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/cmd/plugin_fuzzy_file/finder"
	lexerPlugin "unstable.build/go-tui/cmd/plugin_lexer/plugin"
	"unstable.build/go-tui/plugin"
	plugutil "unstable.build/go-tui/plugin/util"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/workspace"
)

const (
	cmdSearchTypes     = "searchSyntaxTypes"
	cmdSearchVariables = "searchSyntaxVariables"
	cmdSearchFunctions = "searchSyntaxFunctions"
	cmdSearchSyntax    = "searchSyntax"
)

// Grantee returns this plugin's Grantee and the permissions required to run it.
func Grantee() (plugin.Grantee, []plugin.Permission) {
	searchFunctions, perms := plugutil.NewCommandSplitHandler(plugutil.CommandSplitHandlerConfig{
		SplitOrientation: browserapi.OrientationBottom,
		Handler:          newHandler,
		Permissions:      finder.Permissions(),
		Command:          cmdSearchFunctions,
	})

	searchVariables, _ := plugutil.NewCommandSplitHandler(plugutil.CommandSplitHandlerConfig{
		SplitOrientation: browserapi.OrientationBottom,
		Handler:          newHandler,
		Permissions:      finder.Permissions(),
		Command:          cmdSearchVariables,
	})

	searchTypes, _ := plugutil.NewCommandSplitHandler(plugutil.CommandSplitHandlerConfig{
		SplitOrientation: browserapi.OrientationBottom,
		Handler:          newHandler,
		Permissions:      finder.Permissions(),
		Command:          cmdSearchTypes,
	})

	searchSyntax, _ := plugutil.NewCommandSplitHandler(plugutil.CommandSplitHandlerConfig{
		SplitOrientation: browserapi.OrientationBottom,
		Handler:          newHandler,
		Permissions:      finder.Permissions(),
		Command:          cmdSearchSyntax,
	})

	multi := plugutil.MultiGrantee(
		searchFunctions,
		searchVariables,
		searchTypes,
		searchSyntax,
	)

	return multi, perms
}

type queryType string

const (
	queryTypeCustom    queryType = ""
	queryTypeFunctions           = "definition.function"
	queryTypeVariables           = "definition.var"
	queryTypeTypes               = "definition.type"
)

func defaultQueryForFilename(filename string) (string, error) {
	langID, ok := lexerPlugin.LanguageID(filename)
	if !ok {
		return "", errUnknownLanguage
	}

	queries, ok := languageQueriesByName[langID]
	if !ok {
		return "", errUnknownLanguage
	}
	return queries.locals, nil
}

func readSymbolsFunction(qtype queryType) func(workspaceapi.FileSystem, context.Context) (iterator.Iterator[string], error) {
	return func(cwd workspaceapi.FileSystem, ctx context.Context) (iterator.Iterator[string], error) {
		it, err := workspace.ListFiles(ctx, cwd, ".")
		if err != nil {
			return nil, err
		}
		queryFn := defaultQueryForFilename
		query, ok := queryFromContext(ctx)
		if ok {
			queryFn = func(string) (string, error) {
				return query, nil
			}
			qtype = queryTypeCustom
		}
		return readSymbols(ctx, cwd, it, qtype, queryFn)
	}
}

func parseLine(workspace workspaceapi.FileSystem, data string) (
	workspaceapi.URI, term.Coordinates,
) {
	chunks := strings.Split(data, ":")
	y, _ := strconv.Atoi(chunks[1])
	name := chunks[0]
	uri, _ := workspace.URI(name)
	return uri, term.Coordinates{Y: y - 1}
}

func newHandler(
	cmd textapi.Command, grants []plugin.Grant, broker proto.MuxBroker,
	invokeWindow browserapi.Window, c config.Config,
) (browserapi.Handler, error) {
	noHistoryKey := term.KeyComb{}
	ctx := context.Background()

	var qtype queryType
	switch cmd.Name {
	case cmdSearchTypes:
		qtype = queryTypeTypes
	case cmdSearchVariables:
		qtype = queryTypeVariables
	case cmdSearchFunctions:
		qtype = queryTypeFunctions
	case cmdSearchSyntax:
		if len(cmd.Args) == 0 {
			return nil, errors.New("expected source code matching query as command arguments")
		}
		qtype = queryTypeCustom
		ctx = contextWithQuery(ctx, strings.Join(cmd.Args, " "))
	default:
		panic("dispatched unknown command")
	}

	return finder.New(ctx, grants, broker, invokeWindow,
		c, noHistoryKey, "unused", "", readSymbolsFunction(qtype), parseLine)
}
