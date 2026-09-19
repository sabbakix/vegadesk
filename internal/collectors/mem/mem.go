// Package mem reads memory/swap usage from /proc/meminfo.
package mem

import (
	"bufio"
	"context"
	"io"
	"os"
	"strconv"
	"strings"
)

// Info holds the /proc/meminfo fields vegadesk cares about, in kB as the
// kernel reports them.
type Info struct {
	TotalKB     uint64
	FreeKB      uint64
	AvailableKB uint64
	BuffersKB   uint64
	CachedKB    uint64
	SwapTotalKB uint64
	SwapFreeKB  uint64
}

// UsedKB follows btop's convention: total minus "available" (which already
// accounts for reclaimable cache/buffers), not total minus free.
func (i Info) UsedKB() uint64 {
	if i.AvailableKB > i.TotalKB {
		return 0
	}
	return i.TotalKB - i.AvailableKB
}

// UsedPercent is UsedKB as a percentage of TotalKB.
func (i Info) UsedPercent() float64 {
	if i.TotalKB == 0 {
		return 0
	}
	return float64(i.UsedKB()) / float64(i.TotalKB) * 100
}

// SwapUsedKB is SwapTotalKB minus SwapFreeKB.
func (i Info) SwapUsedKB() uint64 {
	if i.SwapFreeKB > i.SwapTotalKB {
		return 0
	}
	return i.SwapTotalKB - i.SwapFreeKB
}

// SwapUsedPercent is SwapUsedKB as a percentage of SwapTotalKB (0 if no swap).
func (i Info) SwapUsedPercent() float64 {
	if i.SwapTotalKB == 0 {
		return 0
	}
	return float64(i.SwapUsedKB()) / float64(i.SwapTotalKB) * 100
}

var fieldSetters = map[string]func(*Info, uint64){
	"MemTotal":     func(i *Info, v uint64) { i.TotalKB = v },
	"MemFree":      func(i *Info, v uint64) { i.FreeKB = v },
	"MemAvailable": func(i *Info, v uint64) { i.AvailableKB = v },
	"Buffers":      func(i *Info, v uint64) { i.BuffersKB = v },
	"Cached":       func(i *Info, v uint64) { i.CachedKB = v },
	"SwapTotal":    func(i *Info, v uint64) { i.SwapTotalKB = v },
	"SwapFree":     func(i *Info, v uint64) { i.SwapFreeKB = v },
}

// ParseMeminfo parses /proc/meminfo content ("Key:   12345 kB" lines).
func ParseMeminfo(r io.Reader) (Info, error) {
	var info Info
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		key, rest, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		setter, ok := fieldSetters[strings.TrimSpace(key)]
		if !ok {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			continue
		}
		v, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			continue
		}
		setter(&info, v)
	}
	if err := sc.Err(); err != nil {
		return Info{}, err
	}
	// MemAvailable is absent on very old kernels (<3.14); fall back to
	// free+buffers+cached so UsedPercent still degrades sanely.
	if info.AvailableKB == 0 {
		info.AvailableKB = info.FreeKB + info.BuffersKB + info.CachedKB
	}
	return info, nil
}

// Collect reads /proc/meminfo and parses it.
func Collect(ctx context.Context) (Info, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return Info{}, err
	}
	defer f.Close()
	return ParseMeminfo(f)
}
