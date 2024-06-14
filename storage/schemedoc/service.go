package schemedoc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"reflect"

	multierr "github.com/ernestrc/go-multierror"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/encoding"
	"github.com/unstablebuild/blue/iterator"
	schemeapi "unstable.build/go-tui/api/scheme"
	workspaceapi "unstable.build/go-tui/api/workspace"
)

// ErrClosing is returned to inflight write requests
// before comitting changes to disk if the service
// is currently closing.
var ErrClosing = errors.New("Service is closing")

// NewDocumentService returns a document.Service backed by a schemeapi.Scheme.
// It its goroutine-safe but only one instance can be operating at a time
// on a given workspace.
func NewDocumentService(scheme schemeapi.Scheme, marshaler encoding.Marshaler) (
	document.Service, error,
) {
	svc := service{
		scheme:    scheme,
		marshaler: marshaler,
	}
	// make it goroutine-safe
	return document.Sync(&svc), nil
}

type service struct {
	scheme    schemeapi.Scheme
	marshaler encoding.Marshaler

	// Used to wait on all writes before Close returns.
	// This is to guarantee that once lock is released,
	// there are no writes to the underlying files.
	wg closeGroup
}

func (s *service) Create(ctx context.Context, ID string, doc interface{}) error {
	return s.create(ctx, ID, doc, os.O_CREATE|os.O_EXCL|os.O_WRONLY)
}

func (s *service) Set(ctx context.Context, ID string, doc interface{}) error {
	return s.create(ctx, ID, doc, os.O_CREATE|os.O_WRONLY|os.O_TRUNC)
}

func (s *service) Update(
	ctx context.Context, ID string, updates []document.Update,
	preconds ...document.Precondition,
) error {
	if len(updates) == 0 {
		panic("Update: no paths to update")
	}
	if ID == "" {
		return errors.New("invalid ID: empty")
	}
	origFileName := s.getFileName(ID)
	orig, werr := s.scheme.Open(origFileName, os.O_RDONLY, 0)
	if werr != nil {
		if werr.IsNotExist {
			return document.ErrNotFound
		}
		if werr.IsPermission {
			return document.ErrPermissionDenied
		}
		return fmt.Errorf("scheme open: %v", werr.ToError())
	}

	proto := make(map[string]interface{})
	err := s.read(orig, &proto)
	if cerr := orig.Close(); cerr != nil {
		err = multierr.Append(err, cerr)
	}
	if err != nil {
		return err
	}

	// order of operations (open .swp, update proto, etc) doesn't matter
	// because all methods are serialized via document.Sync, and
	// there can only be one instance of this document.Service operating
	// at a given workspace at a time.
	err = document.UpdateProto(s.marshaler, updates, proto, preconds...)
	if err != nil {
		return err
	}

	targetFileName := s.getFileName(ID) + ".swp"
	target, werr := s.scheme.Open(targetFileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if werr != nil {
		if werr.IsPermission {
			return document.ErrPermissionDenied
		}
		return fmt.Errorf("scheme open: %v", werr.ToError())
	}

	err = s.write(target, proto)
	if cerr := target.Close(); cerr != nil {
		err = multierr.Append(err, cerr)
	}
	if err != nil {
		return err
	}

	err = s.scheme.Rename(targetFileName, origFileName)
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return document.ErrPermissionDenied
		}
		return fmt.Errorf("scheme rename: %v", err)
	}

	return nil
}

func (s *service) Get(ctx context.Context, ID string, doc interface{}) error {
	if ID == "" {
		return errors.New("invalid ID: empty")
	}
	if !document.IsEncodeable(doc) {
		return errors.New("invalid document argument")
	}
	f, werr := s.scheme.Open(s.getFileName(ID), os.O_RDONLY, 0)
	if werr != nil {
		if werr.IsNotExist {
			return document.ErrNotFound
		}
		if werr.IsPermission {
			return document.ErrPermissionDenied
		}
		return fmt.Errorf("scheme open: %v", werr.ToError())
	}
	defer f.Close()

	return s.read(f, doc)
}

func (s *service) Delete(ctx context.Context, ID string) error {
	if ID == "" {
		return errors.New("invalid ID: empty")
	}
	err := s.scheme.Remove(s.getFileName(ID))
	// delete should be idempotent
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if errors.Is(err, os.ErrPermission) {
		return document.ErrPermissionDenied
	}
	return err
}

