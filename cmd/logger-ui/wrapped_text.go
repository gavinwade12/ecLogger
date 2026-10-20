package main

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// wrappedText is text that wraps at spaces to fit its width. Unlike a
// wrapped widget.Label, it can say how tall it is at any width, so a
// layout can size it before laying it out.
type wrappedText struct {
	widget.BaseWidget
	text      string
	sizeName  fyne.ThemeSizeName
	colorName fyne.ThemeColorName
	style     fyne.TextStyle
	alignment fyne.TextAlign
}

func newWrappedText(text string, sizeName fyne.ThemeSizeName, colorName fyne.ThemeColorName) *wrappedText {
	w := &wrappedText{text: text, sizeName: sizeName, colorName: colorName}
	w.ExtendBaseWidget(w)
	return w
}

func (w *wrappedText) textSize() float32 {
	return w.Theme().Size(w.sizeName)
}

func (w *wrappedText) lines(width float32) []string {
	return wrapText(w.text, width, w.textSize(), w.style)
}

func (w *wrappedText) lineHeight() float32 {
	return fyne.MeasureText("Ag", w.textSize(), w.style).Height
}

// heightForWidth returns how tall the text is when wrapped to width.
func (w *wrappedText) heightForWidth(width float32) float32 {
	return float32(len(w.lines(width))) * w.lineHeight()
}

// MinSize is as wide as the widest word, and as tall as the text is at its
// current width.
func (w *wrappedText) MinSize() fyne.Size {
	var widest float32
	for _, word := range strings.Fields(w.text) {
		widest = max(widest, fyne.MeasureText(word, w.textSize(), w.style).Width)
	}
	return fyne.NewSize(widest, w.heightForWidth(max(w.Size().Width, widest)))
}

func (w *wrappedText) CreateRenderer() fyne.WidgetRenderer {
	r := &wrappedTextRenderer{text: w}
	r.Refresh()
	return r
}

type wrappedTextRenderer struct {
	text *wrappedText
	// lines holds a canvas text for each line the text wraps to
	lines []*canvas.Text
}

func (r *wrappedTextRenderer) Layout(size fyne.Size) {
	lines := r.text.lines(size.Width)
	for len(r.lines) < len(lines) {
		r.lines = append(r.lines, canvas.NewText("", nil))
	}
	r.lines = r.lines[:len(lines)]

	th := r.text.Theme()
	lineHeight := r.text.lineHeight()
	for i, s := range lines {
		line := r.lines[i]
		line.Text = s
		line.TextStyle = r.text.style
		styleText(line, th, r.text.sizeName, r.text.colorName)
		line.Resize(line.MinSize())
		var x float32
		switch r.text.alignment {
		case fyne.TextAlignCenter:
			x = (size.Width - line.Size().Width) / 2
		case fyne.TextAlignTrailing:
			x = size.Width - line.Size().Width
		}
		line.Move(fyne.NewPos(x, float32(i)*lineHeight))
	}
}

func (r *wrappedTextRenderer) MinSize() fyne.Size {
	return r.text.MinSize()
}

func (r *wrappedTextRenderer) Objects() []fyne.CanvasObject {
	objects := make([]fyne.CanvasObject, len(r.lines))
	for i, line := range r.lines {
		objects[i] = line
	}
	return objects
}

func (r *wrappedTextRenderer) Refresh() {
	r.Layout(r.text.Size())
	for _, line := range r.lines {
		line.Refresh()
	}
}

func (r *wrappedTextRenderer) Destroy() {}

// styleText sets text's size and color from th.
func styleText(text *canvas.Text, th fyne.Theme, size fyne.ThemeSizeName, color fyne.ThemeColorName) {
	v := fyne.CurrentApp().Settings().ThemeVariant()
	text.TextSize = th.Size(size)
	text.Color = th.Color(color, v)
}

// wrapText splits text into lines at spaces so each line fits in width.
// A word too long for a line gets a line to itself.
func wrapText(text string, width, size float32, style fyne.TextStyle) []string {
	var lines []string
	line := ""
	for _, word := range strings.Fields(text) {
		next := word
		if line != "" {
			next = line + " " + word
		}
		if line == "" || fyne.MeasureText(next, size, style).Width <= width {
			line = next
			continue
		}
		lines = append(lines, line)
		line = word
	}
	if line != "" || len(lines) == 0 {
		lines = append(lines, line)
	}
	return lines
}
