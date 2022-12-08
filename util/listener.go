package util

import (
	"io/ioutil"
	"net"
	"os"
	"strings"
)

// TempUnixListener creates a temp file and exposes it
// as a unix domain sockets net.Listener.
func TempUnixListener() (net.Listener, error) {
	return TempUnixListenerTags("plugin")
}

// TempUnixListenerTags creates a temp file with the given tags
// and exposes it as a unix dmain socket net.Listener.
func TempUnixListenerTags(tags ...string) (net.Listener, error) {
	tf, err := ioutil.TempFile("", strings.Join(tags, "_"))
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
