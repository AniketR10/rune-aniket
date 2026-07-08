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

package symboldb

import "unstable.build/go-tui/ide/idelsp/symbolresolve"

const (
	filesPartition   = "files"
	symbolsPartition = "symbols"
	namesPartition   = "names"

	metaScanID = "scan"

	// schemaVersion identifies the layout of the persisted partitions.
	// A database written under a different version is fully re-indexed:
	// its derived tables (such as the names partition) may not exist,
	// and per-file records alone cannot rebuild them because unchanged
	// files are normally skipped.
	schemaVersion = 1
)

// Kinds of stored symbol locations. Refs and defs answer 2-part
// (pkg.Sym) lookups; method defs answer 3-part (pkg.Type.Method)
// lookups only. The values are persisted, so the mapping from
// symbolresolve.SymbolKind is explicit rather than relying on that
// enum's ordering.
const (
	kindRef = iota
	kindDef
	kindMethodDef
)

// storedKind maps an extracted symbol kind to its persisted value.
func storedKind(k symbolresolve.SymbolKind) int {
	switch k {
	case symbolresolve.SymbolDef:
		return kindDef
	case symbolresolve.SymbolMethodDef:
		return kindMethodDef
	default:
		return kindRef
	}
}

// symbolLoc is one location contributed by a file to a symbol.
type symbolLoc struct {
	URI  string
	X    int
	Y    int
	Kind int
}

// symbolDoc aggregates every known location of one qualified symbol
// name. Stored in the symbols partition under the name itself.
type symbolDoc struct {
	Name string
	Locs []symbolLoc
}

// nameDoc marks one symbol name as listed. Stored in the names
// partition under the name itself so ListReferencedSymbols can stream
// names without decoding the location-heavy symbol docs: the storage
// backend's List decodes every doc in a partition in full.
type nameDoc struct {
	Name string
}

// listed reports whether locs contain at least one reference or
// definition, the criterion for a name to appear in
// ListReferencedSymbols. Method-definition-only names are excluded,
// matching the backing parser's output.
func listed(locs []symbolLoc) bool {
	for _, l := range locs {
		if l.Kind == kindRef || l.Kind == kindDef {
			return true
		}
	}
	return false
}

// fileDoc records a file's contribution to the index: the timestamp
// and schema version it was indexed under, the symbol names it
// contributed (for invalidation) and its import alias→path map (for
// import dedup at query time). Stored in the files partition under the
// file URI. The per-record version lets an interrupted schema rebuild
// resume: records already rewritten by the current version are trusted
// on the next launch, records from older versions are not.
type fileDoc struct {
	URI     string
	Path    string
	ModTime int64
	Version int
	Names   []string
	Imports map[string]string
}

// metaDoc marks whether a full workspace scan has ever completed and
// which schema version wrote the database.
type metaDoc struct {
	Complete bool
	Version  int
}
