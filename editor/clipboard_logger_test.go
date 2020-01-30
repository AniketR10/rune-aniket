package editor

import (
	"io/ioutil"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestLoggingClipboard(t *testing.T) {
	c := NewEphemeralClipboard()
	logger := logrus.New()
	logger.SetOutput(ioutil.Discard)
	logger.SetLevel(logrus.TraceLevel)

	c = NewLoggingClipboard(logger, c)

	testClipboard(t, c)
}
