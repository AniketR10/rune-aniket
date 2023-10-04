package process

type nopCloser struct {
}

func (n nopCloser) Close() error {
	return nil
}
