package containers

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"

	"github.com/sabbakix/vegadesk/internal/execx"
)

type dockerRuntime struct{}

func (dockerRuntime) Name() string { return "docker" }

type dockerPSLine struct {
	ID     string `json:"ID"`
	Image  string `json:"Image"`
	Names  string `json:"Names"`
	State  string `json:"State"`
	Status string `json:"Status"`
}

// ParseDockerPS parses `docker ps -a --format json`, which -- unlike
// Podman's equivalent -- emits one JSON object per line rather than a
// single array.
func ParseDockerPS(r io.Reader) ([]Container, error) {
	var out []Container
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20) // container labels can be long
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var d dockerPSLine
		if err := json.Unmarshal([]byte(line), &d); err != nil {
			return nil, err
		}
		out = append(out, Container{
			ID: d.ID, Image: d.Image, Name: d.Names, State: d.State, Status: d.Status,
		})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (dockerRuntime) List(ctx context.Context) ([]Container, error) {
	out, err := execx.Run(ctx, "docker", "ps", "-a", "--format", "json")
	if err != nil {
		return nil, err
	}
	return ParseDockerPS(strings.NewReader(out))
}

func (dockerRuntime) Start(ctx context.Context, id string) error {
	_, err := execx.Run(ctx, "docker", "start", id)
	return err
}

func (dockerRuntime) Stop(ctx context.Context, id string) error {
	_, err := execx.Run(ctx, "docker", "stop", id)
	return err
}

func (dockerRuntime) Remove(ctx context.Context, id string) error {
	_, err := execx.Run(ctx, "docker", "rm", id)
	return err
}

func (dockerRuntime) LogsArgv(id string) []string {
	return []string{"docker", "logs", "-f", "--tail", "200", id}
}

func (dockerRuntime) ExecArgv(id, shell string) []string {
	return []string{"docker", "exec", "-it", id, shell}
}
