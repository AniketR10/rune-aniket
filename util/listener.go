package util

import (
	"io/ioutil"
	"net"
	"os"
	"strings"
)

// TempUnixListenerTags creates a temp file with the given tags
// and exposes it as a unix dmain socket net.Listener.
func TempUnixListenerTags(dir string, tags ...string) (net.Listener, error) {
	tf, err := ioutil.TempFile(dir, strings.Join(tags, "_"))
	if err != nil {
		return nil, err
	}
	path := tf.Name()

	// Close the file and remove it because it has to not exist for
	// the domain socket.
	if err := tf.Close(); err != nil {
		return nil, err
	}
	if err := os.Remove(path); err != nil {
		return nil, err
	}

	l, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}

	return l, nil
}
