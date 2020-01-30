package plugin

import (
	"fmt"
)

const (
	envLogLevel = "FRACTAL_PLUGIN_LOG_LEVEL"
)

var pluginEnv = []string{}

func makeEnvVar(k, v string) string {
	return fmt.Sprintf("%s=%s", k, v)
}
