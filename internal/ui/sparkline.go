package ui

import "strings"

// blockLevels gives a compact single-row trend indicator: one character per
// data point, 8 levels of vertical fill. Good for a table cell or inline
// trend; for the big overview panels see Graph below.
var blockLevels = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// Sparkline renders one line, one character per value (oldest first,
// right-aligned to width -- only the most recent `width` values are shown).
func Sparkline(values []float64, width int, max float64) string {
	if width <= 0 {
		return ""
	}
	v := lastN(values, width)
	var b strings.Builder
	pad := width - len(v)
	b.WriteString(strings.Repeat(" ", pad))
	for _, val := range v {
		b.WriteRune(levelRune(val, max))
	}
	return b.String()
}

func levelRune(val, max float64) rune {
	if max <= 0 {
		return blockLevels[0]
	}
	frac := val / max
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	idx := int(frac * float64(len(blockLevels)-1))
	return blockLevels[idx]
}

func lastN(v []float64, n int) []float64 {
	if len(v) <= n {
		return v
	}
	return v[len(v)-n:]
}

// braille dot bit values, indexed [column][rowsLitFromBottom-1]. Bit layout
// per Unicode braille patterns (U+2800 base): dots numbered 1-8 as
//
//	1 4      bits: 0x01 0x08
//	2 5            0x02 0x10
//	3 6            0x04 0x20
//	7 8            0x40 0x80
//
// so, bottom-up, column 0's dots are 3,2,1,7 (wait: row3 is the bottom row,
// which holds dots 7/8, not 3/6) -- see colBits below for the corrected,
// verified-bottom-up order.
var colBits = [2][4]byte{
	// column 0 (left), bottom-up: row3(dot7)=0x40, row2(dot3)=0x04, row1(dot2)=0x02, row0(dot1)=0x01
	{0x40, 0x04, 0x02, 0x01},
	// column 1 (right), bottom-up: row3(dot8)=0x80, row2(dot6)=0x20, row1(dot5)=0x10, row0(dot4)=0x08
	{0x80, 0x20, 0x10, 0x08},
}

// bitsForFill returns the byte mask for lighting `lit` dots (0..4) from the
// bottom of a single braille column.
func bitsForFill(col int, lit int) byte {
	if lit < 0 {
		lit = 0
	}
	if lit > 4 {
		lit = 4
	}
	var mask byte
	for i := 0; i < lit; i++ {
		mask |= colBits[col][i]
	}
	return mask
}

// Graph renders a multi-row braille bar graph: width characters wide (each
// character holds 2 independent data columns), height characters tall (each
// adds 4 dot-rows of vertical resolution, so total resolution is
// height*4 levels). Values are read oldest-to-newest; only the most recent
// width*2 are shown. This is the "looks like btop" widget for CPU/mem/net
// history panels.
func Graph(values []float64, width, height int, max float64) []string {
	if width <= 0 || height <= 0 {
		return nil
	}
	dotCols := width * 2
	dotRows := height * 4

	v := lastN(values, dotCols)
	levels := make([]int, dotCols)
	pad := dotCols - len(v)
	for i := range levels {
		if i < pad {
			levels[i] = 0
			continue
		}
		val := v[i-pad]
		frac := 0.0
		if max > 0 {
			frac = val / max
		}
		if frac < 0 {
			frac = 0
		}
		if frac > 1 {
			frac = 1
		}
		levels[i] = int(frac*float64(dotRows) + 0.5)
	}

	lines := make([]string, height)
	for r := 0; r < height; r++ {
		// Dot-row range for this character row, counted bottom-up across
		// the whole graph: character row 0 is the top, height-1 is bottom.
		rowsBelow := (height - 1 - r) * 4
		var b strings.Builder
		for k := 0; k < width; k++ {
			c0 := levels[2*k]
			c1 := levels[2*k+1]
			lit0 := c0 - rowsBelow
			lit1 := c1 - rowsBelow
			mask := bitsForFill(0, lit0) | bitsForFill(1, lit1)
			b.WriteRune(rune(0x2800 + int(mask)))
		}
		lines[r] = b.String()
	}
	return lines
}
