package debug

import (
	"fmt"
	"io/ioutil"

	"github.com/ernestrc/blue/debug"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// CapturePanicReportDir captures a panic in run and writes a yaml report for the given pkg and
// version to the given dir. See debug.CapturePanic for more details.
func CapturePanicReportDir(dir, pkg, version string, run func()) (ok bool, path string, err error) {
	ok, report := debug.CapturePanic(log.StandardLogger(), pkg, version, run)
	if ok {
		return ok, "", nil
	}
	data, err := yaml.Marshal(report)
	if err != nil {
		return false, "", fmt.Errorf("marshal %v: %v", report, err)
	}
	f, err := ioutil.TempFile(dir, fmt.Sprintf("%s_crash_report_", pkg))
	if err != nil {
		return false, "", fmt.Errorf("temp file %v: %v", report, err)
	}
	_, err = f.Write(data)
	if err != nil {
		return false, "", fmt.Errorf("write %v: %v", report, err)
	}
	return false, f.Name(), nil
}
