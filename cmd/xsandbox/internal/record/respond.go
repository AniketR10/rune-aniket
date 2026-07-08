// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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
