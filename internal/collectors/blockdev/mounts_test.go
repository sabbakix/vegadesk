package blockdev

import (
	"strings"
	"testing"
)

const procMountsFixture = `sysfs /sys sysfs rw,nosuid,nodev,noexec,relatime 0 0
/dev/sda2 / xfs rw,relatime,attr2,inode64 0 0
/dev/sda3 /boot/efi vfat rw,relatime,fmask=0077,dmask=0077 0 0
tmpfs /run/user/1000/my\040dir tmpfs rw,nosuid,nodev,relatime 0 0
`

func TestParseProcMounts(t *testing.T) {
	mounts, err := ParseProcMounts(strings.NewReader(procMountsFixture))
	if err != nil {
		t.Fatalf("ParseProcMounts: %v", err)
	}
	if len(mounts) != 4 {
		t.Fatalf("got %d mounts, want 4: %+v", len(mounts), mounts)
	}
	root := mounts[1]
	if root.Device != "/dev/sda2" || root.Mountpoint != "/" || root.FSType != "xfs" {
		t.Errorf("root mount = %+v", root)
	}
	if len(root.Options) < 2 || root.Options[0] != "rw" {
		t.Errorf("root options = %+v", root.Options)
	}
	escaped := mounts[3]
	if escaped.Mountpoint != "/run/user/1000/my dir" {
		t.Errorf("octal-escaped mountpoint = %q, want unescaped space", escaped.Mountpoint)
	}
}

const fstabFixture = `# /etc/fstab
UUID=56868af8-4452-422e-8382-3ecef3fb94ae / xfs defaults 0 1
UUID=698B-7F03 /boot/efi vfat umask=0077 0 2

/dev/sdb1 /mnt/data ext4 defaults,noatime 0 2
`

func TestParseFstab(t *testing.T) {
	entries, err := ParseFstab(strings.NewReader(fstabFixture))
	if err != nil {
		t.Fatalf("ParseFstab: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3: %+v", len(entries), entries)
	}
	data := entries[2]
	if data.Device != "/dev/sdb1" || data.Mountpoint != "/mnt/data" || data.FSType != "ext4" {
		t.Errorf("entry = %+v", data)
	}
}
