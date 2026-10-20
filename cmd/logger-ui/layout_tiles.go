package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// heightForWidther is a tile whose height depends on its width, like one
// with wrapped text.
type heightForWidther interface {
	heightForWidth(width float32) float32
}

// tileGridLayout lays tiles out in as many equal width columns as fit at
// sizeNameTileMinWidth, so the grid adapts to the screen it's on. Each row
// is as tall as its tallest tile.
type tileGridLayout struct {
	// width is the width last laid out at, which MinSize needs to know
	// how many rows there are.
	width float32
}

func newTileGridLayout() *tileGridLayout {
	return &tileGridLayout{}
}

// columns returns how many columns fit in width.
func (l *tileGridLayout) columns(width float32) int {
	minWidth, padding := theme.Size(sizeNameTileMinWidth), theme.Padding()
	cols := int((width + padding) / (minWidth + padding))
	return max(cols, 1)
}

// rows splits the visible objects into rows for width, returning each
// row's objects and height, and the width of a column.
func (l *tileGridLayout) rows(objects []fyne.CanvasObject, width float32) (rows [][]fyne.CanvasObject, heights []float32, colWidth float32) {
	cols := l.columns(width)
	padding := theme.Padding()
	colWidth = (width - padding*float32(cols-1)) / float32(cols)

	var row []fyne.CanvasObject
	var height float32
	for _, o := range objects {
		if !o.Visible() {
			continue
		}
		row = append(row, o)
		height = max(height, tileHeight(o, colWidth))
		if len(row) == cols {
			rows, heights = append(rows, row), append(heights, height)
			row, height = nil, 0
		}
	}
	if len(row) > 0 {
		rows, heights = append(rows, row), append(heights, height)
	}
	return rows, heights, colWidth
}

func tileHeight(o fyne.CanvasObject, width float32) float32 {
	if t, ok := o.(heightForWidther); ok {
		return t.heightForWidth(width)
	}
	return o.MinSize().Height
}

func (l *tileGridLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	l.width = size.Width
	padding := theme.Padding()
	rows, heights, colWidth := l.rows(objects, size.Width)

	var y float32
	for i, row := range rows {
		for j, o := range row {
			o.Move(fyne.NewPos(float32(j)*(colWidth+padding), y))
			o.Resize(fyne.NewSize(colWidth, heights[i]))
		}
		y += heights[i] + padding
	}
}

// MinSize is one tile wide, so the grid can shrink to a single column, and
// as tall as the rows at the width last laid out at.
func (l *tileGridLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	// before the first layout, measure the tiles at their narrowest
	minWidth := theme.Size(sizeNameTileMinWidth)
	_, heights, _ := l.rows(objects, max(l.width, minWidth))
	var height float32
	for _, h := range heights {
		height += h
	}
	if len(heights) > 1 {
		height += theme.Padding() * float32(len(heights)-1)
	}
	return fyne.NewSize(minWidth, height)
}
