package docker

import (
	"fmt"
	"io"
	"net"
	"path/filepath"

	. "github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
	"github.com/anchore/go-make/run"
)

type Option func(*commandConfig) error

// Flags are passed to the docker command itself, e.g. before the container name in `docker run <flags> <container>`
func Flags(args ...string) Option {
	return func(cfg *commandConfig) error {
		cfg.dockerArgs = append(cfg.dockerArgs, run.Args(args...))
		return nil
	}
}

// Args args are passed to the command being run, e.g. following the container name in `docker run <container> <args>`
func Args(args ...string) Option {
	return func(cfg *commandConfig) error {
		cfg.commandArgs = append(cfg.commandArgs, run.Args(args...))
		return nil
	}
}

func Stdout(writer io.Writer) Option {
	return func(cfg *commandConfig) error {
		cfg.dockerArgs = append(cfg.dockerArgs, run.Stdout(writer))
		return nil
	}
}

func Entrypoint(command string) Option {
	return Flags("--entrypoint", command)
}

func Envs(env map[string]string) Option {
	return func(cfg *commandConfig) error {
		for k, v := range env {
			err := Env(k, v)(cfg)
			if err != nil {
				return err
			}
		}
		return nil
	}
}

func Env(key, value string) Option {
	return func(cfg *commandConfig) error {
		cfg.dockerArgs = append(cfg.dockerArgs, run.Args("--env", fmt.Sprintf("%s=%s", key, value)))
		return nil
	}
}

func ExposeRandomPort(randomPort *int, containerPort int) Option {
	*randomPort = Return(unusedPort())
	return ExposePort(*randomPort, containerPort)
}

func ExposePort(localPort, containerPort int) Option {
	return Flags("-p", fmt.Sprintf("%d:%d", localPort, containerPort))
}

func MountVolume(local, container string) Option {
	local = Return(filepath.Abs(local))
	return Flags("-v", fmt.Sprintf("%s:%s", local, container))
}

// InDir runs docker in the specific directory, also mounting it to DefaultContainerDir or containerDir, if provided
func InDir(localDir string, containerDir ...string) Option {
	return func(cfg *commandConfig) error {
		d := DefaultContainerDir
		if len(containerDir) > 0 {
			d = containerDir[0]
		}
		cfg.dockerArgs = append(cfg.dockerArgs, run.InDir(localDir), run.Args("--workdir", d))
		return MountVolume(localDir, d)(cfg)
	}
}

func unusedPort() (int, error) {
	addr, err := net.Listen("tcp", ":0") //nolint:gosec
	if err != nil {
		return 0, err
	}
	defer func() {
		log.Error(addr.Close())
	}()

	return addr.Addr().(*net.TCPAddr).Port, nil
}
