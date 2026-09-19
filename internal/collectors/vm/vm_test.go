package vm

import (
	"strings"
	"testing"
)

// virshListFixture is the documented `virsh list --all` table format
// (see the package doc comment for why this is fixture-only).
const virshListFixture = ` Id   Name                State
----------------------------------
 1    web01               running
 -    db01                shut off
 3    build-runner         in shutdown
`

func TestParseVirshList(t *testing.T) {
	domains, err := ParseVirshList(strings.NewReader(virshListFixture))
	if err != nil {
		t.Fatalf("ParseVirshList: %v", err)
	}
	if len(domains) != 3 {
		t.Fatalf("got %d domains, want 3: %+v", len(domains), domains)
	}
	if domains[0].ID != "1" || domains[0].Name != "web01" || domains[0].State != "running" || !domains[0].Running() {
		t.Errorf("web01 = %+v", domains[0])
	}
	if domains[1].ID != "-" || domains[1].State != "shut off" || domains[1].Running() {
		t.Errorf("db01 = %+v", domains[1])
	}
	if domains[2].State != "in shutdown" {
		t.Errorf("build-runner state = %q, want multi-word state parsed whole", domains[2].State)
	}
}

func TestParseVirshListEmpty(t *testing.T) {
	const empty = ` Id   Name   State
--------------------
`
	domains, err := ParseVirshList(strings.NewReader(empty))
	if err != nil {
		t.Fatalf("ParseVirshList: %v", err)
	}
	if len(domains) != 0 {
		t.Errorf("got %d domains, want 0: %+v", len(domains), domains)
	}
}
