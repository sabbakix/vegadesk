package process

import (
	"context"
	"sort"
	"testing"
)

func TestParseStatLine(t *testing.T) {
	line := "1 (systemd) S 0 1 1 0 -1 4194560 100 0 0 0 500 200 0 0 20 0 1 0 100 123456 4096"
	p, err := ParseStatLine(line)
	if err != nil {
		t.Fatalf("ParseStatLine: %v", err)
	}
	if p.PID != 1 || p.Comm != "systemd" || p.State != "S" || p.UtimeTicks != 500 || p.StimeTicks != 200 || p.RSSPages != 4096 {
		t.Errorf("parsed = %+v", p)
	}
}

func TestParseStatLineCommWithParens(t *testing.T) {
	// process names can contain spaces and parens, e.g. a renamed thread;
	// the parser must split on the LAST ')' not the first.
	line := "42 (sshd: user (foo)) S 1 1 1 0 -1 4194304 5 0 0 0 10 5 0 0 20 0 1 0 200 654321 2048"
	p, err := ParseStatLine(line)
	if err != nil {
		t.Fatalf("ParseStatLine: %v", err)
	}
	if p.Comm != "sshd: user (foo)" {
		t.Errorf("Comm = %q", p.Comm)
	}
}

func TestCollectFromFixture(t *testing.T) {
	procs, err := CollectFrom(context.Background(), "testdata/proc")
	if err != nil {
		t.Fatalf("CollectFrom: %v", err)
	}
	if len(procs) != 2 {
		t.Fatalf("got %d procs, want 2: %+v", len(procs), procs)
	}
	sort.Slice(procs, func(i, j int) bool { return procs[i].PID < procs[j].PID })

	if procs[0].PID != 1 || procs[0].Cmdline != "/sbin/init --switched-root" {
		t.Errorf("pid1 = %+v", procs[0])
	}
	if procs[1].PID != 42 || procs[1].Cmdline != "[sshd: user]" {
		t.Errorf("pid42 (empty cmdline should fall back to comm) = %+v", procs[1])
	}
}
