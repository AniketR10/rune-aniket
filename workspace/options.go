package workspace

import "time"

// Option is an option to configure Manager.
type Option func(c *managerCfg)

// WithSSHPrivateKey sets a Manager's private key when
// attempting to connect to a remote workspace via ssh.
func WithSSHPrivateKey(key string) Option {
	return func(c *managerCfg) {
		c.sshPrivateKeys = append(c.sshPrivateKeys, key)
	}
}

// WithSSHTimeout sets a Manager's connect timeout when
// attempting to connect to a remote workspace via ssh.
func WithSSHTimeout(t time.Duration) Option {
	return func(c *managerCfg) {
		c.sshTimeout = t
	}
}

// WithSSHCommand sets a Manager's ssh command to use
// rather than using Go's ssh library. This is useful
// when advanced openSSH features like ProxyCommand are needed
// to connect to a remote workspace.
func WithSSHCommand(cmd string) Option {
	return func(c *managerCfg) {
		c.sshCommand = cmd
	}
}
