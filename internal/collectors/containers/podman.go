package containers

import (
	"context"
	"encoding/json"
	"io"
	"strings"

	"vegadesk/internal/execx"
)

type podmanRuntime struct{}

func (podmanRuntime) Name() string { return "podman" }

type podmanPSEntry struct {
	ID     string   `json:"Id"`
	Image  string   `json:"Image"`
	Names  []string `json:"Names"`
	State  string   `json:"State"`
	Status string   `json:"Status"`
}

// ParsePodmanPS parses `podman ps -a --format json`, which -- unlike
// Docker's equivalent -- emits a single JSON array, and gives Names as an
// array rather than a comma-joined string.
func ParsePodmanPS(r io.Reader) ([]Container, error) {
	var entries []podmanPSEntry
	if err := json.NewDecoder(r).Decode(&entries); err != nil {
		return nil, err
	}
	out := make([]Container, len(entries))
	for i, e := range entries {
		name := ""
		if len(e.Names) > 0 {
			name = e.Names[0]
		}
		out[i] = Container{ID: e.ID, Image: e.Image, Name: name, State: e.State, Status: e.Status}
	}
	return out, nil
}

func (podmanRuntime) List(ctx context.Context) ([]Container, error) {
	out, err := execx.Run(ctx, "podman", "ps", "-a", "--format", "json")
	if err != nil {
		return nil, err
	}
	return ParsePodmanPS(strings.NewReader(out))
}

func (podmanRuntime) Start(ctx context.Context, id string) error {
	_, err := execx.Run(ctx, "podman", "start", id)
	return err
}

func (podmanRuntime) Stop(ctx context.Context, id string) error {
	_, err := execx.Run(ctx, "podman", "stop", id)
	return err
}

func (podmanRuntime) Remove(ctx context.Context, id string) error {
	_, err := execx.Run(ctx, "podman", "rm", id)
	return err
}

func (podmanRuntime) LogsArgv(id string) []string {
	return []string{"podman", "logs", "-f", "--tail", "200", id}
}

func (podmanRuntime) ExecArgv(id, shell string) []string {
	return []string{"podman", "exec", "-it", id, shell}
}
