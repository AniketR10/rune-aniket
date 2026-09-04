// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package record

import (
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
)

// build synthesizes the scripted reply for fullMethod. Response
// messages are built dynamically from the protobuf registry so the
// sandbox can script any registered service without per-service code.
func (r *Response) build(fullMethod string) (proto.Message, error) {
	if r.ErrCode != "" || r.ErrMsg != "" {
		code, err := parseCode(r.ErrCode)
		if err != nil {
			return nil, err
		}
		return nil, status.Error(code, r.ErrMsg)
	}
	out, err := methodOutputDescriptor(fullMethod)
	if err != nil {
		return nil, err
	}
	msg := dynamicpb.NewMessage(out)
	body := r.Body
	if body == nil {
		body = map[string]any{}
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal scripted response for %s: %w", fullMethod, err)
	}
	if err := protojson.Unmarshal(data, msg); err != nil {
		return nil, fmt.Errorf(
			"scripted response for %s does not decode into %s: %w",
			fullMethod, out.FullName(), err)
	}
	return msg, nil
}

func methodOutputDescriptor(fullMethod string) (protoreflect.MessageDescriptor, error) {
	service, method, ok := strings.Cut(NormalizeMethod(fullMethod), "/")
	if !ok {
		return nil, fmt.Errorf("invalid rpc method %q", fullMethod)
	}
	desc, err := protoregistry.GlobalFiles.FindDescriptorByName(
		protoreflect.FullName(service))
	if err != nil {
		return nil, fmt.Errorf("unknown rpc service %q: %w", service, err)
	}
	sd, ok := desc.(protoreflect.ServiceDescriptor)
	if !ok {
		return nil, fmt.Errorf("%q is not an rpc service", service)
	}
	md := sd.Methods().ByName(protoreflect.Name(method))
	if md == nil {
		return nil, fmt.Errorf("unknown rpc method %q on service %q", method, service)
	}
	return md.Output(), nil
}

var codeNames = map[string]codes.Code{
	"canceled":            codes.Canceled,
	"unknown":             codes.Unknown,
	"invalid_argument":    codes.InvalidArgument,
	"deadline_exceeded":   codes.DeadlineExceeded,
	"not_found":           codes.NotFound,
	"already_exists":      codes.AlreadyExists,
	"permission_denied":   codes.PermissionDenied,
	"resource_exhausted":  codes.ResourceExhausted,
	"failed_precondition": codes.FailedPrecondition,
	"aborted":             codes.Aborted,
	"out_of_range":        codes.OutOfRange,
	"unimplemented":       codes.Unimplemented,
	"internal":            codes.Internal,
	"unavailable":         codes.Unavailable,
	"data_loss":           codes.DataLoss,
	"unauthenticated":     codes.Unauthenticated,
}

func parseCode(name string) (codes.Code, error) {
	if name == "" {
		return codes.Unknown, nil
	}
	code, ok := codeNames[strings.ToLower(name)]
	if !ok {
		return codes.Unknown, fmt.Errorf("unknown grpc error code %q", name)
	}
	return code, nil
}
