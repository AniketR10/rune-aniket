package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	"github.com/ernestrc/blue/encoding"
	"github.com/ernestrc/blue/issue"
	"github.com/ernestrc/blue/iterator"
	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui/config"
	"unstable.build/go-tui/workspace"
)

const (
	defaultMaxID = 9
)

// issueMapperScheme returns a workspace.SchemeFunc that wraps the given
// schemeFunc to apply mappingFunc to the iterator of file names returned
// by ListFiles.
func issueMapperScheme(
	schemeFunc workspace.SchemeFunc,
	marshaler encoding.Marshaler,
	maxSubjectLen int,
) workspace.SchemeFunc {
	return func(cfg config.Config, uri workspace.URI) (workspace.Scheme, error) {
		s, err := schemeFunc(cfg, uri)
		if err != nil {
			return nil, err
		}
		return mapper{maxSubjectLen: maxSubjectLen, Scheme: s, m: marshaler}, nil
	}
}

type mapper struct {
	workspace.Scheme // of T
	m                encoding.Marshaler
	maxSubjectLen    int
}

func (m mapper) Open(path string, flag int, perm os.FileMode) (
	workspace.File, *workspace.Error,
) {
	path = parseLabels(path)
	// do not allow writing of new arbitraryly-named issues
	if flag&os.O_CREATE != 0 {
		// if this is an open request with O_CREATE for a non swap
		// then it return permission error so file logic opens read-only.
		if !strings.HasPrefix(path, ".") || !strings.HasSuffix(path, ".swp") {
			return nil, &workspace.Error{IsPermission: true}
		}
		if strings.HasSuffix(path, ".swp.swp") {
			// naughty, naughty boy
			return nil, &workspace.Error{IsPermission: true}
		}
		// if this is an O_CREATE for a swap, only allow if issue already exists
		// i.e. we're editing the issue
		pathNoSwap := path[1 : len(path)-4]
		_, err := m.Stat(pathNoSwap)
		if err != nil {
			log.Debugf("Stat(%s): %v", pathNoSwap, err)
			return nil, workspace.NopError(
				fmt.Errorf("use '%s' command to create new issues", defaultCreateIssueCmd))
		}
	}
	return m.Scheme.Open(path, flag, perm)
}

func (m mapper) Stat(path string) (os.FileInfo, error) {
	path = parseLabels(path)
	return m.Scheme.Stat(path)
}
func (m mapper) URI(path string) (workspace.URI, error) {
	path = parseLabels(path)
	return m.Scheme.URI(path)
}

func (m mapper) ListFiles(ctx context.Context) (
	iterator.Iterator[string], error,
) {
	it, err := m.Scheme.ListFiles(ctx)
	if err != nil {
		return nil, err
	}

	return &mappingIterator{
		scheme:        m,
		it:            it,
		m:             m.m,
		maxSubjectLen: m.maxSubjectLen,
	}, nil
}

type mappingIterator struct {
	scheme        mapper
	m             encoding.Marshaler
	it            iterator.Iterator[string]
	maxSubjectLen int

	nextErr error
}

func (m *mappingIterator) Next() (string, bool) {
	for {
		filename, ok := m.it.Next()
		if !ok {
			return "", false
		}

		f, werr := m.scheme.Open(filename, os.O_RDONLY, 0)
		if werr != nil {
			m.nextErr = werr.ToError()
			return "", false
		}
		data, err := ioutil.ReadAll(f)
		if err != nil {
			m.nextErr = err
			return "", false
		}

		var temp issue.ReportDocument
		err = m.m.Unmarshal(data, &temp)
		if err != nil {
			return "", false
		}

		if temp.Report.Closed {
			continue
		}
		return makeLabels(temp, m.maxSubjectLen), true
	}
}

func (m *mappingIterator) Err() error {
	err := m.it.Err()
	if err != nil {
		return err
	}
	return m.nextErr
}

func makeLabels(temp issue.ReportDocument, maxSubjectLen int) string {
	var builder strings.Builder

	id := temp.ID()
	if len(id) > defaultMaxID {
		builder.WriteString(id[:defaultMaxID])
	} else {
		builder.WriteString(id)
		for i := 0; i < defaultMaxID-len(id); i++ {
			builder.WriteByte(' ')
		}
	}
	builder.WriteByte(' ')

	subject := temp.Report.Subject
	if len(subject) > maxSubjectLen {
		builder.WriteString(subject[:maxSubjectLen-3])
		builder.WriteString("...")
	} else {
		builder.WriteString(subject)
		for i := 0; i < maxSubjectLen-len(subject); i++ {
			builder.WriteByte(' ')
		}
	}
	builder.WriteByte(' ')

	metadata := make([]string, 0, len(temp.Report.Metadata))
	for label := range temp.Report.Metadata {
		if strings.HasPrefix(label, "_") || label == "stack" {
			continue
		}
		metadata = append(metadata, label)
	}

	// sort so it's the same order for every row with the same labels
	sort.Strings(metadata)

	for _, key := range metadata {
		value := temp.Report.Metadata[key]
		builder.WriteString(key)
		if value != "" {
			builder.WriteByte(':')
			builder.WriteString(value)
		}
		builder.WriteByte(' ')
	}

	return builder.String()
}

func parseLabels(str string) string {
	path := strings.Split(str, " ")
	return path[0]
}
