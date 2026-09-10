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

package font

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/blue/logging"
	"golang.org/x/image/font/sfnt"
	"unstable.build/rune/internal/workspace/walkdir"
)

type findFont interface {
	findByFamily(family string) (iterator.Iterator[metadata], error)
	list() (iterator.Iterator[metadata], error)
}

type systemFindFont struct {
	reader walkdir.Reader
}

func (s systemFindFont) findByFamily(family string) (
	iterator.Iterator[metadata], error,
) {
	fonts, err := s.find(matchFamily(family))
	if err != nil {
		return nil, err
	}

	it, isEmpty := iterator.IsEmpty(context.Background(), fonts)
	if isEmpty {
		return nil, fmt.Errorf("font '%s' not found", family)
	}
	return it, nil
}

func (s systemFindFont) list() (iterator.Iterator[metadata], error) {
	fonts, err := s.find(nil)
	if err != nil {
		return nil, err
	}
	return fonts, nil
}

func (s systemFindFont) find(matcher matcher) (
	iterator.Iterator[metadata], error,
) {
	ctx := context.Background()
	var iters []iterator.Iterator[metadata]
	for _, dir := range fontDirs() {
		if info, err := os.Stat(dir); os.IsNotExist(err) || !info.IsDir() {
			continue
		}

		it, err := walkdir.ListFiles(ctx, s.reader, dir)
		if err != nil {
			s.log(log.WarnLevel, "list files in dir '%s': %v", dir, err)
			continue
		}

		validit := iterator.Filter(it, func(path string) bool {
			ext := filepath.Ext(path)
			return strings.EqualFold(ext, ".ttf") ||
				strings.EqualFold(ext, ".ttc") ||
				strings.EqualFold(ext, ".otc") ||
				strings.EqualFold(ext, ".otf")
		})

		metait := iterator.Map(validit, func(path string) []metadata {
			f, err := os.Open(path)
			if err != nil {
				s.log(log.WarnLevel, "open font file '%s': %v", path, err)
				return nil
			}
			defer f.Close()
			m, err := readMetadata(path, f)
			if err != nil {
				s.log(log.WarnLevel, "read font '%s' metadata: %v", path, err)
				return nil
			}
			return m
		})

		unsliceit := iterator.Unslice(metait)
		if matcher == nil {
			iters = append(iters, unsliceit)
			continue
		}

		matchit := iterator.Filter(unsliceit, func(m metadata) bool {
			return matcher(m)
		})

		iters = append(iters, matchit)
	}

	return iterator.Aggregate(iters...), nil
}

func (p *systemFindFont) log(level log.Level, msg string, args ...any) {
	if !log.IsLevelEnabled(level) {
		return
	}
	log.WithFields(log.Fields{
		logging.KeyClass: "font.Manager",
	}).Logf(level, msg, args...)
}

type matcher func(metadata) bool

func matchFamily(family string) matcher {
	return func(m metadata) bool {
		return strings.EqualFold(m.family, family)
	}
}

type metadata struct {
	family string
	path   string
}

// readMetadata reads family names through r without loading the whole
// file: system collections such as Apple Color Emoji are hundreds of
// megabytes and discovery only needs their names.
func readMetadata(path string, r io.ReaderAt) ([]metadata, error) {
	col, err := sfnt.ParseCollectionReaderAt(r)
	if err != nil {
		return nil, fmt.Errorf("sfnt parse collection: %w", err)
	}
	if col.NumFonts() == 0 {
		return nil, errors.New("no fonts found in file")
	}
	ret := make([]metadata, col.NumFonts())
	for i := 0; i < col.NumFonts(); i++ {
		font, err := col.Font(i)
		if err != nil {
			return nil, fmt.Errorf("font %d of collection: %w", i, err)
		}
		var buf sfnt.Buffer
		family, err := font.Name(&buf, sfnt.NameIDFamily)
		if err != nil {
			return nil, fmt.Errorf("font %d read family: %w", i, err)
		}
		ret[i] = metadata{
			path:   path,
			family: family,
		}
	}
	return ret, nil
}

func expandUser(path string) (expandedPath string) {
	if strings.HasPrefix(path, "~") {
		if u, err := user.Current(); err == nil {
			return strings.ReplaceAll(path, "~", u.HomeDir)
		}
	}
	return path
}
