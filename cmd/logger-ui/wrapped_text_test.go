package main

import (
	"reflect"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
)

func TestWrapText(t *testing.T) {
	test.NewTempApp(t)
	width := fyne.MeasureText("Quantity Correction", 14, fyne.TextStyle{}).Width

	for _, tt := range []struct {
		text string
		want []string
	}{
		{"", []string{""}},
		{"Engine Speed", []string{"Engine Speed"}},
		{"Quantity Correction Cylinder #1", []string{"Quantity Correction", "Cylinder #1"}},
		// a word too long for a line overflows rather than being split
		{"Roughness QuantityCorrectionCylinder", []string{"Roughness", "QuantityCorrectionCylinder"}},
	} {
		if got := wrapText(tt.text, width, 14, fyne.TextStyle{}); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("wrapText(%q) = %q, want %q", tt.text, got, tt.want)
		}
	}
}

func TestWrappedText(t *testing.T) {
	test.NewTempApp(t)
	text := newWrappedText("Quantity Correction Cylinder #1", theme.SizeNameText, theme.ColorNameForeground)
	width := fyne.MeasureText("Quantity Correction", 14, fyne.TextStyle{}).Width
	lineHeight := fyne.MeasureText("Ag", 14, fyne.TextStyle{}).Height

	if got, want := text.heightForWidth(width), 2*lineHeight; got != want {
		t.Errorf("height wrapped to two lines = %v, want %v", got, want)
	}
	if got, want := text.heightForWidth(1000), lineHeight; got != want {
		t.Errorf("height on one line = %v, want %v", got, want)
	}

	text.Resize(fyne.NewSize(width, text.heightForWidth(width)))
	var lines []string
	for _, o := range test.TempWidgetRenderer(t, text).Objects() {
		lines = append(lines, o.(*canvas.Text).Text)
	}
	if want := []string{"Quantity Correction", "Cylinder #1"}; !reflect.DeepEqual(lines, want) {
		t.Errorf("lines = %q, want %q", lines, want)
	}
}
