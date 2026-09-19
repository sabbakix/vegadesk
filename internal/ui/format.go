package ui

import "fmt"

// FormatBytes renders a byte count human-readably (B/KB/MB/GB/TB), the
// convention used throughout vegadesk's dashboards and tables.
func FormatBytes(v float64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	return fmt.Sprintf("%.1f%s", v, units[i])
}
