package main

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func TestListLayout(t *testing.T) {
	test.NewTempApp(t)
	objects := []fyne.CanvasObject{newTestTile(30), newTestTile(50), newTestTile(20)}
	objects[1].Hide()

	l := newListLayout(4)
	l.Layout(objects, fyne.NewSize(300, 0))
	for i, want := range map[int]struct{ pos, size fyne.Size }{
		0: {fyne.NewSize(0, 0), fyne.NewSize(300, 30)},
		2: {fyne.NewSize(0, 34), fyne.NewSize(300, 20)},
	} {
		o := objects[i]
		if pos := fyne.NewSize(o.Position().X, o.Position().Y); pos != want.pos || o.Size() != want.size {
			t.Errorf("object %d at %v sized %v, want at %v sized %v", i, pos, o.Size(), want.pos, want.size)
		}
	}
	if got, want := l.MinSize(objects), fyne.NewSize(10, 54); got != want {
		t.Errorf("MinSize = %v, want %v", got, want)
	}
}

func TestColumnsLayout(t *testing.T) {
	test.NewTempApp(t)
	gap := columnGap()
	// the middle column takes the width the 40 and 60 wide columns leave
	l := &columnsLayout{widths: []float32{40, 60}, flex: 1}
	objects := []fyne.CanvasObject{newTestTile(10), newTestTile(30), newTestTile(20)}

	l.Layout(objects, fyne.NewSize(300, 30))
	flexWidth := 300 - 40 - 60 - gap*2
	for i, want := range []struct{ pos, size fyne.Size }{
		{fyne.NewSize(0, 10), fyne.NewSize(40, 10)},
		{fyne.NewSize(40+gap, 0), fyne.NewSize(flexWidth, 30)},
		{fyne.NewSize(40+gap+flexWidth+gap, 5), fyne.NewSize(60, 20)},
	} {
		o := objects[i]
		if pos := fyne.NewSize(o.Position().X, o.Position().Y); pos != want.pos || o.Size() != want.size {
			t.Errorf("column %d at %v sized %v, want at %v sized %v", i, pos, o.Size(), want.pos, want.size)
		}
	}
	if got, want := l.heightForWidth(objects, 300), float32(30); got != want {
		t.Errorf("heightForWidth = %v, want %v", got, want)
	}
}
