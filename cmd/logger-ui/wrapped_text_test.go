package main_test

import (
	"slices"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	ui "github.com/gavinwade12/ecLogger/cmd/logger-ui"
)

func TestLongNamesWrapAtSpacesToFitTheirColumn(t *testing.T) {
	ta := newTestApp(t)
	ta.connect()
	ta.show(ui.TabParameters)
	const name = "Quantity Correction Cylinder #1"
	row := ta.paramRow(name)

	ta.do(func() {
		if got := texts(row.cells); !slices.Contains(got, name) {
			t.Errorf("row shows %q with room for the name, want it on one line", got)
		}
	})

	ta.resize(500, 600)
	ta.do(func() {
		// the lines of the name, and the column they're in
		var lines []*canvas.Text
		var column fyne.CanvasObject
		walk(row.cells, func(o fyne.CanvasObject, ancestors []fyne.CanvasObject) {
			if line, ok := o.(*canvas.Text); ok && line.Text != "" && strings.Contains(name, line.Text) {
				lines = append(lines, line)
				column = ancestors[len(ancestors)-1]
			}
		})
		if len(lines) < 2 {
			t.Fatalf("row shows %q in a narrow window, want the name wrapped", texts(row.cells))
		}

		// only at spaces, so no word is split
		var words []string
		for _, line := range lines {
			words = append(words, line.Text)
		}
		if got := strings.Join(words, " "); got != name {
			t.Errorf("name wrapped to %q, want the same words", words)
		}
		// and each line fits the column, unless it's one word, which isn't
		// split however long
		for _, line := range lines {
			w := fyne.MeasureText(line.Text, line.TextSize, line.TextStyle).Width
			if w > column.Size().Width && strings.Contains(line.Text, " ") {
				t.Errorf("line %q is %v wide in a %v wide column", line.Text, w, column.Size().Width)
			}
		}
	})
}
