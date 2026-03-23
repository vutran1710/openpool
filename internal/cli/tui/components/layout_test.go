package components

import (
	"strings"
	"testing"
)

func TestLayout_SingleRow_SingleCol(t *testing.T) {
	l := NewLayout(40, 10).Rows(
		Row(Col("hello")),
	)
	out := l.Render()
	if !strings.Contains(out, "hello") {
		t.Fatalf("expected 'hello' in output: %q", out)
	}
}

func TestLayout_TwoCols_EqualWidth(t *testing.T) {
	l := NewLayout(40, 5).Rows(
		Row(
			Col("left").Width(0.5),
			Col("right").Width(0.5),
		),
	)
	out := l.Render()
	if !strings.Contains(out, "left") || !strings.Contains(out, "right") {
		t.Fatalf("expected both cols in output: %q", out)
	}
}

func TestLayout_TwoRows(t *testing.T) {
	l := NewLayout(40, 10).Rows(
		Row(Col("top")).Height(0.5),
		Row(Col("bottom")).Height(0.5),
	)
	out := l.Render()
	if !strings.Contains(out, "top") || !strings.Contains(out, "bottom") {
		t.Fatalf("expected both rows: %q", out)
	}
}

func TestLayout_FixedWidth(t *testing.T) {
	l := NewLayout(80, 5).Rows(
		Row(
			Col("sidebar").FixedWidth(20),
			Col("main"),
		),
	)
	out := l.Render()
	if !strings.Contains(out, "sidebar") || !strings.Contains(out, "main") {
		t.Fatalf("expected both cols: %q", out)
	}
}

func TestLayout_FixedHeight(t *testing.T) {
	l := NewLayout(40, 20).Rows(
		Row(Col("header")).FixedHeight(3),
		Row(Col("body")),
		Row(Col("footer")).FixedHeight(2),
	)
	out := l.Render()
	if !strings.Contains(out, "header") || !strings.Contains(out, "body") || !strings.Contains(out, "footer") {
		t.Fatalf("expected all rows: %q", out)
	}
}

func TestLayout_BorderLeft(t *testing.T) {
	l := NewLayout(40, 5).Rows(
		Row(
			Col("left").Width(0.5),
			Col("right").Width(0.5).BorderLeft(true),
		),
	)
	out := l.Render()
	if !strings.Contains(out, "│") {
		t.Fatalf("expected border char: %q", out)
	}
}

func TestLayout_FullBorder(t *testing.T) {
	l := NewLayout(40, 5).Rows(
		Row(Col("boxed").Border(true)),
	)
	out := l.Render()
	if !strings.Contains(out, "─") || !strings.Contains(out, "│") {
		t.Fatalf("expected border chars: %q", out)
	}
}

func TestLayout_NoPadding(t *testing.T) {
	l := NewLayout(20, 3).Rows(
		Row(Col("tight").NoPadding()),
	)
	out := l.Render()
	lines := strings.Split(out, "\n")
	// First non-empty line should start with 't' (no padding)
	for _, line := range lines {
		if strings.Contains(line, "tight") {
			if strings.HasPrefix(strings.TrimRight(line, " "), "tight") {
				return // pass
			}
		}
	}
	t.Fatalf("expected no padding: %q", out)
}

func TestLayout_AlignCenter(t *testing.T) {
	l := NewLayout(40, 3).Rows(
		Row(Col("center").AlignTo(AlignCenter).NoPadding()),
	)
	out := l.Render()
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		if strings.Contains(line, "center") {
			trimmed := strings.TrimLeft(line, " ")
			if len(trimmed) < len(line) {
				return // has leading spaces = centered
			}
		}
	}
	t.Fatalf("expected centered content: %q", out)
}

func TestLayout_AlignRight(t *testing.T) {
	l := NewLayout(40, 3).Rows(
		Row(Col("right").AlignTo(AlignRight).NoPadding()),
	)
	out := l.Render()
	if !strings.Contains(out, "right") {
		t.Fatalf("expected 'right' in output: %q", out)
	}
}

func TestLayout_EmptyRows(t *testing.T) {
	l := NewLayout(40, 10)
	out := l.Render()
	if out != "" {
		t.Fatalf("expected empty output, got: %q", out)
	}
}

func TestLayout_AutoWidthDistribution(t *testing.T) {
	// 3 auto cols should each get ~1/3 of width
	l := NewLayout(60, 5).Rows(
		Row(Col("a"), Col("b"), Col("c")),
	)
	out := l.Render()
	if !strings.Contains(out, "a") || !strings.Contains(out, "b") || !strings.Contains(out, "c") {
		t.Fatalf("expected all cols: %q", out)
	}
}

func TestSeparator(t *testing.T) {
	sep := Separator(40)
	if len(sep) == 0 {
		t.Fatal("separator should not be empty")
	}
	if !strings.Contains(sep, "─") {
		t.Fatalf("expected ─ in separator: %q", sep)
	}
}

func TestLayout_ComplexGrid(t *testing.T) {
	// Real-world: home screen layout
	l := NewLayout(80, 20).Rows(
		Row(
			Col("Menu\n  Discover\n  Matches\n  Inbox").Width(0.5),
			Col("Conversations\n  alice: hey!\n  bob: sup").Width(0.5).BorderLeft(true),
		).Height(0.8),
		Row(
			Col("↑↓ navigate · enter select · tab switch").Width(1.0),
		).FixedHeight(2),
	)
	out := l.Render()
	if !strings.Contains(out, "Menu") || !strings.Contains(out, "Conversations") || !strings.Contains(out, "navigate") {
		t.Fatalf("expected all sections: %q", out)
	}
}
