package main

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
)

// hintRows returns the rows of text a hint draws, as the canvas sees them.
func hintRows(h *hint) []*canvas.Text {
	var rows []*canvas.Text
	for _, o := range test.WidgetRenderer(h).Objects() {
		if t, ok := o.(*canvas.Text); ok && t.Text != "" {
			rows = append(rows, t)
		}
	}
	return rows
}

func TestHintShowsEveryRowAfterItsTextGrows(t *testing.T) {
	app := test.NewTempApp(t)
	app.Settings().SetTheme(newAppTheme())

	h := newHint("")
	h.Wrapping = fyne.TextWrapWord
	w := test.NewWindow(container.NewVBox(h))
	defer w.Close()
	w.Resize(fyne.NewSize(300, 400))

	h.SetText("Short.")
	if got := hintRows(h); len(got) != 1 {
		t.Fatalf("short hint shows %d rows, want 1", len(got))
	}

	long := "This hint is long enough that it has to wrap over several rows at this width."
	h.SetError(long)
	rows := hintRows(h)
	if len(rows) < 2 {
		t.Fatalf("long hint shows %d rows, want it wrapped", len(rows))
	}
	var words []string
	for _, r := range rows {
		words = append(words, r.Text)
		if r.Color != app.Settings().Theme().Color(theme.ColorNameError, app.Settings().ThemeVariant()) {
			t.Errorf("row %q isn't drawn as an error", r.Text)
		}
		if r.Alignment != fyne.TextAlignCenter {
			t.Errorf("row %q isn't centered", r.Text)
		}
	}
	if got := strings.Join(words, " "); got != long {
		t.Errorf("hint shows %q, want %q", got, long)
	}
}
