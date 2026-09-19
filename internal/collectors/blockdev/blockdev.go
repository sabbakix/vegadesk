// Package blockdev reads block-device/partition topology via `lsblk -J`,
// the one part of Storage that genuinely needs external-command parsing
// (there's no clean /sys or /proc equivalent that also gives fstype and
// mountpoint in one shot across distros).
package blockdev

import (
	"context"
	"encoding/json"
	"io"
	"strconv"
	"strings"

	"vegadesk/internal/execx"
)

// Device is one lsblk node: a whole disk, or (recursively) one of its
// partitions/children (LVM logical volumes, LUKS mappings, etc.).
type Device struct {
	Name       string   `json:"name"`
	SizeBytes  sizeStr  `json:"size"`
	Type       string   `json:"type"` // disk, part, rom, loop, lvm, crypt, raid1, ...
	FSType     string   `json:"fstype"`
	Mountpoint string   `json:"mountpoint"`
	Model      string   `json:"model"`
	UUID       string   `json:"uuid"`
	Removable  bool     `json:"rm"`
	ReadOnly   bool     `json:"ro"`
	Children   []Device `json:"children,omitempty"`
}

// sizeStr unmarshals lsblk's `-b` byte-count column, which is still
// JSON-quoted as a string, into a plain uint64.
type sizeStr uint64

func (s *sizeStr) UnmarshalJSON(b []byte) error {
	str := strings.Trim(string(b), `"`)
	if str == "null" || str == "" {
		*s = 0
		return nil
	}
	v, err := strconv.ParseUint(str, 10, 64)
	if err != nil {
		return err
	}
	*s = sizeStr(v)
	return nil
}

// Bytes is Device.SizeBytes as a plain uint64, for callers outside this
// package that shouldn't need to know about the sizeStr unmarshal shim.
func (d Device) Bytes() uint64 { return uint64(d.SizeBytes) }

// IsSwap reports whether this device is (or contains) swap space -- lsblk
// reports its mountpoint as the literal string "[SWAP]".
func (d Device) IsSwap() bool { return d.Mountpoint == "[SWAP]" }

// ParseLsblkJSON parses `lsblk -J -b -o NAME,SIZE,TYPE,FSTYPE,MOUNTPOINT,MODEL,UUID,RM,RO` output.
func ParseLsblkJSON(r io.Reader) ([]Device, error) {
	var out struct {
		BlockDevices []Device `json:"blockdevices"`
	}
	if err := json.NewDecoder(r).Decode(&out); err != nil {
		return nil, err
	}
	return out.BlockDevices, nil
}

var lsblkColumns = "NAME,SIZE,TYPE,FSTYPE,MOUNTPOINT,MODEL,UUID,RM,RO"

// Collect runs lsblk and parses its output.
func Collect(ctx context.Context) ([]Device, error) {
	out, err := execx.Run(ctx, "lsblk", "-J", "-b", "-o", lsblkColumns)
	if err != nil {
		return nil, err
	}
	return ParseLsblkJSON(strings.NewReader(out))
}
