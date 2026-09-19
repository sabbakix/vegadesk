package blockdev

import (
	"os"
	"testing"
)

func TestParseLsblkJSON(t *testing.T) {
	f, err := os.Open("testdata/lsblk.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	devs, err := ParseLsblkJSON(f)
	if err != nil {
		t.Fatalf("ParseLsblkJSON: %v", err)
	}
	if len(devs) != 3 {
		t.Fatalf("got %d top-level devices, want 3: %+v", len(devs), devs)
	}

	sda := devs[1]
	if sda.Name != "sda" || sda.Bytes() != 429496729600 || sda.Model != "QEMU HARDDISK" {
		t.Errorf("sda = %+v", sda)
	}
	if sda.FSType != "" || sda.Mountpoint != "" {
		t.Errorf("sda should have empty (JSON null) fstype/mountpoint, got %+v", sda)
	}
	if len(sda.Children) != 2 {
		t.Fatalf("sda children = %d, want 2", len(sda.Children))
	}

	swap := sda.Children[0]
	if !swap.IsSwap() {
		t.Errorf("sda1 should report IsSwap(), got mountpoint %q", swap.Mountpoint)
	}

	root := sda.Children[1]
	if root.Mountpoint != "/" || root.FSType != "xfs" || root.Bytes() != 416385334272 {
		t.Errorf("sda2 = %+v", root)
	}
}
