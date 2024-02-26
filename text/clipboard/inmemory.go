package clipboard

import "sync/atomic"

type inmemoryRegister struct {
	data atomic.Value
}

// NewInMemory returns a simple in-memory implementation of Register.
func NewInMemory() Register {
	ret := new(inmemoryRegister)
	// initialize atomic so below we can simply to a type-assertion
	// without further branches.
	ret.data.Store(Data{})
	return ret
}

func (d *inmemoryRegister) Paste(id string) (ret Data, err error) {
	return d.data.Load().(Data), nil
}

func (d *inmemoryRegister) Copy(id string, data Data) error {
	d.data.Store(data)
	return nil
}
