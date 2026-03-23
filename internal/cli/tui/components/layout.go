package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/vutran1710/dating-dev/internal/cli/tui/theme"
)

// Align controls horizontal alignment within a cell.
type Align int

const (
	AlignLeft Align = iota
	AlignCenter
	AlignRight
)

// Cell represents a single content area within a row.
type Cell struct {
	content   string
	widthFrac float64 // fraction of row width (0.0-1.0), 0 = auto
	widthFixed int    // fixed width in chars, 0 = use fraction
	border    bool
	borderLeft bool
	borderRight bool
	padding   int
	align     Align
}

// Col creates a new cell with content.
func Col(content string) Cell {
	return Cell{
		content: content,
		padding: 1,
		align:   AlignLeft,
	}
}

// Width sets the cell width as a fraction of the row (0.0-1.0).
func (c Cell) Width(frac float64) Cell {
	c.widthFrac = frac
	return c
}

// FixedWidth sets the cell width in characters.
func (c Cell) FixedWidth(chars int) Cell {
	c.widthFixed = chars
	return c
}

// Border adds a full border around the cell.
func (c Cell) Border(on bool) Cell {
	c.border = on
	return c
}

// BorderLeft adds a left border to the cell.
func (c Cell) BorderLeft(on bool) Cell {
	c.borderLeft = on
	return c
}

// BorderRight adds a right border to the cell.
func (c Cell) BorderRight(on bool) Cell {
	c.borderRight = on
	return c
}

// Padding sets horizontal padding (left + right).
func (c Cell) Padding(p int) Cell {
	c.padding = p
	return c
}

// NoPadding removes padding.
func (c Cell) NoPadding() Cell {
	c.padding = 0
	return c
}

// AlignTo sets horizontal alignment.
func (c Cell) AlignTo(a Align) Cell {
	c.align = a
	return c
}

// LayoutRow represents a horizontal row of cells.
type LayoutRow struct {
	cells      []Cell
	heightFrac float64 // fraction of layout height (0.0-1.0), 0 = auto
	heightFixed int    // fixed height in lines, 0 = use fraction
}

// Row creates a new layout row with cells.
func Row(cells ...Cell) LayoutRow {
	return LayoutRow{cells: cells}
}

// Height sets the row height as a fraction of the layout (0.0-1.0).
func (r LayoutRow) Height(frac float64) LayoutRow {
	r.heightFrac = frac
	return r
}

// FixedHeight sets the row height in lines.
func (r LayoutRow) FixedHeight(lines int) LayoutRow {
	r.heightFixed = lines
	return r
}

// Layout arranges content in rows and columns.
type Layout struct {
	width  int
	height int
	rows   []LayoutRow
}

// NewLayout creates a layout with the given dimensions.
func NewLayout(width, height int) Layout {
	return Layout{width: width, height: height}
}

// Rows sets the layout rows.
func (l Layout) Rows(rows ...LayoutRow) Layout {
	l.rows = rows
	return l
}

// Render produces the final string output.
func (l Layout) Render() string {
	if len(l.rows) == 0 {
		return ""
	}

	// Calculate row heights
	rowHeights := l.calcRowHeights()

	// Render each row
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

	// First pass: fixed + fractional heights
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

	// Second pass: distribute remaining to auto rows
	if autoCount > 0 && remaining > 0 {
		each := remaining / autoCount
		for i, row := range l.rows {
			if row.heightFixed == 0 && row.heightFrac == 0 {
				heights[i] = each
			}
		}
	}

	// Ensure minimum height of 1
	for i := range heights {
		if heights[i] < 1 {
			heights[i] = 1
		}
	}

	return heights
}

func (l Layout) renderRow(row LayoutRow, height int) string {
	if len(row.cells) == 0 {
		return ""
	}

	// Calculate cell widths
	cellWidths := l.calcCellWidths(row)

	// Render each cell
	var renderedCells []string
	for i, cell := range row.cells {
		rendered := l.renderCell(cell, cellWidths[i], height)
		renderedCells = append(renderedCells, rendered)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, renderedCells...)
}

func (l Layout) calcCellWidths(row LayoutRow) []int {
	widths := make([]int, len(row.cells))
	remaining := l.width
	autoCount := 0

	// First pass: fixed + fractional widths
	for i, cell := range row.cells {
		if cell.widthFixed > 0 {
			widths[i] = cell.widthFixed
			remaining -= cell.widthFixed
		} else if cell.widthFrac > 0 {
			w := int(float64(l.width) * cell.widthFrac)
			widths[i] = w
			remaining -= w
		} else {
			autoCount++
		}
	}

	// Second pass: distribute remaining to auto cells
	if autoCount > 0 && remaining > 0 {
		each := remaining / autoCount
		for i, cell := range row.cells {
			if cell.widthFixed == 0 && cell.widthFrac == 0 {
				widths[i] = each
			}
		}
	}

	// Ensure minimum width of 1
	for i := range widths {
		if widths[i] < 1 {
			widths[i] = 1
		}
	}

	return widths
}

func (l Layout) renderCell(cell Cell, width, height int) string {
	style := lipgloss.NewStyle()

	// Padding
	if cell.padding > 0 {
		style = style.PaddingLeft(cell.padding).PaddingRight(cell.padding)
		width -= cell.padding * 2
	}

	// Border
	if cell.border {
		style = style.
			Border(lipgloss.NormalBorder()).
			BorderForeground(theme.Border)
		width -= 2
		height -= 2
	} else {
		if cell.borderLeft {
			style = style.
				BorderLeft(true).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(theme.Border)
			width -= 1
		}
		if cell.borderRight {
			style = style.
				BorderRight(true).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(theme.Border)
			width -= 1
		}
	}

	// Alignment
	switch cell.align {
	case AlignCenter:
		style = style.Align(lipgloss.Center)
	case AlignRight:
		style = style.Align(lipgloss.Right)
	default:
		style = style.Align(lipgloss.Left)
	}

	// Size
	if width > 0 {
		style = style.Width(width)
	}
	if height > 0 {
		style = style.Height(height)
	}

	return style.Render(cell.content)
}

// Separator creates a horizontal line spanning the full width.
func Separator(width int) string {
	return lipgloss.NewStyle().
		Width(width).
		Foreground(theme.Border).
		Render(Repeat("─", width))
}
