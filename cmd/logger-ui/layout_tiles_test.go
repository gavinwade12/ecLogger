package main

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
)

func newTestTile(height float32) fyne.CanvasObject {
	r := canvas.NewRectangle(nil)
	r.SetMinSize(fyne.NewSize(10, height))
	return r
}

func TestTileGridLayout(t *testing.T) {
	test.NewTempApp(t).Settings().SetTheme(newAppTheme())
	// tiles are at least 180 wide with 4 between them
	tiles := []fyne.CanvasObject{newTestTile(50), newTestTile(80), newTestTile(60), newTestTile(40)}

	for _, tt := range []struct {
		width    float32
		wantCols int
	}{
		{width: 180, wantCols: 1},
		{width: 363, wantCols: 1},
		{width: 364, wantCols: 2},
		{width: 560, wantCols: 3},
	} {
		if got := newTileGridLayout().columns(tt.width); got != tt.wantCols {
			t.Errorf("columns(%v) = %d, want %d", tt.width, got, tt.wantCols)
		}
	}

	l := newTileGridLayout()
	l.Layout(tiles, fyne.NewSize(560, 0))

	// 3 columns 184 wide, each row as tall as its tallest tile
	want := []struct{ pos, size fyne.Size }{
		{fyne.NewSize(0, 0), fyne.NewSize(184, 80)},
		{fyne.NewSize(188, 0), fyne.NewSize(184, 80)},
		{fyne.NewSize(376, 0), fyne.NewSize(184, 80)},
		{fyne.NewSize(0, 84), fyne.NewSize(184, 40)},
	}
	for i, tile := range tiles {
		pos := fyne.NewSize(tile.Position().X, tile.Position().Y)
		if pos != want[i].pos || tile.Size() != want[i].size {
			t.Errorf("tile %d at %v sized %v, want at %v sized %v",
				i, pos, tile.Size(), want[i].pos, want[i].size)
		}
	}
	if got, want := l.MinSize(tiles), fyne.NewSize(180, 124); got != want {
		t.Errorf("MinSize = %v, want %v", got, want)
	}

	tiles[3].Hide()
	if got, want := l.MinSize(tiles), fyne.NewSize(180, 80); got != want {
		t.Errorf("MinSize with a hidden tile = %v, want %v", got, want)
	}
}
