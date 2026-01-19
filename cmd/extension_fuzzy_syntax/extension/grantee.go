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
	"unstable.build/go-tui/api/browserapi"
	"unstable.build/go-tui/api/config"
	"unstable.build/go-tui/api/extensionapi"
	"unstable.build/go-tui/api/textapi"
	"unstable.build/go-tui/api/workspaceapi"
	"unstable.build/go-tui/cmd/extension_fuzzy_file/finder"
	"unstable.build/go-tui/extension"
	"unstable.build/go-tui/extension/extutil"
	"unstable.build/go-tui/rpc"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/workspace/walkdir"
)

var (
	cmdSearchTypes = textapi.CommandManual{
		Name:    "searchtypes",
		Summary: "Fuzzy search 'definition.type' symbols in the workspace's AST, using the detected programming language's default AST queries.",
	}
	cmdSearchVariables = textapi.CommandManual{
		Name:    "searchvar",
		Summary: "Fuzzy search 'definition.var' symbols in the workspace's AST, using the detected programming language's default AST queries.",
	}
	cmdSearchFunctions = textapi.CommandManual{
		Name:    "searchfunc",
		Summary: "Fuzzy search 'definition.function' symbols in the workspace's AST, using the detected programming language's default AST queries.",
	}
	cmdSearchSyntax = textapi.CommandManual{
		Name:     "searchsyntax",
		Summary:  "Fuzzy search custom symbols in the workspace's AST, using the given query. Check tree-sitter's manual for more details https://tree-sitter.github.io/tree-sitter/using-parsers#query-syntax",
		Synopsis: "query",
	}
)

// Grantee returns this extension's Grantee and the permissions required to run it.
func Grantee() (extension.Grantee, []extensionapi.Permission) {
	perms := finder.Permissions()
	perms = append(perms, extensionapi.PermissionConfig)

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

	/*searchSyntax, _ := extutil.NewCommandSplitHandler(extutil.CommandSplitHandlerConfig{
		SplitOrientation: browserapi.OrientationBottom,
		Handler:          newHandler,
		Permissions:      perms,
		Command:          cmdSearchSyntax,
	})*/

	multi := extutil.MultiGrantee(
		searchFunctions,
		searchVariables,
		searchTypes,
		//searchSyntax,
	)

	return multi, finalPerms
}

type queryType string

const (
	queryTypeCustom    queryType = ""
	queryTypeFunctions queryType = "local.definition.function"
	queryTypeVariables queryType = "local.definition.var"
	queryTypeTypes     queryType = "local.definition.type"
)

func readSymbolsFunction(dataDir string, qtype queryType) func(
	workspaceapi.FileSystem, context.Context) (iterator.Iterator[string], error) {
	return func(cwd workspaceapi.FileSystem, ctx context.Context) (iterator.Iterator[string], error) {
		it, err := walkdir.ListFiles(ctx, cwd, ".")
		if err != nil {
			return nil, err
		}
		query, ok := queryFromContext(ctx)
		if ok {
			qtype = queryTypeCustom
		}
		uri, err := cwd.URI(".")
		if err != nil {
			return nil, err
		}
		return readSymbols(ctx, cwd, dataDir, uri, it, qtype, query)
	}
}

func parseLine(exec workspaceapi.FileSystem, data string) (
	uri workspaceapi.URI, coords term.Coordinates, ok bool,
) {
	chunks := strings.Split(data, ":")
	if len(chunks) < 2 {
		return workspaceapi.URI{}, term.Coordinates{}, false
	}
	y, _ := strconv.Atoi(chunks[1])
	name := chunks[0]
	uri, err := exec.URI(name)
	return uri, term.Coordinates{Y: y - 1}, err == nil
}

func newHandler(
	ctx context.Context, cmd textapi.Command,
	grants []extension.Grant, broker rpc.MuxBroker,
	invokeWindow browserapi.Window, c config.Config,
) (extutil.RedispatchHandler, error) {
	noHistoryKey := term.KeyComb{}

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
	dataDir, err := c.GetString("datadir")
	if err != nil {
		return nil, fmt.Errorf("could not get data directory: %v", err)
	}
	return finder.New(ctx, grants, broker, invokeWindow,
		c, noHistoryKey, "unused", "",
		readSymbolsFunction(dataDir, qtype), parseLine)
}