func (s *service) List(ctx context.Context, filters []document.Filter) (document.Iterator, error) {
	for _, f := range filters {
		if len(f.FieldPath) == 0 || f.Op == "" {
			panic("invalid filter")
		}
	}
	entries, err := s.scheme.ReadDir(".")
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return nil, document.ErrPermissionDenied
		}
		return nil, fmt.Errorf("scheme list files: %v", err)
	}
	it := iterator.Map(iterator.FromSlice(entries), func(entry os.DirEntry) string {
		return entry.Name()
	})
	return &docIter{filters: filters, svc: s, it: it}, nil
}

func (s *service) Close() (ret error) {
	// wait on writes to finish and prevent any new
	// writes from making progress
	if !s.wg.Close() {
		// already closed
		return
	}
	if err := s.scheme.Close(); err != nil {
		ret = multierr.Append(ret, err)
	}
	return ret
}

func (s *service) read(f workspaceapi.File, doc interface{}) error {
	data, err := io.ReadAll(f)
	if err != nil {
		return fmt.Errorf("file read all: %v", err)
	}

	err = s.marshaler.Unmarshal(data, doc)
	if err != nil {
		return fmt.Errorf("unmarshal %s: %v: %s", f.Name(), err, string(data))
	}
	return nil
}

func (s *service) getFileName(id string) string {
	return url.PathEscape(id)
}

func (s *service) create(ctx context.Context, ID string, doc interface{}, openFlags int) error {
	if doc == nil {
		panic("invalid nil data argument to Create/Set")
	}
	if ID == "" {
		return errors.New("invalid ID: empty")
	}
	doc, err := document.DerefCreateValue(reflect.ValueOf(doc))
	if err != nil {
		return err
	}
	f, werr := s.scheme.Open(s.getFileName(ID), openFlags, 0666)
	if werr != nil {
		if werr.IsExist {
			return document.ErrAlreadyExists
		}
		if werr.IsPermission {
			return document.ErrPermissionDenied
		}
		return fmt.Errorf("scheme open: %v", werr.ToError())
	}

	doc = document.UpdateCreatedAtField(s.marshaler, doc)

	var ret error
	if err := s.write(f, doc); err != nil {
		ret = multierr.Append(ret, err)
	}
	if cerr := f.Close(); cerr != nil {
		ret = multierr.Append(ret, cerr)
	}
	return ret
}

func (s *service) write(f workspaceapi.File, doc interface{}) error {
	data, err := s.marshaler.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshal: %v", err)
	}
	done, ok := s.wg.AddOne()
	if !ok {
		return ErrClosing
	}
	defer done()
	_, err = f.Write(data)
	if err != nil {
		return fmt.Errorf("file write: %v", err)
	}
	err = f.Sync()
	if err != nil {
		return fmt.Errorf("file sync: %v", err)
	}
	return nil
}

type docIter struct {
	filters []document.Filter
	svc     *service
	it      iterator.Iterator[string]

	doneErr       error
	nextMatchFile workspaceapi.File
}

func (d *docIter) HasNext() (ok bool) {
	for d.nextMatchFile == nil && d.doneErr == nil {
		nextFile, ok := d.it.Next()
		if !ok {
			d.doneErr = d.it.Err()
			return false
		}
		f, werr := d.svc.scheme.Open(d.svc.getFileName(nextFile), os.O_RDONLY, 0)
		if werr != nil {
			if werr.IsNotExist {
				continue // should not happend but let's be resilient
			}
			if werr.IsPermission {
				d.doneErr = document.ErrPermissionDenied
				return true
			}
			d.doneErr = werr.ToError()
			return true
		}

		proto := make(map[string]interface{})
		err := d.svc.read(f, &proto)
		if err != nil {
			if cerr := f.Close(); cerr != nil {
				d.doneErr = multierr.Append(err, cerr)
				return true
			}
			d.doneErr = err
			return true
		}

		if document.MatchesAllFilters(d.svc.marshaler, proto, d.filters) {
			d.nextMatchFile = f
			return true
		}

		if cerr := f.Close(); cerr != nil {
			d.doneErr = cerr
			return true
		}

		/* continue */
	}

	return true
}

func (d *docIter) NextTo(doc interface{}) error {
	if !document.IsEncodeable(doc) {
		return errors.New("receiver is not a pointer and not a map or is nil")
	}
	if d.doneErr != nil {
		return d.doneErr
	}
	if d.nextMatchFile == nil {
		if !d.HasNext() {
			return errors.New("exhausted iterator")
		}
		if d.doneErr != nil {
			return d.doneErr
		}
	}
	f := d.nextMatchFile
	d.nextMatchFile = nil
	defer f.Close()
	_, err := f.Seek(0, 0)
	if err != nil {
		return fmt.Errorf("file seek: %v", err)
	}
	return d.svc.read(f, doc)
}

func (d *docIter) Close() error {
	return nil
}
