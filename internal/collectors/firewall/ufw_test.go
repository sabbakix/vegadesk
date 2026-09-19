package firewall

import (
	"strings"
	"testing"
)

const ufwStatusFixture = `Status: active
Logging: on (low)
Default: deny (incoming), allow (outgoing), disabled (routed)
New profiles: skip

To                         Action      From
--                         ------      ----
22/tcp                     ALLOW IN    Anywhere
80,443/tcp                 ALLOW IN    Anywhere
22/tcp (v6)                ALLOW IN    Anywhere (v6)
`

func TestParseStatusVerbose(t *testing.T) {
	st, err := ParseStatusVerbose(strings.NewReader(ufwStatusFixture))
	if err != nil {
		t.Fatalf("ParseStatusVerbose: %v", err)
	}
	if !st.Active {
		t.Errorf("Active = false, want true")
	}
	if st.DefaultIncoming != "deny" || st.DefaultOutgoing != "allow" {
		t.Errorf("defaults = %q/%q", st.DefaultIncoming, st.DefaultOutgoing)
	}
	if len(st.Rules) != 3 {
		t.Fatalf("got %d rules, want 3: %+v", len(st.Rules), st.Rules)
	}
	if st.Rules[1].To != "80,443/tcp" || st.Rules[1].Action != "ALLOW IN" || st.Rules[1].From != "Anywhere" {
		t.Errorf("rule1 = %+v", st.Rules[1])
	}
}

func TestParseStatusVerboseInactive(t *testing.T) {
	const inactive = "Status: inactive\n"
	st, err := ParseStatusVerbose(strings.NewReader(inactive))
	if err != nil {
		t.Fatalf("ParseStatusVerbose: %v", err)
	}
	if st.Active {
		t.Errorf("Active = true, want false")
	}
	if len(st.Rules) != 0 {
		t.Errorf("Rules = %+v, want none", st.Rules)
	}
}
