// Package net reads per-interface network byte counters from /proc/net/dev.
package net

import (
	"bufio"
	"context"
	"io"
	"os"
	"strconv"
	"strings"
)

// IfaceStats is one interface's cumulative counters since boot.
type IfaceStats struct {
	Name     string
	RxBytes  uint64
	TxBytes  uint64
	RxPacket uint64
	TxPacket uint64
}

// ParseNetDev parses /proc/net/dev, skipping its two header lines.
func ParseNetDev(r io.Reader) ([]IfaceStats, error) {
	var out []IfaceStats
	sc := bufio.NewScanner(r)
	lineNum := 0
	for sc.Scan() {
		lineNum++
		if lineNum <= 2 {
			continue // "Inter-|   Receive" / " face |bytes packets ..." headers
		}
		line := sc.Text()
		name, rest, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) < 9 {
			continue
		}
		rx, err1 := strconv.ParseUint(fields[0], 10, 64)
		rxPkt, err2 := strconv.ParseUint(fields[1], 10, 64)
		tx, err3 := strconv.ParseUint(fields[8], 10, 64)
		txPkt, err4 := strconv.ParseUint(fields[9], 10, 64)
		if err1 != nil || err2 != nil || err3 != nil {
			continue
		}
		if err4 != nil {
			txPkt = 0
		}
		out = append(out, IfaceStats{
			Name:     strings.TrimSpace(name),
			RxBytes:  rx,
			RxPacket: rxPkt,
			TxBytes:  tx,
			TxPacket: txPkt,
		})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// Collect reads /proc/net/dev and parses it.
func Collect(ctx context.Context) ([]IfaceStats, error) {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseNetDev(f)
}

// RateBps computes a bytes/sec rate given a prior and current cumulative
// counter and the elapsed seconds between the two samples.
func RateBps(prev, cur uint64, elapsedSeconds float64) float64 {
	if elapsedSeconds <= 0 || cur < prev {
		return 0
	}
	return float64(cur-prev) / elapsedSeconds
}
