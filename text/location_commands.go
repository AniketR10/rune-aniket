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

package text

import (
	"context"
	"errors"
	"strings"

	"github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/iterator"
	"unstable.build/go-tui/api/textapi"
	"unstable.build/go-tui/api/workspaceapi"
)

// SubscribeLocationCommands returns a slice of location list related commands
// that can be registered for the returned CommandHandler.
func SubscribeLocationCommands(
	file workspaceapi.URI, registry FileCommandRegistry, ed Handler,
) (Handler, error) {
	ret := locationCommandHandler{
		file:     file,
		registry: registry,
		Handler:  ed,
	}
	for _, cmd := range locationCommands {
		_ = registry.SubscribeCommandForFile(file, cmd, ret)
	}
	return ret, nil
}

const (
	commandLocationJump = "locationjump"
)

var locationCommands = []textapi.CommandManual{
	{
		Name:     commandLocationJump,
		Summary:  "Jumps to locations on the given location list.",
		Synopsis: "(next|prev) <location-list>",
		Commands: []textapi.CommandManual{
			{
				Name:     "next",
				Summary:  "Jumps to the next location.",
				Synopsis: "<location-list>",
			},
			{
				Name:     "previous",
				Summary:  "Jumps to the previous location.",
				Synopsis: "<location-list>",
			},
		},
	},
}

type locationCommandHandler struct {
	file     workspaceapi.URI
	registry FileCommandRegistry
	Handler
}

func (u locationCommandHandler) Close() (ret error) {
	for _, cmd := range locationCommands {
		err := u.registry.UnsubscribeCommandForFile(u.file, cmd.Name)
		if err != nil {
			ret = multierror.Append(ret, err)
		}
	}
	return
}

func (u locationCommandHandler) HandleCommand(
	ctx context.Context, cmd textapi.Command,
) (err error) {
	switch cmd.Name {
	case commandLocationJump:
		err = u.handleLocationJump(cmd)
	default:
		err = errors.New("extraneous command")
	}
	return
}

func (u locationCommandHandler) Complete(ctx context.Context, cmd textapi.Command) (
	ret iterator.Iterator[string], _ string, err error,
) {
	switch cmd.Name {
	case commandLocationJump:
		ret, err = u.completeLocationJump(cmd)
	default:
		err = errors.New("extraneous command")
	}
	return
}

func (u locationCommandHandler) handleLocationJump(cmd textapi.Command) error {
	if len(cmd.Args) == 0 {
		return errors.New("missing next/previous and location list id")
	}
	if len(cmd.Args) == 1 {
		return errors.New("missing location list id")
	}
	lists := u.Handler.LocationLists()
	for _, list := range lists {
		if list.ID != cmd.Args[1] {
			continue
		}
		switch cmd.Args[0] {
		case "next":
			ok := u.Handler.MoveToNextLocation(cmd.Args[1])
			if !ok {
				log.Debugf("reached end of location list")
			}
			return nil
		case "previous":
			ok := u.Handler.MoveToPrevLocation(cmd.Args[1])
			if !ok {
				log.Debugf("reached start of location list")
			}
			return nil
		}
		return errors.New("only 'next' or 'previous' is accepted")
	}
	return errors.New("location list does not exist")
}

func (u locationCommandHandler) completeLocationJump(
	cmd textapi.Command,
) (ret iterator.Iterator[string], err error) {
	if len(cmd.Args) == 1 {
		ret = iterator.FromSlice([]string{"previous", "next"})
		return
	}
	lists := u.Handler.LocationLists()
	if len(cmd.Args) == 2 {
		var ids []string
		for _, list := range lists {
			// lists starting with _ are not displayed.
			// This is useful to hide lists that might
			// be needed for internal implementations.
			if strings.HasPrefix(list.ID, "_") {
				continue
			}
			ids = append(ids, list.ID)
		}
		ret = iterator.FromSlice(ids)
		return
	}
	ret = iterator.Empty[string]()
	return
}
