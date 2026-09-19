// Package logs reads recent entries from the systemd journal via
// `journalctl --output=json`, which -- like Docker's ps output -- emits one
// JSON object per line rather than a single array.
package logs

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"time"

	"vegadesk/internal/execx"
)

// Entry is one journal line, normalized to the fields vegadesk displays.
type Entry struct {
	Time             time.Time
	Priority         string // syslog priority 0 (emerg) - 7 (debug), as a string
	Message          string
	Unit             string // _SYSTEMD_UNIT, when present
	SyslogIdentifier string
}

// Source is the unit name if known, else the syslog identifier (e.g. for
// entries from processes not managed by systemd).
func (e Entry) Source() string {
	if e.Unit != "" {
		return e.Unit
	}
	return e.SyslogIdentifier
}

type rawEntry struct {
	RealtimeTimestamp string          `json:"__REALTIME_TIMESTAMP"`
	Priority          string          `json:"PRIORITY"`
	Message           json.RawMessage `json:"MESSAGE"`
	Unit              string          `json:"_SYSTEMD_UNIT"`
	SyslogIdentifier  string          `json:"SYSLOG_IDENTIFIER"`
}

// ParseJSONLines parses `journalctl --output=json` output. Lines that fail
// to parse are skipped rather than failing the whole batch -- a single
// malformed or binary-message entry shouldn't blank the entire log view.
func ParseJSONLines(r io.Reader) ([]Entry, error) {
	var out []Entry
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var re rawEntry
		if err := json.Unmarshal([]byte(line), &re); err != nil {
			continue
		}
		e := Entry{
			Priority:         re.Priority,
			Unit:             re.Unit,
			SyslogIdentifier: re.SyslogIdentifier,
		}
		// __REALTIME_TIMESTAMP is microseconds since epoch, decimal.
		if us, err := strconv.ParseInt(re.RealtimeTimestamp, 10, 64); err == nil {
			e.Time = time.UnixMicro(us)
		}
		var msg string
		if err := json.Unmarshal(re.Message, &msg); err == nil {
			e.Message = msg
		} else {
			e.Message = "<binary message>"
		}
		out = append(out, e)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// Collect fetches the most recent n journal entries, system-wide.
func Collect(ctx context.Context, n int) ([]Entry, error) {
	out, err := execx.Run(ctx, "journalctl", "-n", strconv.Itoa(n), "--output=json", "--no-pager")
	if err != nil {
		return nil, err
	}
	return ParseJSONLines(strings.NewReader(out))
}
