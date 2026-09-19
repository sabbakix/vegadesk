package containers

import (
	"os"
	"testing"
)

func TestParseDockerPS(t *testing.T) {
	f, err := os.Open("testdata/docker_ps.ndjson")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	got, err := ParseDockerPS(f)
	if err != nil {
		t.Fatalf("ParseDockerPS: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d containers, want 2: %+v", len(got), got)
	}
	if got[1].ID != "470059e8eec5" || got[1].Name != "portainer" || got[1].State != "running" {
		t.Errorf("container1 = %+v", got[1])
	}
}

func TestParsePodmanPS(t *testing.T) {
	f, err := os.Open("testdata/podman_ps.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	got, err := ParsePodmanPS(f)
	if err != nil {
		t.Fatalf("ParsePodmanPS: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d containers, want 2: %+v", len(got), got)
	}
	if got[0].ID != "a1b2c3d4e5f6" || got[0].Name != "my-alpine" || got[0].State != "running" {
		t.Errorf("container0 = %+v", got[0])
	}
	if got[1].Name != "web" || got[1].State != "exited" {
		t.Errorf("container1 = %+v", got[1])
	}
}

func TestDetectPrefersDocker(t *testing.T) {
	if rt := Detect(true, true); rt == nil || rt.Name() != "docker" {
		t.Errorf("Detect(true,true) = %v, want docker", rt)
	}
	if rt := Detect(false, true); rt == nil || rt.Name() != "podman" {
		t.Errorf("Detect(false,true) = %v, want podman", rt)
	}
	if rt := Detect(false, false); rt != nil {
		t.Errorf("Detect(false,false) = %v, want nil", rt)
	}
}
