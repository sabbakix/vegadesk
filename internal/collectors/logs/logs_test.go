package logs

import (
	"strings"
	"testing"
)

const journalFixture = `{"__REALTIME_TIMESTAMP":"1789851321709759","PRIORITY":"6","MESSAGE":"Started example.service","_SYSTEMD_UNIT":"example.service","SYSLOG_IDENTIFIER":"systemd"}
{"__REALTIME_TIMESTAMP":"1789851321714773","PRIORITY":"1","MESSAGE":"a password is required","SYSLOG_IDENTIFIER":"sudo"}
`

func TestParseJSONLines(t *testing.T) {
	entries, err := ParseJSONLines(strings.NewReader(journalFixture))
	if err != nil {
		t.Fatalf("ParseJSONLines: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2: %+v", len(entries), entries)
	}
	if entries[0].Source() != "example.service" || entries[0].Priority != "6" {
		t.Errorf("entry0 = %+v", entries[0])
	}
	if entries[1].Source() != "sudo" || entries[1].Message != "a password is required" {
		t.Errorf("entry1 (falls back to SYSLOG_IDENTIFIER, no unit) = %+v", entries[1])
	}
	if entries[0].Time.IsZero() {
		t.Errorf("entry0 Time should be parsed from __REALTIME_TIMESTAMP")
	}
}

func TestParseJSONLinesSkipsMalformed(t *testing.T) {
	const mixed = `not json at all
{"__REALTIME_TIMESTAMP":"1789851321709759","PRIORITY":"6","MESSAGE":"ok","SYSLOG_IDENTIFIER":"x"}
`
	entries, err := ParseJSONLines(strings.NewReader(mixed))
	if err != nil {
		t.Fatalf("ParseJSONLines: %v", err)
	}
	if len(entries) != 1 || entries[0].Message != "ok" {
		t.Errorf("entries = %+v, want 1 valid entry", entries)
	}
}
