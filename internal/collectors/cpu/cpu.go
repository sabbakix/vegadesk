// Package cpu reads CPU utilization from /proc/stat. Parsing is kept
// separate from collection (ParseStat vs Collect) so it can be unit tested
// against a fixture without a live /proc filesystem.
package cpu

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// Times holds one line of /proc/stat in jiffies (clock ticks since boot).
type Times struct {
	User, Nice, System, Idle, Iowait, IRQ, SoftIRQ, Steal uint64
}

// Total is the sum of all accounted time for this CPU line.
func (t Times) Total() uint64 {
	return t.User + t.Nice + t.System + t.Idle + t.Iowait + t.IRQ + t.SoftIRQ + t.Steal
}

// Busy is Total minus idle and iowait -- the conventional "not idle" measure.
func (t Times) Busy() uint64 {
	return t.Total() - t.Idle - t.Iowait
}

// Sample is one snapshot of /proc/stat: the aggregate "cpu" line plus each
// individual core's line, in file order (cpu0, cpu1, ...).
type Sample struct {
	Total Times
	Cores []Times
}

// ParseStat parses /proc/stat content. Only "cpu"-prefixed lines are used;
// the rest of /proc/stat (interrupts, context switches, ...) is ignored.
func ParseStat(r io.Reader) (Sample, error) {
	var s Sample
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "cpu") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}
		t, err := parseTimesFields(fields[1:9])
		if err != nil {
			return Sample{}, fmt.Errorf("cpu: parsing %q: %w", line, err)
		}
		if fields[0] == "cpu" {
			s.Total = t
		} else {
			s.Cores = append(s.Cores, t)
		}
	}
	if err := sc.Err(); err != nil {
		return Sample{}, err
	}
	return s, nil
}

func parseTimesFields(f []string) (Times, error) {
	vals := make([]uint64, len(f))
	for i, s := range f {
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return Times{}, err
		}
		vals[i] = v
	}
	return Times{
		User: vals[0], Nice: vals[1], System: vals[2], Idle: vals[3],
		Iowait: vals[4], IRQ: vals[5], SoftIRQ: vals[6], Steal: vals[7],
	}, nil
}

// Collect reads /proc/stat and parses it.
func Collect(ctx context.Context) (Sample, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return Sample{}, err
	}
	defer f.Close()
	return ParseStat(f)
}

// UsagePercent computes utilization between two samples of the same CPU
// line (aggregate or a single core). Callers hold the previous Times and
// feed the new one each refresh tick.
func UsagePercent(prev, cur Times) float64 {
	totalDelta := float64(cur.Total()) - float64(prev.Total())
	if totalDelta <= 0 {
		return 0
	}
	busyDelta := float64(cur.Busy()) - float64(prev.Busy())
	if busyDelta < 0 {
		busyDelta = 0
	}
	return busyDelta / totalDelta * 100
}
