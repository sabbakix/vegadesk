// Package accounts lists local user accounts from /etc/passwd and their
// supplementary groups from /etc/group. Both files are plain, world-
// readable text on every Linux distro -- no shelling out needed, unlike
// most other collectors.
package accounts

import (
	"bufio"
	"context"
	"io"
	"os"
	"strconv"
	"strings"
)

// User is one /etc/passwd entry.
type User struct {
	Name    string
	UID     int
	GID     int
	Comment string // GECOS field, usually the display name
	Home    string
	Shell   string
	Groups  []string // supplementary groups, from /etc/group; filled by Collect
}

// IsHuman reports whether this looks like a real login account rather than
// a system/service account -- UID 0 (root) or in [1000, 65534), the common
// convention across Debian/Ubuntu/RHEL/Fedora/Arch alike. 65534 itself is
// excluded because it's the conventional sentinel UID for "nobody".
func (u User) IsHuman() bool { return u.UID == 0 || (u.UID >= 1000 && u.UID < 65534) }

// ParsePasswd parses /etc/passwd content.
func ParsePasswd(r io.Reader) ([]User, error) {
	var out []User
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, ":")
		if len(f) < 7 {
			continue
		}
		uid, err1 := strconv.Atoi(f[2])
		gid, err2 := strconv.Atoi(f[3])
		if err1 != nil || err2 != nil {
			continue
		}
		out = append(out, User{
			Name: f[0], UID: uid, GID: gid, Comment: f[4], Home: f[5], Shell: f[6],
		})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// group is one /etc/group entry: name, gid, and its member list.
type group struct {
	Name    string
	GID     int
	Members []string
}

// ParseGroup parses /etc/group content.
func ParseGroup(r io.Reader) ([]group, error) {
	var out []group
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, ":")
		if len(f) < 4 {
			continue
		}
		gid, err := strconv.Atoi(f[2])
		if err != nil {
			continue
		}
		var members []string
		if f[3] != "" {
			members = strings.Split(f[3], ",")
		}
		out = append(out, group{Name: f[0], GID: gid, Members: members})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// mergeGroups fills each User's Groups: their primary group (by gid) first,
// then any supplementary groups that list them as a member.
func mergeGroups(users []User, groups []group) []User {
	byGID := make(map[int]string, len(groups))
	for _, g := range groups {
		byGID[g.GID] = g.Name
	}
	for i, u := range users {
		var names []string
		if name, ok := byGID[u.GID]; ok {
			names = append(names, name)
		}
		for _, g := range groups {
			if g.GID == u.GID {
				continue // already added as primary
			}
			for _, m := range g.Members {
				if m == u.Name {
					names = append(names, g.Name)
					break
				}
			}
		}
		users[i].Groups = names
	}
	return users
}

// Collect reads /etc/passwd and /etc/group and merges them.
func Collect(ctx context.Context) ([]User, error) {
	pf, err := os.Open("/etc/passwd")
	if err != nil {
		return nil, err
	}
	defer pf.Close()
	users, err := ParsePasswd(pf)
	if err != nil {
		return nil, err
	}

	gf, err := os.Open("/etc/group")
	if err != nil {
		return users, nil // group enrichment is best-effort
	}
	defer gf.Close()
	groups, err := ParseGroup(gf)
	if err != nil {
		return users, nil
	}
	return mergeGroups(users, groups), nil
}
