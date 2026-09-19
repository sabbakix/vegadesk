package service

import (
	"strings"
	"testing"
)

const listUnitsFixture = `[{"unit":"accounts-daemon.service","load":"loaded","active":"active","sub":"running","description":"Accounts Service"},{"unit":"anacron.service","load":"loaded","active":"inactive","sub":"dead","description":"Run anacron jobs"}]`

const listUnitFilesFixture = `[{"unit_file":"accounts-daemon.service","state":"enabled","preset":"enabled"},{"unit_file":"anacron.service","state":"static","preset":null}]`

func TestParseListUnits(t *testing.T) {
	units, err := ParseListUnits(strings.NewReader(listUnitsFixture))
	if err != nil {
		t.Fatalf("ParseListUnits: %v", err)
	}
	if len(units) != 2 {
		t.Fatalf("got %d units, want 2", len(units))
	}
	if units[0].Name != "accounts-daemon.service" || units[0].Active != "active" || units[0].Sub != "running" {
		t.Errorf("unit0 = %+v", units[0])
	}
}

func TestParseListUnitFiles(t *testing.T) {
	states, err := ParseListUnitFiles(strings.NewReader(listUnitFilesFixture))
	if err != nil {
		t.Fatalf("ParseListUnitFiles: %v", err)
	}
	if states["accounts-daemon.service"] != "enabled" {
		t.Errorf("accounts-daemon state = %q, want enabled", states["accounts-daemon.service"])
	}
	if states["anacron.service"] != "static" {
		t.Errorf("anacron state = %q, want static", states["anacron.service"])
	}
}
