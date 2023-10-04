package extension

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ernestrc/blue/iterator"
	browserapi "unstable.build/go-tui/api/browser"
	"unstable.build/go-tui/api/config"
	configextension "unstable.build/go-tui/api/config/extension"
	textapi "unstable.build/go-tui/api/text"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/cmd/extension_fuzzy_file/finder"
	"unstable.build/go-tui/extension"
	extutil "unstable.build/go-tui/extension/util"
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

// Grantee returns this extension's Grantee and the permissions required to run it.
func Grantee() (extension.Grantee, []extension.Permission) {
	perms := finder.Permissions()
	perms = append(perms, extension.PermissionConfig)

	searchFunctions, finalPerms := extutil.NewCommandSplitHandler(extutil.CommandSplitHandlerConfig{
		SplitOrientation: browserapi.OrientationBottom,
		Handler:          newHandler,
		Permissions:      perms,
		Command:          cmdSearchFunctions,
	})

	searchVariables, _ := extutil.NewCommandSplitHandler(extutil.CommandSplitHandlerConfig{
		SplitOrientation: browserapi.OrientationBottom,
		Handler:          newHandler,
		Permissions:      perms,
		Command:          cmdSearchVariables,
	})

	searchTypes, _ := extutil.NewCommandSplitHandler(extutil.CommandSplitHandlerConfig{
		SplitOrientation: browserapi.OrientationBottom,
		Handler:          newHandler,
		Permissions:      perms,
		Command:          cmdSearchTypes,
	})

	searchSyntax, _ := extutil.NewCommandSplitHandler(extutil.CommandSplitHandlerConfig{
		SplitOrientation: browserapi.OrientationBottom,
		Handler:          newHandler,
		Permissions:      perms,
		Command:          cmdSearchSyntax,
	})

	multi := extutil.MultiGrantee(
		searchFunctions,
		searchVariables,
		searchTypes,
		searchSyntax,
	)

	return multi, finalPerms
}

type queryType string

const (
	queryTypeCustom    queryType = ""
	queryTypeFunctions           = "definition.function"
	queryTypeVariables           = "definition.var"
	queryTypeTypes               = "definition.type"
)

func defaultQueryForFilename(filename string) (string, error) {
	langID, ok := LanguageID(filename)
	if !ok {
		return "", errUnknownLanguage
	}

	queries, ok := languageQueriesByName[langID]
	if !ok {
		return "", errUnknownLanguage
	}
	return queries.locals, nil
}

func readSymbolsFunction(qtype queryType, tabspaces int) func(
	workspaceapi.FileSystem, context.Context) (iterator.Iterator[string], error) {
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
		return readSymbols(ctx, cwd, it, qtype, tabspaces, queryFn)
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
	cmd textapi.Command, grants []extension.Grant, broker proto.MuxBroker,
	invokeWindow browserapi.Window, c config.Config,
) (browserapi.Handler, error) {
	noHistoryKey := term.KeyComb{}
	ctx := context.Background()

	tabspaces := -1
	for _, grant := range grants {
		if grant.Permission == extension.PermissionConfig {
			config, err := configextension.FetchConfig(grant, broker)
			if err != nil {
				return nil, err
			}
			tabspaces, err = extutil.Tabspaces(config)
			if err != nil {
				return nil, fmt.Errorf("get tabspaces from config: %v", err)
			}
		}
	}

	if tabspaces == -1 {
		return nil, errors.New("cannot parse AST correctly without access to configuration permission")
	}

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
		c, noHistoryKey, "unused", "",
		readSymbolsFunction(qtype, tabspaces), parseLine)
}
