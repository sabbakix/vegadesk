package netif

import (
	"strings"
	"testing"
)

const ipAddrFixture = `[
  {
    "ifindex": 1,
    "ifname": "lo",
    "flags": ["LOOPBACK","UP","LOWER_UP"],
    "mtu": 65536,
    "operstate": "UNKNOWN",
    "address": "00:00:00:00:00:00",
    "addr_info": [
      {"family":"inet","local":"127.0.0.1","prefixlen":8,"scope":"host"}
    ]
  },
  {
    "ifindex": 2,
    "ifname": "enp6s18",
    "flags": ["BROADCAST","MULTICAST","UP","LOWER_UP"],
    "mtu": 1500,
    "operstate": "UP",
    "address": "bc:24:11:16:1c:19",
    "addr_info": [
      {"family":"inet","local":"192.168.0.188","prefixlen":24,"scope":"global"},
      {"family":"inet6","local":"fe80::be24:11ff:fe16:1c19","prefixlen":64,"scope":"link"}
    ]
  }
]`

func TestParseIPAddrJSON(t *testing.T) {
	ifaces, err := ParseIPAddrJSON(strings.NewReader(ipAddrFixture))
	if err != nil {
		t.Fatalf("ParseIPAddrJSON: %v", err)
	}
	if len(ifaces) != 2 {
		t.Fatalf("got %d ifaces, want 2", len(ifaces))
	}
	eth := ifaces[1]
	if eth.Name != "enp6s18" || !eth.IsUp() || eth.IPv4() != "192.168.0.188" {
		t.Errorf("enp6s18 = %+v", eth)
	}
	if ifaces[0].IsUp() {
		t.Errorf("lo operstate UNKNOWN should not report IsUp()")
	}
}
