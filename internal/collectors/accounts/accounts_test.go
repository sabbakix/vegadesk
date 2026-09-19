package accounts

import (
	"strings"
	"testing"
)

const passwdFixture = `root:x:0:0:root:/root:/bin/bash
daemon:x:1:1:daemon:/usr/sbin:/usr/sbin/nologin
user:x:1000:1000:Test User,,,:/home/user:/bin/bash
nobody:x:65534:65534:nobody:/nonexistent:/usr/sbin/nologin
`

const groupFixture = `root:x:0:
daemon:x:1:
user:x:1000:
sudo:x:27:user
docker:x:983:user
`

func TestParsePasswd(t *testing.T) {
	users, err := ParsePasswd(strings.NewReader(passwdFixture))
	if err != nil {
		t.Fatalf("ParsePasswd: %v", err)
	}
	if len(users) != 4 {
		t.Fatalf("got %d users, want 4", len(users))
	}
	if !users[0].IsHuman() { // root
		t.Errorf("root should be IsHuman()")
	}
	if users[1].IsHuman() { // daemon, uid 1
		t.Errorf("daemon (uid 1) should not be IsHuman()")
	}
	if !users[2].IsHuman() || users[2].Name != "user" || users[2].Comment != "Test User,,," {
		t.Errorf("user = %+v", users[2])
	}
	if users[3].IsHuman() { // nobody, uid 65534
		t.Errorf("nobody (uid 65534) should not be IsHuman()")
	}
}

func TestMergeGroups(t *testing.T) {
	users, err := ParsePasswd(strings.NewReader(passwdFixture))
	if err != nil {
		t.Fatal(err)
	}
	groups, err := ParseGroup(strings.NewReader(groupFixture))
	if err != nil {
		t.Fatal(err)
	}
	merged := mergeGroups(users, groups)

	var testUser User
	for _, u := range merged {
		if u.Name == "user" {
			testUser = u
		}
	}
	want := map[string]bool{"user": true, "sudo": true, "docker": true}
	if len(testUser.Groups) != 3 {
		t.Fatalf("groups = %+v, want primary+2 supplementary", testUser.Groups)
	}
	for _, g := range testUser.Groups {
		if !want[g] {
			t.Errorf("unexpected group %q in %+v", g, testUser.Groups)
		}
	}
}
