package net

import (
	"strings"
	"testing"
)

const netDevFixture = `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo: 123456     100    0    0    0     0          0         0   123456     100    0    0    0     0       0          0
  eth0: 987654321  654321    0    0    0     0          0        12 123456789   98765    0    0    0     0       0          0
`

func TestParseNetDev(t *testing.T) {
	stats, err := ParseNetDev(strings.NewReader(netDevFixture))
	if err != nil {
		t.Fatalf("ParseNetDev: %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("got %d interfaces, want 2: %+v", len(stats), stats)
	}
	if stats[0].Name != "lo" || stats[0].RxBytes != 123456 {
		t.Errorf("lo = %+v", stats[0])
	}
	if stats[1].Name != "eth0" || stats[1].RxBytes != 987654321 || stats[1].TxBytes != 123456789 {
		t.Errorf("eth0 = %+v", stats[1])
	}
}

func TestRateBps(t *testing.T) {
	if got := RateBps(1000, 3000, 2); got != 1000 {
		t.Errorf("RateBps = %v, want 1000", got)
	}
	if got := RateBps(1000, 500, 2); got != 0 { // counter reset (e.g. interface flap)
		t.Errorf("RateBps on counter decrease = %v, want 0", got)
	}
}
