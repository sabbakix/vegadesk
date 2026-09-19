package disk

import (
	"syscall"
	"testing"
)

func TestUsageFromStatfs(t *testing.T) {
	st := syscall.Statfs_t{Bsize: 4096, Blocks: 1000, Bfree: 250, Bavail: 200}
	u := usageFromStatfs(st)
	if u.TotalBytes != 1000*4096 || u.FreeBytes != 250*4096 || u.AvailBytes != 200*4096 {
		t.Fatalf("usage = %+v", u)
	}
	if got, want := u.UsedBytes(), uint64(750*4096); got != want {
		t.Errorf("UsedBytes = %d, want %d", got, want)
	}
	if got, want := u.UsedPercent(), 75.0; got != want {
		t.Errorf("UsedPercent = %v, want %v", got, want)
	}
}
