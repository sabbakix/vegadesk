// Package firewall wraps ufw, the only firewall frontend vegadesk actually
// drives in this build (see internal/platform: firewalld/nftables/iptables
// are detected but not yet implemented here).
//
// Unlike every other read-only collector in vegadesk, `ufw status` itself
// requires root -- there is no unprivileged read path -- so Collect takes
// the argv to run (["ufw", ...] if already root, ["sudo", "-n", "ufw", ...]
// otherwise) rather than deciding privilege itself; the caller (the
// Firewall feature) owns that decision the same way it does for every other
// privileged write action.
//
// ufw is not installed/enabled in a way this session could exercise live
// (no passwordless sudo in this environment), so the parser is verified
// against ufw's documented `status verbose` format rather than live output.
package firewall

import (
	"bufio"
	"context"
	"io"
	"regexp"
	"strings"

	"vegadesk/internal/execx"
)

// Rule is one line of `ufw status verbose`'s rule table.
type Rule struct {
	To     string
	Action string
	From   string
}

// Status is the parsed result of `ufw status verbose`.
type Status struct {
	Active          bool
	DefaultIncoming string
	DefaultOutgoing string
	Rules           []Rule
}

var columnSplit = regexp.MustCompile(`\s{2,}`)

// ParseStatusVerbose parses `ufw status verbose` output:
//
//	Status: active
//	Logging: on (low)
//	Default: deny (incoming), allow (outgoing), disabled (routed)
//	New profiles: skip
//
//	To                         Action      From
//	--                         ------      ----
//	22/tcp                     ALLOW IN    Anywhere
//	80,443/tcp                 ALLOW IN    Anywhere
func ParseStatusVerbose(r io.Reader) (Status, error) {
	var st Status
	sc := bufio.NewScanner(r)
	inTable := false
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), " \t")
		trimmed := strings.TrimSpace(line)

		switch {
		case strings.HasPrefix(trimmed, "Status:"):
			st.Active = strings.TrimSpace(strings.TrimPrefix(trimmed, "Status:")) == "active"
		case strings.HasPrefix(trimmed, "Default:"):
			// "deny (incoming), allow (outgoing), disabled (routed)"
			rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "Default:"))
			for _, part := range strings.Split(rest, ",") {
				part = strings.TrimSpace(part)
				fields := strings.Fields(part)
				if len(fields) != 2 {
					continue
				}
				verb, noun := fields[0], strings.Trim(fields[1], "()")
				switch noun {
				case "incoming":
					st.DefaultIncoming = verb
				case "outgoing":
					st.DefaultOutgoing = verb
				}
			}
		case strings.HasPrefix(trimmed, "To") && strings.Contains(trimmed, "Action"):
			inTable = true
		case strings.HasPrefix(trimmed, "--"):
			continue
		case inTable && trimmed != "":
			cols := columnSplit.Split(trimmed, 3)
			if len(cols) < 3 {
				continue
			}
			st.Rules = append(st.Rules, Rule{To: cols[0], Action: cols[1], From: cols[2]})
		}
	}
	if err := sc.Err(); err != nil {
		return Status{}, err
	}
	return st, nil
}

// Collect runs argv (see package doc: caller decides the sudo prefix) and
// parses its output as `ufw status verbose`.
func Collect(ctx context.Context, argv []string) (Status, error) {
	out, err := execx.Run(ctx, argv[0], argv[1:]...)
	if err != nil {
		return Status{}, err
	}
	return ParseStatusVerbose(strings.NewReader(out))
}
