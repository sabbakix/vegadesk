// Package containers manages Docker or Podman containers through their
// respective CLIs. Both runtimes are driven the same way from the shell,
// but their `ps --format json` output shapes genuinely differ -- Docker
// emits newline-delimited JSON objects, Podman emits a single JSON array --
// so each gets its own parser (see docker.go, podman.go) rather than a
// shared "just json.Unmarshal it" helper that would only work for one.
package containers

import "context"

// Container is a runtime-agnostic view of one container, normalized from
// whichever of Docker/Podman's ps output produced it.
type Container struct {
	ID     string
	Image  string
	Name   string
	State  string // running, exited, created, paused, ...
	Status string // human-readable, e.g. "Up 2 hours" / "Exited (0) 3 months ago"
}

// Runtime abstracts over `docker` and `podman`; both CLIs are argument-
// compatible enough that only List's output parsing and the binary name
// actually differ.
type Runtime interface {
	Name() string // "docker" or "podman", for display and argv[0]
	List(ctx context.Context) ([]Container, error)
	Start(ctx context.Context, id string) error
	Stop(ctx context.Context, id string) error
	Remove(ctx context.Context, id string) error
	// LogsArgv/ExecArgv return argv for use with ui.Suspend, which needs a
	// real attached TTY for `-f` follow and `-it` exec.
	LogsArgv(id string) []string
	ExecArgv(id, shell string) []string
}

// Detect picks the preferred runtime the same way internal/platform does
// (docker over podman when both are present), or nil if neither is on PATH.
func Detect(hasDocker, hasPodman bool) Runtime {
	switch {
	case hasDocker:
		return dockerRuntime{}
	case hasPodman:
		return podmanRuntime{}
	default:
		return nil
	}
}
