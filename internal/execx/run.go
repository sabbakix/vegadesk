// Package execx wraps os/exec with a context timeout, captured output, and a
// typed error so every collector and feature action shells out the same way.
package execx

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// DefaultTimeout bounds any single shell-out. Collectors run on a refresh
// ticker; a hung external command must not freeze the UI.
const DefaultTimeout = 5 * time.Second

// Error wraps a failed external command with enough context to show the user
// or a test failure without re-running the command.
type Error struct {
	Cmd    string
	Args   []string
	Stderr string
	Err    error
}

func (e *Error) Error() string {
	stderr := strings.TrimSpace(e.Stderr)
	if stderr != "" {
		return fmt.Sprintf("%s %s: %v: %s", e.Cmd, strings.Join(e.Args, " "), e.Err, stderr)
	}
	return fmt.Sprintf("%s %s: %v", e.Cmd, strings.Join(e.Args, " "), e.Err)
}

func (e *Error) Unwrap() error { return e.Err }

// Run executes name with args under DefaultTimeout and returns trimmed stdout.
// Non-zero exit or timeout is returned as *Error with captured stderr.
func Run(ctx context.Context, name string, args ...string) (string, error) {
	return RunTimeout(ctx, DefaultTimeout, name, args...)
}

// RunTimeout is Run with an explicit timeout, for commands known to be slower
// or faster than the default (e.g. a quick `command -v` probe).
func RunTimeout(ctx context.Context, timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", &Error{Cmd: name, Args: args, Stderr: stderr.String(), Err: err}
	}
	return strings.TrimRight(stdout.String(), "\n"), nil
}

// LookPath reports whether name is found on PATH, without invoking it.
// Used by internal/platform to probe for optional subsystems.
func LookPath(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
