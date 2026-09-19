package mem

import (
	"strings"
	"testing"
)

const meminfoFixture = `MemTotal:       16384000 kB
MemFree:         2048000 kB
MemAvailable:    8192000 kB
Buffers:          512000 kB
Cached:          4096000 kB
SwapTotal:       2097148 kB
SwapFree:        1048574 kB
Dirty:               128 kB
`

func TestParseMeminfo(t *testing.T) {
	info, err := ParseMeminfo(strings.NewReader(meminfoFixture))
	if err != nil {
		t.Fatalf("ParseMeminfo: %v", err)
	}
	if info.TotalKB != 16384000 || info.AvailableKB != 8192000 || info.SwapTotalKB != 2097148 {
		t.Fatalf("parsed = %+v", info)
	}
	wantUsed := uint64(16384000 - 8192000)
	if info.UsedKB() != wantUsed {
		t.Errorf("UsedKB = %d, want %d", info.UsedKB(), wantUsed)
	}
	wantPct := float64(wantUsed) / 16384000 * 100
	if info.UsedPercent() != wantPct {
		t.Errorf("UsedPercent = %v, want %v", info.UsedPercent(), wantPct)
	}
}

func TestParseMeminfoNoAvailableFallsBack(t *testing.T) {
	const noAvail = `MemTotal:       1000 kB
MemFree:         100 kB
Buffers:          50 kB
Cached:          150 kB
`
	info, err := ParseMeminfo(strings.NewReader(noAvail))
	if err != nil {
		t.Fatalf("ParseMeminfo: %v", err)
	}
	if info.AvailableKB != 300 { // free+buffers+cached fallback
		t.Errorf("AvailableKB fallback = %d, want 300", info.AvailableKB)
	}
}
