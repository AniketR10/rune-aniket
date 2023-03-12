package clipboard

type inmemoryRegister struct {
	data Data
}

// NewInMemory returns a simple in-memory implementation of Register.
func NewInMemory() Register {
	return &inmemoryRegister{}
}

func (d *inmemoryRegister) Paste(id string) (ret Data, err error) {
	return d.data, nil
}

func (d *inmemoryRegister) Copy(id string, data Data) error {
	d.data = data
	return nil
}
