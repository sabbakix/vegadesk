// Package process lists running processes from /proc/[pid]/{stat,cmdline}.
// CPU% is a delta measure (utime+stime jiffies over elapsed wall time), so
// this package returns raw ticks per snapshot; the caller (the Processes
// feature) diffs successive snapshots against its own previous-sample map.
package process

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// Proc is one process's state at snapshot time.
type Proc struct {
	PID        int
	PPID       int
	Comm       string // short name, from /proc/[pid]/stat, e.g. "sshd"
	State      string // one-letter state: R, S, D, Z, T, ...
	UtimeTicks uint64
	StimeTicks uint64
	RSSPages   uint64
	Cmdline    string // full command line, from /proc/[pid]/cmdline; falls back to Comm
	UID        uint32 // owner uid, from stat()ing the /proc/[pid] directory
}

// ParseStatLine parses one /proc/[pid]/stat line's content. The comm field
// is parenthesized and may itself contain spaces or parens (e.g. a renamed
// process), so it's extracted by the last ')' rather than by field-splitting.
func ParseStatLine(line string) (Proc, error) {
	openParen := strings.IndexByte(line, '(')
	closeParen := strings.LastIndexByte(line, ')')
	if openParen < 0 || closeParen < 0 || closeParen < openParen {
		return Proc{}, fmt.Errorf("process: malformed stat line %q", line)
	}
	pidStr := strings.TrimSpace(line[:openParen])
	comm := line[openParen+1 : closeParen]
	rest := strings.Fields(line[closeParen+1:])
	// rest[0]=state rest[1]=ppid ... rest[11]=utime rest[12]=stime ... rest[21]=rss(pages)
	// (offsets per proc(5), 0-indexed from state)
	if len(rest) < 22 {
		return Proc{}, fmt.Errorf("process: too few fields after comm in %q", line)
	}
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return Proc{}, err
	}
	ppid, _ := strconv.Atoi(rest[1])
	utime, _ := strconv.ParseUint(rest[11], 10, 64)
	stime, _ := strconv.ParseUint(rest[12], 10, 64)
	rss, _ := strconv.ParseUint(rest[21], 10, 64)
	return Proc{
		PID:        pid,
		PPID:       ppid,
		Comm:       comm,
		State:      rest[0],
		UtimeTicks: utime,
		StimeTicks: stime,
		RSSPages:   rss,
	}, nil
}

// listPIDs returns every numeric entry directly under procRoot (normally
// "/proc"), i.e. every process currently visible to us.
func listPIDs(procRoot string) ([]int, error) {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return nil, err
	}
	var pids []int
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		pids = append(pids, pid)
	}
	return pids, nil
}

// readCmdline reads /proc/[pid]/cmdline, joining its NUL-separated argv with
// spaces. Kernel threads and zombies have an empty cmdline.
func readCmdline(procRoot string, pid int) string {
	b, err := os.ReadFile(filepath.Join(procRoot, strconv.Itoa(pid), "cmdline"))
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.TrimRight(string(b), "\x00"), "\x00")
	return strings.Join(parts, " ")
}

// CollectFrom lists all processes under procRoot (pass "/proc" in
// production; a fixture directory in tests). A process that exits between
// listing and reading is silently skipped rather than failing the whole
// snapshot -- this is inherent to /proc and not an error condition.
func CollectFrom(ctx context.Context, procRoot string) ([]Proc, error) {
	pids, err := listPIDs(procRoot)
	if err != nil {
		return nil, err
	}
	procs := make([]Proc, 0, len(pids))
	for _, pid := range pids {
		statPath := filepath.Join(procRoot, strconv.Itoa(pid), "stat")
		f, err := os.Open(statPath)
		if err != nil {
			continue // exited since listPIDs
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 0, 4096), 1<<16)
		var line string
		if sc.Scan() {
			line = sc.Text()
		}
		f.Close()
		if line == "" {
			continue
		}
		p, err := ParseStatLine(line)
		if err != nil {
			continue
		}
		p.Cmdline = readCmdline(procRoot, pid)
		if p.Cmdline == "" {
			p.Cmdline = "[" + p.Comm + "]"
		}
		if fi, err := os.Stat(filepath.Join(procRoot, strconv.Itoa(pid))); err == nil {
			if st, ok := fi.Sys().(*syscall.Stat_t); ok {
				p.UID = st.Uid
			}
		}
		procs = append(procs, p)
	}
	return procs, nil
}

// Collect is CollectFrom against the real /proc.
func Collect(ctx context.Context) ([]Proc, error) {
	return CollectFrom(ctx, "/proc")
}
