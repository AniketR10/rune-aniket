package extension

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	"github.com/unstablebuild/blue/encoding"
	"github.com/unstablebuild/blue/issue"
	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui/api/config"
	schemeapi "unstable.build/go-tui/api/scheme"
	workspaceapi "unstable.build/go-tui/api/workspace"
)

const (
	defaultMaxID = 9
)

// issueMapperScheme returns a schemeapi.SchemeFunc that wraps the given
// schemeFunc to apply mappingFunc to the iterator of file names returned
// by ListFiles.
func issueMapperScheme(
	schemeFunc schemeapi.SchemeFunc,
	marshaler encoding.Marshaler,
	maxSubjectLen int,
) schemeapi.SchemeFunc {
	return func(ctx context.Context, cfg config.Config, uri workspaceapi.URI) (
		schemeapi.Scheme, error,
	) {
		s, err := schemeFunc(ctx, cfg, uri)
		if err != nil {
			return nil, err
		}
		return mapper{maxSubjectLen: maxSubjectLen, Scheme: s, m: marshaler}, nil
	}
}

type mapper struct {
	schemeapi.Scheme // of T
	m                encoding.Marshaler
	maxSubjectLen    int
}

func (m mapper) Open(path string, flag int, perm os.FileMode) (
	workspaceapi.File, *workspaceapi.Error,
) {
	path = parseLabels(path)
	// do not allow writing of new arbitraryly-named issues
	if flag&os.O_CREATE != 0 {
		// if this is an open request with O_CREATE for a non swap
		// then it return permission error so file logic opens read-only.
		if !strings.HasPrefix(path, ".") || !strings.HasSuffix(path, ".swp") ||
			strings.HasSuffix(path, ".swp.swp") { // naugthy boy
			return nil, &workspaceapi.Error{IsPermission: true}
		}
		// if this is an O_CREATE for a swap, only allow if issue already exists
		// i.e. we're editing the issue
		pathNoSwap := path[1 : len(path)-4]
		_, err := m.Stat(pathNoSwap)
		if err != nil {
			log.Debugf("Stat(%s): %v", pathNoSwap, err)
			return nil, workspaceapi.NopError(
				fmt.Errorf("use '%s' command to create new issues", defaultCreateIssueCmd))
		}
	}
	return m.Scheme.Open(path, flag, perm)
}

func (m mapper) Stat(path string) (os.FileInfo, error) {
	path = parseLabels(path)
	return m.Scheme.Stat(path)
}
func (m mapper) URI(path string) (workspaceapi.URI, error) {
	path = parseLabels(path)
	return m.Scheme.URI(path)
}

func (m mapper) ReadDir(name string) (
	[]os.DirEntry, error,
) {
	name = parseLabels(name)
	entries, err := m.Scheme.ReadDir(name)
	if err != nil {
		return nil, err
	}

	var ret []os.DirEntry
	for _, entry := range entries {
		filename := entry.Name()
		// swap file for an open issue
		if strings.HasSuffix(filename, ".swp") {
			continue
		}
		f, werr := m.Scheme.Open(filename, os.O_RDONLY, 0)
		if werr != nil {
			return nil, werr.ToError()
		}
		data, err := ioutil.ReadAll(f)
		if err != nil {
			return nil, err
		}
		var temp issue.ReportDocument
		err = m.m.Unmarshal(data, &temp)
		if err != nil {
			return nil, err
		}
		if temp.Report.Closed {
			continue
		}
		nameWithLabels := makeLabels(temp, m.maxSubjectLen)
		ret = append(ret, mappedEntry{name: nameWithLabels, DirEntry: entry})
	}

	return ret, nil
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

type mappedEntry struct {
	name string
	os.DirEntry
}

func (m mappedEntry) Name() string {
	return m.name
}
