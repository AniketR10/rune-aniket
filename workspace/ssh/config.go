package ssh

import (
	"fmt"
	"time"

	multierr "github.com/ernestrc/go-multierror"
	"unstable.build/go-tui/api/config"
)

const (
	defSSHTimeout = 5 * time.Second
)

type sshConfig struct {
	privateKeys []string
	timeout     time.Duration
	command     string
	shell       string
	insecure    bool
}

func fromConfig(cfg config.Config) (ret sshConfig, retErr error) {
	timeout, err := getTimeout(cfg)
	if err != nil && err != config.ErrNotFound {
		retErr = multierr.Append(retErr, err)
	}
	command, err := getCommand(cfg)
	if err != nil && err != config.ErrNotFound {
		retErr = multierr.Append(retErr, err)
	}
	shell, err := getShell(cfg)
	if err != nil && err != config.ErrNotFound {
		retErr = multierr.Append(retErr, err)
	}
	privateKeys, err := getPrivateKeys(cfg)
	if err != nil && err != config.ErrNotFound {
		retErr = multierr.Append(retErr, err)
	}
	insecure, err := getInsecure(cfg)
	if err != nil && err != config.ErrNotFound {
		retErr = multierr.Append(retErr, err)
	}
	if retErr != nil {
		retErr = fmt.Errorf("could not load ssh config: %s", retErr)
		return
	}

	ret.timeout = timeout
	ret.command = command
	ret.privateKeys = privateKeys
	ret.shell = shell
	ret.insecure = insecure
	return
}

func getTimeout(cfg config.Config) (ret time.Duration, err error) {
	ret = defSSHTimeout

	sshTimeout, err := config.GetDuration(cfg, "timeout", defSSHTimeout)
	if err != nil {
		return
	}

	ret = sshTimeout
	return
}

func getInsecure(cfg config.Config) (ret bool, err error) {
	ret, err = cfg.GetBool("insecure")
	return
}

func getCommand(cfg config.Config) (string, error) {
	cmd, err := cfg.GetString("command")
	if err != nil {
		return "", err
	}

	return cmd, nil
}

func getShell(cfg config.Config) (string, error) {
	cmd, err := cfg.GetString("shell")
	if err != nil {
		return "", err
	}

	return cmd, nil
}

func getPrivateKeys(cfg config.Config) (ret []string, err error) {
	keyIfcs, err := cfg.GetSlice("private_keys")
	if err != nil {
		return nil, err
	}

	for _, keyIfc := range keyIfcs {
		key, ok := keyIfc.(string)
		if !ok {
			err = multierr.Append(err, fmt.Errorf("slice of strings expected for 'private_keys' but found %v", key))
			continue
		}
		ret = append(ret, key)
	}
	if err != nil {
		return nil, err
	}
	return ret, err
}
