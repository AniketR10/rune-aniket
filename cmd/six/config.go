package main

import (
	"fmt"
	"os"

	"github.com/ernestrc/go-tui/term"
	yaml "gopkg.in/yaml.v3"
)

const (
	outputNormal    = "normal"
	output256       = "color_256"
	outputGrayscale = "grayscale"
	inputEsc        = "esc"
	inputAlt        = "alt"
	inputMouse      = "mouse"
)

func strToOutput(str string) term.OutputMode {
	switch str {
	case outputNormal:
		return term.OutputNormal
	case output256:
		return term.Output256
	case outputGrayscale:
		return term.OutputGrayscale
	default:
		return term.OutputCurrent
	}
}

func strArrayToInput(strs []string) (ret term.InputMode) {
	for _, str := range strs {
		i := strToInput(str)
		ret |= i
	}
	return
}

func strToInput(str string) term.InputMode {
	switch str {
	case inputEsc:
		return term.InputEsc
	case inputAlt:
		return term.InputAlt
	case inputMouse:
		return term.InputMouse
	default:
		return term.InputCurrent
	}
}

func defaultConfig() Config {
	return Config{
		OutputMode: outputNormal,
		InputMode:  []string{inputEsc},
		Browser: BrowserConfig{
			Tabspaces: 4,
			SwapDir:   "",
		},
	}
}

type BrowserConfig struct {
	Tabspaces int
	SwapDir   string `yaml:"swap_dir"`
	StartText string `yaml:"start_text"`
}

type PluginConfig struct {
	Path   string
	Config map[string]interface{}
}

type Config struct {
	LogLevel      string                  `yaml:"log_level"`
	LogOutputPath string                  `yaml:"log_path"`
	OutputMode    string                  `yaml:"output_mode"`
	InputMode     []string                `yaml:"input_mode"`
	Plugins       map[string]PluginConfig `yaml:"plugins"`
	Browser       BrowserConfig           `yaml:"browser"`
}

func loadYamlConfig(configpath string) (Config, error) {
	// TODO initialize if configpath is not found
	f, err := os.Open(configpath)
	if err != nil {
		return Config{}, fmt.Errorf("could not open config file: %v", err)
	}

	cfg := defaultConfig()
	d := yaml.NewDecoder(f)

	err = d.Decode(&cfg)
	if err != nil {
		return Config{}, fmt.Errorf("could not decode config file: %v", err)
	}

	return cfg, nil
}
