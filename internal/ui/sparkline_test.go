package ui

import "testing"

func TestBarStringFullEmpty(t *testing.T) {
	if got := BarString(10, 0); got != "          " {
		t.Errorf("empty bar = %q", got)
	}
	if got := BarString(10, 1); got != "██████████" {
		t.Errorf("full bar = %q", got)
	}
}

func TestBarStringHalf(t *testing.T) {
	got := BarString(10, 0.5)
	if len([]rune(got)) != 10 {
		t.Fatalf("width = %d, want 10 (%q)", len([]rune(got)), got)
	}
	// exactly half of 10*8=80 eighths => 5 full blocks, no partial
	want := "█████     "
	if got != want {
		t.Errorf("half bar = %q, want %q", got, want)
	}
}

func TestGraphAllZero(t *testing.T) {
	lines := Graph(make([]float64, 20), 4, 2, 100)
	for _, l := range lines {
		for _, r := range []rune(l) {
			if r != 0x2800 {
				t.Fatalf("expected blank braille cell, got %q in line %q", r, l)
			}
		}
	}
}

func TestGraphAllMax(t *testing.T) {
	vals := make([]float64, 20)
	for i := range vals {
		vals[i] = 100
	}
	lines := Graph(vals, 4, 2, 100)
	for _, l := range lines {
		for _, r := range []rune(l) {
			if r != 0x28FF {
				t.Fatalf("expected fully-lit braille cell (0x28FF), got %#x in line %q", r, l)
			}
		}
	}
}

func TestGraphHalfHeightBottomFilled(t *testing.T) {
	// max=100, values=50 => half of dotRows (height=2 -> 8 dot-rows) = 4 dots
	// lit per column, which is exactly the bottom character row fully lit
	// and the top character row fully blank.
	vals := make([]float64, 20)
	for i := range vals {
		vals[i] = 50
	}
	lines := Graph(vals, 4, 2, 100)
	if len(lines) != 2 {
		t.Fatalf("want 2 lines, got %d", len(lines))
	}
	top, bottom := lines[0], lines[1]
	for _, r := range []rune(top) {
		if r != 0x2800 {
			t.Errorf("top row should be blank, got %#x", r)
		}
	}
	for _, r := range []rune(bottom) {
		if r != 0x28FF {
			t.Errorf("bottom row should be fully lit, got %#x", r)
		}
	}
}
