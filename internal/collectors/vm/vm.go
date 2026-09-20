// Package vm lists libvirt domains via `virsh list --all`.
//
// Unlike systemctl/docker/lsblk, virsh has no JSON output mode, so this is
// the one collector in vegadesk that parses a human-formatted table rather
// than JSON. libvirt isn't installed on the machine this was built against,
// so this parser is verified against a captured/documented sample (see
// vm_test.go) rather than a live virsh -- flagged explicitly per the plan's
// verification section as fixture-only, not live-tested.
package vm

import (
	"bufio"
	"context"
	"io"
	"strings"

	"github.com/sabbakix/vegadesk/internal/execx"
)

// Domain is one libvirt domain (VM) as reported by `virsh list --all`.
type Domain struct {
	ID    string // numeric when running, "-" when inactive
	Name  string
	State string // running, shut off, paused, in shutdown, crashed, pmsuspended, ...
}

// Running reports whether the domain is currently active.
func (d Domain) Running() bool { return d.State == "running" }

// ParseVirshList parses `virsh list --all` output:
//
//	 Id   Name                State
//	----------------------------------
//	 1    web01               running
//	 -    db01                shut off
//
// State can itself contain spaces ("shut off", "in shutdown"), so it's
// everything after the Name field rather than a single whitespace-split
// token.
func ParseVirshList(r io.Reader) ([]Domain, error) {
	var out []Domain
	sc := bufio.NewScanner(r)
	seenHeader := false
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if !seenHeader {
			if strings.HasPrefix(line, "Id") {
				seenHeader = true
			}
			continue
		}
		if strings.HasPrefix(line, "---") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		out = append(out, Domain{ID: fields[0], Name: fields[1], State: strings.Join(fields[2:], " ")})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// Collect runs `virsh list --all` and parses it.
func Collect(ctx context.Context) ([]Domain, error) {
	out, err := execx.Run(ctx, "virsh", "list", "--all")
	if err != nil {
		return nil, err
	}
	return ParseVirshList(strings.NewReader(out))
}
