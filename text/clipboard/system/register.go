package system

import (
	"errors"

	sysclip "github.com/atotto/clipboard"
	"unstable.build/go-tui/text/clipboard"
)

type systemRegister struct {
	metadata interface{}
	data     string
}

func NewRegister() (clipboard.Register, error) {
	if sysclip.Unsupported {
		return nil, errors.New("system clipboard unsupported")
	}
	return &systemRegister{}, nil
}

func (r *systemRegister) Paste(id string) (ret clipboard.Data, err error) {
	text, err := sysclip.ReadAll()
	if err != nil {
		return ret, err
	}
	// only return metadata if it matches last copy
	ret.Text = text
	if r.data != text {
		return
	}
	ret.Metadata = r.metadata
	return
}

func (r *systemRegister) Copy(id string, data clipboard.Data) error {
	r.data = data.Text
	r.metadata = data.Metadata
	return sysclip.WriteAll(data.Text)
}
