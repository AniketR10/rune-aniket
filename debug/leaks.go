package debug

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// WriteTagsFile writes the given file to a random file prefixed by DEBUG
// on the default os temp dir. It returns a cleanup function to remove
// the file.
func WriteTagsFile(tags ...string) func() {
	dir := os.TempDir()
	filename := "DEBUG" + strings.Join(append(tags, strconv.Itoa(rand.Int())), "_")
	filename = strings.ReplaceAll(filename, "/", "_")
	tempfile := filepath.Join(dir, filename)
	err := os.WriteFile(tempfile, []byte(fmt.Sprintf("%#v", tags)), 0777)
	if err != nil {
		panic(err)
	}
	return func() { os.Remove(tempfile) }
}
