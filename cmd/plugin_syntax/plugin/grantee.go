package plugin

import (
	"context"
	"strconv"
	"strings"

	"github.com/ernestrc/blue/iterator"
	browserapi "unstable.build/go-tui/api/browser"
	"unstable.build/go-tui/api/config"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/cmd/plugin_fuzzy_file/finder"
	"unstable.build/go-tui/plugin"
	plugutil "unstable.build/go-tui/plugin/util"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/workspace"
)

const (
	// TODO use embedded queries
	// TODO add ability to pass argument in finder.Handler to the command
	// which then gets passed to readSymbolsFunction
	queryListFunctions = "(function_declaration) @func"
	queryListVariables = "(short_var_declaration) @short\n(var_declaration) @var"
	queryListTypes     = "(type_declaration) @type"
)

// Grantee returns this plugin's Grantee and the permissions required to run it.
func Grantee() (plugin.Grantee, []plugin.Permission) {
	searchFunctions, perms := plugutil.NewCommandSplitHandler(plugutil.CommandSplitHandlerConfig{
		SplitOrientation: browserapi.OrientationBottom,
		Handler:          newHandler(queryListFunctions),
		Permissions:      finder.Permissions(),
		Command:          "searchFunctions",
	})

	searchVariables, _ := plugutil.NewCommandSplitHandler(plugutil.CommandSplitHandlerConfig{
		SplitOrientation: browserapi.OrientationBottom,
		Handler:          newHandler(queryListVariables),
		Permissions:      finder.Permissions(),
		Command:          "searchVariables",
	})

	searchTypes, _ := plugutil.NewCommandSplitHandler(plugutil.CommandSplitHandlerConfig{
		SplitOrientation: browserapi.OrientationBottom,
		Handler:          newHandler(queryListTypes),
		Permissions:      finder.Permissions(),
		Command:          "searchTypes",
	})

	multi := plugutil.MultiGrantee(
		searchFunctions,
		searchVariables,
		searchTypes,
	)

	return multi, perms
}

func readSymbolsFunction(query string) func(workspaceapi.FileSystem, context.Context) (iterator.Iterator[string], error) {
	return func(cwd workspaceapi.FileSystem, ctx context.Context) (iterator.Iterator[string], error) {
		it, err := workspace.ListFiles(ctx, cwd, ".")
		if err != nil {
			return nil, err
		}

		return readSymbols(ctx, cwd, it, query)
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

func newHandler(query string) func([]plugin.Grant, proto.MuxBroker, browserapi.Window, config.Config) (browserapi.Handler, error) {
	noHistoryKey := term.KeyComb{}
	return func(
		grants []plugin.Grant, broker proto.MuxBroker,
		invokeWindow browserapi.Window, c config.Config,
	) (browserapi.Handler, error) {
		return finder.New(grants, broker, invokeWindow,
			c, noHistoryKey, "unused", "",
			readSymbolsFunction(query), parseLine)
	}
}
