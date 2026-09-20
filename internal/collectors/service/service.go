// Package service lists systemd service units via `systemctl ... --output=json`.
//
// This requires a systemd new enough to support JSON output for list-units
// and list-unit-files (roughly systemd 245+, i.e. most distros shipped since
// 2020). Older systemd versions only emit systemctl's table/plain formats,
// which this package does not parse; on such a host the Services section
// will simply fail to collect. Given every other subsystem in vegadesk
// already assumes a reasonably current distro, this is treated as an
// acceptable v1 limitation rather than a second parser to maintain.
package service

import (
	"context"
	"encoding/json"
	"io"
	"strings"

	"github.com/sabbakix/vegadesk/internal/execx"
)

// Unit is one service unit's runtime state plus (if resolvable) its
// enabled/disabled state from list-unit-files.
type Unit struct {
	Name        string `json:"unit"`
	Load        string `json:"load"`   // loaded, not-found, masked, ...
	Active      string `json:"active"` // active, inactive, failed, ...
	Sub         string `json:"sub"`    // running, exited, dead, failed, ...
	Description string `json:"description"`
	Enabled     string // enabled, disabled, static, masked, "" if unknown
}

// ParseListUnits parses `systemctl list-units --type=service --all --output=json`.
func ParseListUnits(r io.Reader) ([]Unit, error) {
	var units []Unit
	if err := json.NewDecoder(r).Decode(&units); err != nil {
		return nil, err
	}
	return units, nil
}

type unitFile struct {
	UnitFile string `json:"unit_file"`
	State    string `json:"state"`
}

// ParseListUnitFiles parses `systemctl list-unit-files --type=service --output=json`
// into a map from unit name to its enabled-state string.
func ParseListUnitFiles(r io.Reader) (map[string]string, error) {
	var files []unitFile
	if err := json.NewDecoder(r).Decode(&files); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(files))
	for _, f := range files {
		out[f.UnitFile] = f.State
	}
	return out, nil
}

// Collect lists every service unit systemd currently knows about (loaded or
// not) and merges in each one's enabled-state from list-unit-files.
func Collect(ctx context.Context) ([]Unit, error) {
	unitsOut, err := execx.Run(ctx, "systemctl", "list-units", "--type=service", "--all", "--output=json", "--no-pager")
	if err != nil {
		return nil, err
	}
	units, err := ParseListUnits(strings.NewReader(unitsOut))
	if err != nil {
		return nil, err
	}

	filesOut, err := execx.Run(ctx, "systemctl", "list-unit-files", "--type=service", "--output=json", "--no-pager")
	if err != nil {
		// enabled-state is a nice-to-have; don't fail the whole collection
		// over it (e.g. some sandboxed/container systemd setups restrict it).
		return units, nil
	}
	states, err := ParseListUnitFiles(strings.NewReader(filesOut))
	if err != nil {
		return units, nil
	}
	for i := range units {
		units[i].Enabled = states[units[i].Name]
	}
	return units, nil
}
