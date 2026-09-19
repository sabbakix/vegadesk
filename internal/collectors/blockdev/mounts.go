package blockdev

import (
	"bufio"
	"context"
	"io"
	"os"
	"strings"
)

// Mount is one active mount, as reported by /proc/mounts.
type Mount struct {
	Device     string
	Mountpoint string
	FSType     string
	Options    []string
}

// ParseProcMounts parses /proc/mounts (or /etc/mtab -- same format). Fields
// are whitespace-separated; device/mountpoint paths with spaces are escaped
// as \040 by the kernel, which is unescaped here.
func ParseProcMounts(r io.Reader) ([]Mount, error) {
	var out []Mount
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 4 {
			continue
		}
		out = append(out, Mount{
			Device:     unescapeOctal(fields[0]),
			Mountpoint: unescapeOctal(fields[1]),
			FSType:     fields[2],
			Options:    strings.Split(fields[3], ","),
		})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func unescapeOctal(s string) string {
	if !strings.Contains(s, "\\") {
		return s
	}
	return strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`).Replace(s)
}

// CollectMounts reads /proc/mounts and parses it.
func CollectMounts(ctx context.Context) ([]Mount, error) {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseProcMounts(f)
}

// FstabEntry is one line of /etc/fstab.
type FstabEntry struct {
	Device     string
	Mountpoint string
	FSType     string
	Options    []string
}

// ParseFstab parses /etc/fstab content, skipping comments and blank lines.
func ParseFstab(r io.Reader) ([]FstabEntry, error) {
	var out []FstabEntry
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		out = append(out, FstabEntry{
			Device:     fields[0],
			Mountpoint: fields[1],
			FSType:     fields[2],
			Options:    strings.Split(fields[3], ","),
		})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// CollectFstab reads /etc/fstab and parses it.
func CollectFstab(ctx context.Context) ([]FstabEntry, error) {
	f, err := os.Open("/etc/fstab")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseFstab(f)
}
