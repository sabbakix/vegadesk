// Package disk provides filesystem usage for the overview dashboard's disk
// gauge. Full block-device/partition topology (lsblk) lives in the Storage
// feature (Phase C); this file is deliberately small: one statfs call.
package disk

import "syscall"

// Usage is a filesystem usage snapshot in bytes.
type Usage struct {
	TotalBytes uint64
	FreeBytes  uint64
	AvailBytes uint64 // free space available to unprivileged users
}

// UsedBytes is TotalBytes minus FreeBytes.
func (u Usage) UsedBytes() uint64 {
	if u.FreeBytes > u.TotalBytes {
		return 0
	}
	return u.TotalBytes - u.FreeBytes
}

// UsedPercent is UsedBytes as a percentage of TotalBytes.
func (u Usage) UsedPercent() float64 {
	if u.TotalBytes == 0 {
		return 0
	}
	return float64(u.UsedBytes()) / float64(u.TotalBytes) * 100
}

// usageFromStatfs converts a raw statfs result into Usage. Split out from
// UsagePath so the arithmetic is unit-testable without a real filesystem.
func usageFromStatfs(st syscall.Statfs_t) Usage {
	blockSize := uint64(st.Bsize)
	return Usage{
		TotalBytes: st.Blocks * blockSize,
		FreeBytes:  st.Bfree * blockSize,
		AvailBytes: st.Bavail * blockSize,
	}
}

// UsagePath statfs(2)s path (typically "/") and returns its usage.
func UsagePath(path string) (Usage, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return Usage{}, err
	}
	return usageFromStatfs(st), nil
}
