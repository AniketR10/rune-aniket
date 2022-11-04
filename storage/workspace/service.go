package workspace

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"

	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/iterator"
	multierr "github.com/ernestrc/go-multierror"
	"unstable.build/go-tui/storage/encoding"
	"unstable.build/go-tui/workspace"
)

// ErrClosing is returned to inflight write requests
// before comitting changes to disk if the service
// is currently closing.
var ErrClosing = errors.New("Service is closing")

// NewWorkspaceService returns a document.Service backed by a workspace.Scheme.
// It its goroutine-safe but only one instance can be operating at a time
// on a given workspace.
func NewWorkspaceService(scheme workspace.Scheme, marshaler encoding.Marshaler) (document.Service, error) {
	svc := service{
		scheme:    scheme,
		marshaler: marshaler,
	}
	// make it goroutine-safe
	return document.Sync(&svc), nil
}

type service struct {
	scheme    workspace.Scheme
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
	origFileName := s.getFileName(ID)
	orig, werr := s.scheme.Open(origFileName, os.O_RDONLY, 0)
	if werr != nil {
		if werr.IsNotExist {
			return document.ErrNotFound
		}
		return fmt.Errorf("Scheme.Open: %v", werr.ToError())
	}

	proto := make(map[string]interface{})
	err := s.read(orig, &proto)
	if cerr := orig.Close(); cerr != nil {
		err = multierr.Append(err, cerr)
	}
	if err != nil {
		return err
	}

	err = document.UpdateProto(updates, proto, preconds...)
	if err != nil {
		return err
	}

	targetFileName := s.getFileName(ID) + ".swp"
	target, werr := s.scheme.Open(targetFileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if werr != nil {
		return fmt.Errorf("Scheme.Open: %v", werr.ToError())
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
		return fmt.Errorf("Scheme.Rename: %v", werr.ToError())
	}

	return nil
}

func (s *service) Get(ctx context.Context, ID string, doc interface{}) error {
	if !document.IsEncodeable(doc) {
		return errors.New("invalid document argument")
	}
	f, werr := s.scheme.Open(s.getFileName(ID), os.O_RDONLY, 0)
	if werr != nil {
		if werr.IsNotExist {
			return document.ErrNotFound
		}
		return fmt.Errorf("Scheme.Open: %v", werr.ToError())
	}
	defer f.Close()

	return s.read(f, doc)
}

func (s *service) Delete(ctx context.Context, ID string) error {
	return s.scheme.Remove(s.getFileName(ID))
}

func (s *service) List(ctx context.Context, filters []document.Filter) (document.Iterator, error) {
	for _, f := range filters {
		if len(f.FieldPath) == 0 || f.Op == "" {
			panic("invalid filter")
		}
	}
	it, err := s.scheme.ListFiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("Scheme.ListFiles: %v", err)
	}
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

func (s *service) read(f workspace.File, doc interface{}) error {
	data, err := ioutil.ReadAll(f)
	if err != nil {
		return fmt.Errorf("Scheme.Read: %v", err)
	}

	err = s.marshaler.Unmarshal(data, doc)
	if err != nil {
		return fmt.Errorf("Unmarshal %s: %v: %s", f.Name(), err, string(data))
	}
	return nil
}

func (s *service) getFileName(id string) string {
	return id
}

func (s *service) create(ctx context.Context, ID string, doc interface{}, openFlags int) error {
	if doc == nil {
		panic("invalid nil data argument to Create/Set")
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
		return fmt.Errorf("Scheme.Open: %v", werr.ToError())
	}

	doc = document.UpdateCreatedAtField(doc)

	var ret error
	if err := s.write(f, doc); err != nil {
		ret = multierr.Append(ret, err)
	}
	if cerr := f.Close(); cerr != nil {
		ret = multierr.Append(ret, cerr)
	}
	return ret
}

func (s *service) write(f workspace.File, doc interface{}) error {
	data, err := s.marshaler.Marshal(doc)
	if err != nil {
		return fmt.Errorf("Marshal: %v", err)
	}
	done, ok := s.wg.AddOne()
	if !ok {
		return ErrClosing
	}
	defer done()
	_, err = f.Write(data)
	if err != nil {
		return fmt.Errorf("Write: %v", err)
	}
	err = f.Sync()
	if err != nil {
		return fmt.Errorf("Sync: %v", err)
	}
	return nil
}

type docIter struct {
	filters []document.Filter
	svc     *service
	it      iterator.Iterator[string]

	doneErr       error
	nextMatchFile workspace.File
}

func (d *docIter) HasNext() (ok bool) {
	for {
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

		if matchesAllFilters(proto, d.filters) {
			d.nextMatchFile = f
			return true
		}

		if cerr := f.Close(); cerr != nil {
			d.doneErr = cerr
			return true
		}

		/* continue */
	}
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
	defer d.nextMatchFile.Close()
	_, err := d.nextMatchFile.Seek(0, 0)
	if err != nil {
		return fmt.Errorf("Seek: %v", err)
	}
	return d.svc.read(d.nextMatchFile, doc)
}

func matchesAllFilters(proto map[string]interface{}, filters []document.Filter) bool {
	for _, f := range filters {
		if !document.MatchFilter(proto, f) {
			return false
		}
	}
	return true
}

func (d *docIter) Close() error {
	return nil
}
