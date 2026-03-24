package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/vutran1710/openpool/internal/cli/tui/theme"
)

// Align controls horizontal alignment within a column.
type Align int

const (
	AlignLeft Align = iota
	AlignCenter
	AlignRight
)

// Col represents a single column within a row.
type Col struct {
	content    string
	widthFrac  float64 // fraction of row width (0.0-1.0), 0 = auto
	widthFixed int     // fixed width in chars, 0 = use fraction
	border     bool
	borderLeft bool
	borderRight bool
	padding    int
	align      Align
}

// NewCol creates a new column with content.
func NewCol(content string) Col {
	return Col{
		content: content,
		padding: 1,
		align:   AlignLeft,
	}
}

// Width sets the column width as a fraction of the row (0.0-1.0).
func (c Col) Width(frac float64) Col {
	c.widthFrac = frac
	return c
}

// FixedWidth sets the column width in characters.
func (c Col) FixedWidth(chars int) Col {
	c.widthFixed = chars
	return c
}

// Border adds a full border around the column.
func (c Col) Border(on bool) Col {
	c.border = on
	return c
}

// BorderLeft adds a left border to the column.
func (c Col) BorderLeft(on bool) Col {
	c.borderLeft = on
	return c
}

// BorderRight adds a right border to the column.
func (c Col) BorderRight(on bool) Col {
	c.borderRight = on
	return c
}

// Padding sets horizontal padding (left + right).
func (c Col) Padding(p int) Col {
	c.padding = p
	return c
}

// NoPadding removes padding.
func (c Col) NoPadding() Col {
	c.padding = 0
	return c
}

// AlignTo sets horizontal alignment.
func (c Col) AlignTo(a Align) Col {
	c.align = a
	return c
}

// Row represents a horizontal row of columns.
type Row struct {
	cols       []Col
	heightFrac  float64
	heightFixed int
}

// NewRow creates a new row with columns.
func NewRow(cols ...Col) Row {
	return Row{cols: cols}
}

// Height sets the row height as a fraction of the layout (0.0-1.0).
func (r Row) Height(frac float64) Row {
	r.heightFrac = frac
	return r
}

// FixedHeight sets the row height in lines.
func (r Row) FixedHeight(lines int) Row {
	r.heightFixed = lines
	return r
}

// Layout arranges content in rows and columns.
type Layout struct {
	width  int
	height int
	rows   []Row
}

// NewLayout creates a layout with the given dimensions.
func NewLayout(width, height int) Layout {
	return Layout{width: width, height: height}
}

// Rows sets the layout rows.
func (l Layout) Rows(rows ...Row) Layout {
	l.rows = rows
	return l
}

// Render produces the final string output.
func (l Layout) Render() string {
	if len(l.rows) == 0 {
		return ""
	}

	rowHeights := l.calcRowHeights()

	var renderedRows []string
	for i, row := range l.rows {
		rendered := l.renderRow(row, rowHeights[i])
		renderedRows = append(renderedRows, rendered)
	}

	return strings.Join(renderedRows, "\n")
}

func (l Layout) calcRowHeights() []int {
	heights := make([]int, len(l.rows))
	remaining := l.height
	autoCount := 0

	for i, row := range l.rows {
		if row.heightFixed > 0 {
			heights[i] = row.heightFixed
			remaining -= row.heightFixed
		} else if row.heightFrac > 0 {
			h := int(float64(l.height) * row.heightFrac)
			heights[i] = h
			remaining -= h
		} else {
			autoCount++
		}
	}

	if autoCount > 0 && remaining > 0 {
		each := remaining / autoCount
		for i, row := range l.rows {
			if row.heightFixed == 0 && row.heightFrac == 0 {
				heights[i] = each
			}
		}
	}

	for i := range heights {
		if heights[i] < 1 {
			heights[i] = 1
		}
	}

	return heights
}

func (l Layout) renderRow(row Row, height int) string {
	if len(row.cols) == 0 {
		return ""
	}

	colWidths := l.calcColWidths(row)

	var renderedCols []string
	for i, col := range row.cols {
		rendered := l.renderCol(col, colWidths[i], height)
		renderedCols = append(renderedCols, rendered)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, renderedCols...)
}

func (l Layout) calcColWidths(row Row) []int {
	widths := make([]int, len(row.cols))
	remaining := l.width
	autoCount := 0

	for i, col := range row.cols {
		if col.widthFixed > 0 {
			widths[i] = col.widthFixed
			remaining -= col.widthFixed
		} else if col.widthFrac > 0 {
			w := int(float64(l.width) * col.widthFrac)
			widths[i] = w
			remaining -= w
		} else {
			autoCount++
		}
	}

	if autoCount > 0 && remaining > 0 {
		each := remaining / autoCount
		for i, col := range row.cols {
			if col.widthFixed == 0 && col.widthFrac == 0 {
				widths[i] = each
			}
		}
	}

	for i := range widths {
		if widths[i] < 1 {
			widths[i] = 1
		}
	}

	return widths
}

func (l Layout) renderCol(col Col, width, height int) string {
	style := lipgloss.NewStyle()

	if col.padding > 0 {
		style = style.PaddingLeft(col.padding).PaddingRight(col.padding)
		width -= col.padding * 2
	}

	if col.border {
		style = style.
			Border(lipgloss.NormalBorder()).
			BorderForeground(theme.Border)
		width -= 2
		height -= 2
	} else {
		if col.borderLeft {
			style = style.
				BorderLeft(true).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(theme.Border)
			width -= 1
		}
		if col.borderRight {
			style = style.
				BorderRight(true).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(theme.Border)
			width -= 1
		}
	}

	switch col.align {
	case AlignCenter:
		style = style.Align(lipgloss.Center)
	case AlignRight:
		style = style.Align(lipgloss.Right)
	default:
		style = style.Align(lipgloss.Left)
	}

	if width > 0 {
		style = style.Width(width)
	}
	if height > 0 {
		style = style.Height(height)
	}

	return style.Render(col.content)
}

// Separator creates a horizontal line spanning the full width.
func Separator(width int) string {
	return lipgloss.NewStyle().
		Width(width).
		Foreground(theme.Border).
		Render(Repeat("─", width))
}
