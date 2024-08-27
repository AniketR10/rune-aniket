// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package extension

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/unstablebuild/blue/iterator"
	browserapi "unstable.build/go-tui/api/browser"
	"unstable.build/go-tui/api/config"
	configextension "unstable.build/go-tui/api/config/extension"
	textapi "unstable.build/go-tui/api/text"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/cmd/extension_fuzzy_file/finder"
	"unstable.build/go-tui/extension"
	"unstable.build/go-tui/extension/extutil"
	"unstable.build/go-tui/rpc"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/workspace/walkdir"
)

var (
	cmdSearchTypes = textapi.CommandManual{
		Name:    "searchSyntaxTypes",
		Summary: "Fuzzy search 'definition.type' symbols in the workspace's AST, using the detected programming language's default AST queries.",
	}
	cmdSearchVariables = textapi.CommandManual{
		Name:    "searchSyntaxVariables",
		Summary: "Fuzzy search 'definition.var' symbols in the workspace's AST, using the detected programming language's default AST queries.",
	}
	cmdSearchFunctions = textapi.CommandManual{
		Name:    "searchSyntaxFunctions",
		Summary: "Fuzzy search 'definition.function' symbols in the workspace's AST, using the detected programming language's default AST queries.",
	}
	cmdSearchSyntax = textapi.CommandManual{
		Name:     "searchSyntax",
		Summary:  "Fuzzy search custom symbols in the workspace's AST, using the given query. Check tree-sitter's manual for more details https://tree-sitter.github.io/tree-sitter/using-parsers#query-syntax",
		Synopsis: "query",
	}
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
	queryTypeFunctions queryType = "definition.function"
	queryTypeVariables queryType = "definition.var"
	queryTypeTypes     queryType = "definition.type"
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
		it, err := walkdir.ListFiles(ctx, cwd, ".")
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
	ctx context.Context, cmd textapi.Command,
	grants []extension.Grant, broker rpc.MuxBroker,
	invokeWindow browserapi.Window, c config.Config,
) (browserapi.Handler, error) {
	noHistoryKey := term.KeyComb{}

	tabspaces := -1
	for _, grant := range grants {
		if grant.Permission == extension.PermissionConfig {
			config, err := configextension.FetchConfig(ctx, grant, broker)
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
	case cmdSearchTypes.Name:
		qtype = queryTypeTypes
	case cmdSearchVariables.Name:
		qtype = queryTypeVariables
	case cmdSearchFunctions.Name:
		qtype = queryTypeFunctions
	case cmdSearchSyntax.Name:
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
