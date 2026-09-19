// Package netif reads network interface addresses and state via `ip -j addr`
// (iproute2's JSON output, supported since ~2018 on any distro's ip binary
// modern enough to matter). Throughput uses internal/collectors/net's
// /proc/net/dev byte counters; this package only covers what `ip` uniquely
// provides: addresses, link state, MTU.
package netif

import (
	"context"
	"encoding/json"
	"io"
	"strings"

	"vegadesk/internal/execx"
)

// AddrInfo is one address assigned to an interface.
type AddrInfo struct {
	Family    string `json:"family"` // inet, inet6
	Local     string `json:"local"`
	PrefixLen int    `json:"prefixlen"`
}

// Iface is one network interface as reported by `ip -j addr`.
type Iface struct {
	Name      string     `json:"ifname"`
	Flags     []string   `json:"flags"`
	MTU       int        `json:"mtu"`
	OperState string     `json:"operstate"` // UP, DOWN, UNKNOWN, ...
	Address   string     `json:"address"`   // MAC, when applicable
	AddrInfo  []AddrInfo `json:"addr_info"`
}

// IPv4 returns the first inet address, or "" if none.
func (i Iface) IPv4() string {
	for _, a := range i.AddrInfo {
		if a.Family == "inet" {
			return a.Local
		}
	}
	return ""
}

// IsUp reports whether the interface is administratively and physically up.
func (i Iface) IsUp() bool { return i.OperState == "UP" }

// ParseIPAddrJSON parses `ip -j addr` output.
func ParseIPAddrJSON(r io.Reader) ([]Iface, error) {
	var out []Iface
	if err := json.NewDecoder(r).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// Collect runs `ip -j addr` and parses it.
func Collect(ctx context.Context) ([]Iface, error) {
	out, err := execx.Run(ctx, "ip", "-j", "addr")
	if err != nil {
		return nil, err
	}
	return ParseIPAddrJSON(strings.NewReader(out))
}
