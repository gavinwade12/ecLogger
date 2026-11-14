package main_test

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	ui "github.com/gavinwade12/ecLogger/cmd/logger-ui"
)

func TestTileGridFitsAsManyColumnsAsItCan(t *testing.T) {
	ta := newTestApp(t)
	ta.connect()
	names := []string{"Engine Speed", "Vehicle Speed", "Boost Pressure Feedback", "Quantity Correction Cylinder #1"}
	for _, name := range names {
		ta.setLiveLog(name, true)
	}
	ta.show(ui.TabLogging)

	// tiles are at least this wide
	const minTileWidth = 180
	pad := theme.Padding()

	for _, width := range []float32{300, 450, 600, 800, 1400} {
		ta.resize(width, 600)
		ta.waitFor("the tiles", func() bool { return len(ta.tilesNow()) == len(names) })
		ta.do(func() {
			var tiles []fyne.CanvasObject
			for _, v := range ta.tilesNow() {
				tiles = append(tiles, v.tile)
			}
			var grid fyne.CanvasObject
			walk(ta.content(ui.TabLogging), func(o fyne.CanvasObject, ancestors []fyne.CanvasObject) {
				if o == tiles[0] {
					grid = ancestors[len(ancestors)-1]
				}
			})
			gridWidth := grid.Size().Width
			cols := max(int((gridWidth+pad)/(minTileWidth+pad)), 1)
			colWidth := (gridWidth - pad*float32(cols-1)) / float32(cols)

			// each row is as tall as its tallest tile
			var y float32
			for start := 0; start < len(tiles); start += cols {
				row := tiles[start:min(start+cols, len(tiles))]
				var height float32
				for _, tile := range row {
					height = max(height, tile.Size().Height)
				}
				for i, tile := range row {
					wantPos := fyne.NewPos(float32(i)*(colWidth+pad), y)
					if tile.Position() != wantPos || tile.Size() != fyne.NewSize(colWidth, height) {
						t.Errorf("window %v: tile %d at %v sized %v, want %v sized %v (%d columns in %v)",
							width, start+i, tile.Position(), tile.Size(), wantPos, fyne.NewSize(colWidth, height), cols, gridWidth)
					}
				}
				y += height + pad
			}
		})
	}
}
