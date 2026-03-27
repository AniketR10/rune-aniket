// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2018-2024 Unstable Build, All Rights Reserved.
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

package storagerpc

import (
	"errors"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/docmarshal"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagerpc/docpb"
)

// this is just a trick to be able to re-use encode functionality
const protoFieldKey = "X"

func makeModelUpdates(m docmarshal.Marshaler, updates []*docpb.UpdateDocumentRequest_Field) (
	ret []storageapi.Update, err error,
) {
	fields, err := makeModelFields(m, updates)
	if err != nil {
		return nil, err
	}
	for _, field := range fields {
		ret = append(ret, storageapi.Update(field))
	}
	return
}

func makeModelPreconds(m docmarshal.Marshaler, preconds []*docpb.UpdateDocumentRequest_Field) (
	ret []storageapi.Precondition, err error,
) {
	fields, err := makeModelFields(m, preconds)
	if err != nil {
		return nil, err
	}
	for _, field := range fields {
		ret = append(ret, storageapi.Precondition(field))
	}
	return
}

func makeModelFields(m docmarshal.Marshaler, fields []*docpb.UpdateDocumentRequest_Field) (
	ret []storageapi.Field, err error,
) {
	var slab map[string]any
	var f storageapi.Filter

	for _, u := range fields {
		// re-use make filter logic
		pf := docpb.ListDocumentRequest_Filter{
			FieldPath: u.FieldPath,
			Data:      u.Data,
		}
		f, err = makeModelFilter(m, slab, &pf)
		if err != nil {
			return
		}
		ret = append(ret, storageapi.Field{
			FieldPath: f.FieldPath,
			Value:     f.Value,
		})
	}
	return
}

func makeModelFilter(m docmarshal.Marshaler,
	slab map[string]any, pf *docpb.ListDocumentRequest_Filter,
) (storageapi.Filter, error) {
	err := storageapi.SafeDecode(m, &slab, pf.Data)
	if err != nil {
		return storageapi.Filter{}, err
	}

	lowerCase := m.DefaultLowerCase()

	fieldPath := pf.FieldPath
	if lowerCase {
		fieldPath = make([]string, len(pf.FieldPath))
		for i, comp := range pf.FieldPath {
			fieldPath[i] = strings.ToLower(comp)
		}
	}
	if len(fieldPath) == 0 {
		return storageapi.Filter{}, errors.New("empty field path")
	}
	return storageapi.Filter{
		Field: storageapi.Field{
			FieldPath: fieldPath,
			Value:     slab[protoFieldKey],
		},
		Op: storageapi.Op(pf.Operation),
	}, nil
}

func makeModelFilters(m docmarshal.Marshaler, filters []*docpb.ListDocumentRequest_Filter) (
	ret []storageapi.Filter, err error,
) {
	var slab map[string]any
	for _, pf := range filters {
		var f storageapi.Filter
		f, err = makeModelFilter(m, slab, pf)
		if err != nil {
			return
		}
		ret = append(ret, f)
	}
	return
}
