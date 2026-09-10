// Copyright (C) 2017-2026 The Rune Authors
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
