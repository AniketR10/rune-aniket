package test

import (
	"context"
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/api/config"
	"unstable.build/go-tui/workspace"
	"unstable.build/go-tui/workspace/ssh"
	"unstable.build/go-tui/workspace/test"
)

// NOTE if this is failing or you are iterating on functionality
// used by SSH, remember to build_docker.sh before running these tests again.
func TestIntegrationScheme(t *testing.T) {
	hostname, teardown := runDockerOrSkip(t)
	t.Cleanup(func() { teardown() })

	cfgs := map[string]config.Config{
		"openssh_proc_remote": config.MapConfig(map[string]interface{}{
			"command": "ssh -o StrictHostKeyChecking=no -i ./id_ed25519 %h -p %p",
			"timeout": "20s",
		}),
		"go_stdlib_remote": config.MapConfig(map[string]interface{}{
			"private_keys": []interface{}{"./id_ed25519"},
			"timeout":      "20s",
			"insecure":     true,
		}),
	}
	for desc, cfg := range cfgs {
		cfg := cfg
		t.Run(desc, func(t *testing.T) {
			test.TestWorkspaceSchemeFiles(t, func(t *testing.T) workspace.Scheme {
				return newSchemeIntegration(t, hostname, cfg)
			})

			test.TestWorkspaceSchemeExecutor(t, func(t *testing.T) workspace.Scheme {
				return newSchemeIntegration(t, hostname, cfg)
			})
		})
	}
}

func teardownFn(container string) func() error {
	return func() error {
		cmd := exec.Cmd{
			Path: "stop_docker.sh",
			Args: []string{"stop_docker.sh", container},
		}
		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("stop_docker.sh %q", container)
		}
		return nil
	}
}

func runContainer() (string, func() error, error) {
	cmd := exec.Cmd{
		Path: "run_docker.sh",
		Dir:  ".",
	}
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", nil, err
	}
	errPipe, err := cmd.StderrPipe()
	if err != nil {
		return "", nil, err
	}

	err = cmd.Start()
	if err != nil {
		return "", nil, err
	}

	data, err := ioutil.ReadAll(pipe)
	errdata, _ := ioutil.ReadAll(errPipe)
	if err != nil {
		_ = cmd.Wait()
		err = fmt.Errorf("%v: %s", err, string(errdata))
		return "", nil, err
	}

	err = cmd.Wait()
	if err != nil {
		err = fmt.Errorf("%v: %s", err, string(errdata))
		return "", nil, err
	}

	// $CONTAINER:$IP:$PORT
	chunks := strings.Split(string(data), ":")
	if len(chunks) != 3 {
		panic("could not parse script stdout")
	}

	container := strings.Trim(chunks[0], "\n ")
	ip := strings.Trim(chunks[1], "\n ")
	port := strings.Trim(chunks[2], "\n ")

	if ip == "" || port == "" || container == "" {
		panic(fmt.Sprintf("missing one of ip, port or container: %q %q %q", ip, port, container))
	}

	return fmt.Sprintf("%s:%s", ip, port), teardownFn(container), nil
}

func runDockerOrSkip(t *testing.T) (string, func()) {
	hostname, teardown, err := runContainer()
	if err != nil {
		t.Logf("problem starting container, maybe you want to build_docker.sh first? skipping test: %s\n", err)
		t.SkipNow()
		return "", func() {}
	}
	return hostname, func() {
		err := teardown()
		if err != nil {
			t.Logf("error stopping container for %q: %s\n", hostname, err)
		}
	}
}

func newSchemeIntegration(
	t *testing.T, hostname string, cfg config.Config,
) workspace.Scheme {
	workspaceURI, err := workspaceapi.ParseURI("ssh://test@" + hostname + "/tmp")
	require.NoError(t, err)

	// emulate event loop holding mutex while we initialize ssh scheme
	var mu sync.Mutex
	ctx := workspace.ContextWithLocker(context.Background(), &mu)
	mu.Lock()

	s, err := ssh.New(ctx, cfg, workspaceURI)
	require.NoError(t, err)

	t.Cleanup(func() {
		s, err := ssh.New(ctx, cfg, workspaceURI)
		require.NoError(t, err)
		it, err := workspace.ListFiles(context.Background(), s, "/tmp")
		require.NoError(t, err)
		for {
			f, ok := it.Next()
			if !ok {
				break
			}
			require.NoError(t, s.Remove(f))
		}
		require.NoError(t, it.Err())
		s.Close()
	})
	return s
}
