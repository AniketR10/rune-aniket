package system

import (
	"errors"
	"sync/atomic"

	sysclip "github.com/atotto/clipboard"
	"unstable.build/go-tui/text/clipboard"
)

type register struct {
	data atomic.Value
}

type registerData struct {
	metadata interface{}
	data     string
}

// NewRegister allocates initializes a new system clipboard.
func NewRegister() (clipboard.Register, error) {
	if sysclip.Unsupported {
		return nil, errors.New("system clipboard unsupported")
	}
	ret := new(register)
	ret.data.Store(registerData{})
	return ret, nil
}

// Paste satisfies clipboard.Register.
func (r *register) Paste(id string) (ret clipboard.Data, err error) {
	text, err := sysclip.ReadAll()
	if err != nil {
		return ret, err
	}
	// only return metadata if it matches last copy
	ret.Text = text
	data := r.data.Load().(registerData)
	if data.data != text {
		return
	}
	ret.Metadata = data.metadata
	return
}

// Copy satisfies clipboard.Register.
func (r *register) Copy(id string, data clipboard.Data) error {
	r.data.Store(registerData{data: data.Text, metadata: data.Metadata})
	return sysclip.WriteAll(data.Text)
}
