package cpu

import (
	"strings"
	"testing"
)

const statFixture = `cpu  100 10 50 800 20 0 5 0 0 0
cpu0 50 5 25 400 10 0 2 0 0 0
cpu1 50 5 25 400 10 0 3 0 0 0
intr 12345 0 0 0
ctxt 98765
btime 1700000000
processes 4321
`

func TestParseStat(t *testing.T) {
	s, err := ParseStat(strings.NewReader(statFixture))
	if err != nil {
		t.Fatalf("ParseStat: %v", err)
	}
	want := Times{User: 100, Nice: 10, System: 50, Idle: 800, Iowait: 20, SoftIRQ: 5}
	if s.Total != want {
		t.Errorf("Total = %+v, want %+v", s.Total, want)
	}
	if len(s.Cores) != 2 {
		t.Fatalf("Cores = %d, want 2", len(s.Cores))
	}
	if s.Cores[0].User != 50 || s.Cores[1].SoftIRQ != 3 {
		t.Errorf("core parsing wrong: %+v", s.Cores)
	}
}

func TestUsagePercent(t *testing.T) {
	prev := Times{User: 100, Idle: 900} // total 1000, busy 100
	cur := Times{User: 150, Idle: 950}  // total 1100, busy 150
	got := UsagePercent(prev, cur)
	want := 50.0 // busyDelta=50, totalDelta=100 -> 50%
	if got != want {
		t.Errorf("UsagePercent = %v, want %v", got, want)
	}
}

func TestUsagePercentNoElapsed(t *testing.T) {
	t1 := Times{User: 100, Idle: 900}
	if got := UsagePercent(t1, t1); got != 0 {
		t.Errorf("UsagePercent with no delta = %v, want 0", got)
	}
}
