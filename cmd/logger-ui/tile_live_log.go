package main

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// liveLogTile shows a live logged parameter: its name, its current value
// and unit, and the lowest and highest values seen.
type liveLogTile struct {
	widget.BaseWidget
	model *liveLogModel
}

func newLiveLogTile(m *liveLogModel) *liveLogTile {
	t := &liveLogTile{model: m}
	t.ExtendBaseWidget(t)
	return t
}

// tileMetrics are the sizes a tile is laid out with.
type tileMetrics struct {
	padding, gap               float32
	textSize, valueSize        float32
	detailSize                 float32
	lineHeight, valueHeight    float32
	detailHeight, minTileWidth float32
}

func (t *liveLogTile) metrics() tileMetrics {
	th := t.Theme()
	m := tileMetrics{
		padding:      th.Size(theme.SizeNameInnerPadding),
		gap:          th.Size(theme.SizeNamePadding),
		textSize:     th.Size(theme.SizeNameText),
		valueSize:    th.Size(sizeNameTileValueText),
		detailSize:   th.Size(sizeNameTileDetailText),
		minTileWidth: th.Size(sizeNameTileMinWidth),
	}
	m.lineHeight = fyne.MeasureText("Ag", m.textSize, fyne.TextStyle{}).Height
	m.valueHeight = fyne.MeasureText("0", m.valueSize, valueStyle).Height
	m.detailHeight = fyne.MeasureText("0", m.detailSize, fyne.TextStyle{}).Height
	return m
}

var valueStyle = fyne.TextStyle{Bold: true}

// heightForWidth returns how tall the tile is at width, which depends on
// how many lines its name wraps to.
func (t *liveLogTile) heightForWidth(width float32) float32 {
	m := t.metrics()
	lines := wrapText(t.model.Name, width-m.padding*2, m.textSize, fyne.TextStyle{})
	return m.padding + float32(len(lines))*m.lineHeight + m.gap*2 +
		m.valueHeight + m.gap + m.detailHeight + m.padding
}

func (t *liveLogTile) MinSize() fyne.Size {
	width := t.metrics().minTileWidth
	return fyne.NewSize(width, t.heightForWidth(width))
}

func (t *liveLogTile) CreateRenderer() fyne.WidgetRenderer {
	text := func(s string) *canvas.Text {
		return canvas.NewText(s, nil)
	}
	r := &liveLogTileRenderer{
		tile:       t,
		background: canvas.NewRectangle(nil),
		value:      text(""),
		unit:       text(""),
		minLabel:   text("min"),
		min:        text(""),
		maxLabel:   text("max"),
		max:        text(""),
	}
	r.value.TextStyle = valueStyle
	r.bind(t.model.CurrentValueBinding, r.value)
	r.bind(t.model.UnitBinding, r.unit)
	r.bind(t.model.MinValueBinding, r.min)
	r.bind(t.model.MaxValueBinding, r.max)
	r.Refresh()
	return r
}

type liveLogTileRenderer struct {
	tile       *liveLogTile
	background *canvas.Rectangle
	// name holds a line of text for each line the name wraps to
	name                         []*canvas.Text
	value, unit                  *canvas.Text
	minLabel, min, maxLabel, max *canvas.Text
	listeners                    []binding.DataListener
	bindings                     []binding.String
}

// bind shows b's value in text, laying the tile out again as its width
// changes.
func (r *liveLogTileRenderer) bind(b binding.String, text *canvas.Text) {
	l := binding.NewDataListener(func() {
		s, _ := b.Get()
		if s == text.Text {
			return
		}
		text.Text = s
		r.Layout(r.tile.Size())
		canvas.Refresh(text)
	})
	b.AddListener(l)
	r.bindings = append(r.bindings, b)
	r.listeners = append(r.listeners, l)
}

func (r *liveLogTileRenderer) Destroy() {
	for i, b := range r.bindings {
		b.RemoveListener(r.listeners[i])
	}
}

func (r *liveLogTileRenderer) Layout(size fyne.Size) {
	m := r.tile.metrics()
	r.background.Resize(size)

	lines := wrapText(r.tile.model.Name, size.Width-m.padding*2, m.textSize, fyne.TextStyle{})
	r.setNameLines(lines)
	for i, line := range r.name {
		line.Move(fyne.NewPos(m.padding, m.padding+float32(i)*m.lineHeight))
	}

	// the value and details sit at the bottom, so they line up across a
	// row of tiles whose names wrap to different numbers of lines
	detailY := size.Height - m.padding - m.detailHeight
	r.minLabel.Move(fyne.NewPos(m.padding, detailY))
	r.min.Move(fyne.NewPos(m.padding+r.minLabel.MinSize().Width+m.gap, detailY))
	maxX := size.Width - m.padding - r.max.MinSize().Width
	r.max.Move(fyne.NewPos(maxX, detailY))
	r.maxLabel.Move(fyne.NewPos(maxX-m.gap-r.maxLabel.MinSize().Width, detailY))

	valueY := detailY - m.gap - m.valueHeight
	r.value.Move(fyne.NewPos(m.padding, valueY))
	// line the unit's baseline up with the value's
	driver := fyne.CurrentApp().Driver()
	_, valueBaseline := driver.RenderedTextSize(r.value.Text, m.valueSize, valueStyle, nil)
	_, unitBaseline := driver.RenderedTextSize(r.unit.Text, m.textSize, fyne.TextStyle{}, nil)
	r.unit.Move(fyne.NewPos(m.padding+r.value.MinSize().Width+m.gap, valueY+valueBaseline-unitBaseline))

	for _, o := range r.texts() {
		o.Resize(o.MinSize())
	}
}

// setNameLines shows lines as the name, reusing the existing line texts.
func (r *liveLogTileRenderer) setNameLines(lines []string) {
	for len(r.name) < len(lines) {
		line := canvas.NewText("", nil)
		styleText(line, r.tile.Theme(), theme.SizeNameText, theme.ColorNameForeground)
		r.name = append(r.name, line)
	}
	r.name = r.name[:len(lines)]
	for i, s := range lines {
		r.name[i].Text = s
	}
}

func styleText(text *canvas.Text, th fyne.Theme, size fyne.ThemeSizeName, color fyne.ThemeColorName) {
	v := fyne.CurrentApp().Settings().ThemeVariant()
	text.TextSize = th.Size(size)
	text.Color = th.Color(color, v)
}

func (r *liveLogTileRenderer) texts() []*canvas.Text {
	return append([]*canvas.Text{r.value, r.unit, r.minLabel, r.min, r.maxLabel, r.max}, r.name...)
}

func (r *liveLogTileRenderer) MinSize() fyne.Size {
	return r.tile.MinSize()
}

func (r *liveLogTileRenderer) Objects() []fyne.CanvasObject {
	objects := []fyne.CanvasObject{r.background}
	for _, t := range r.texts() {
		objects = append(objects, t)
	}
	return objects
}

func (r *liveLogTileRenderer) Refresh() {
	th := r.tile.Theme()
	v := fyne.CurrentApp().Settings().ThemeVariant()
	r.background.FillColor = th.Color(colorNameTile, v)
	r.background.CornerRadius = th.Size(theme.SizeNameInputRadius)

	styleText(r.value, th, sizeNameTileValueText, theme.ColorNameForeground)
	styleText(r.unit, th, theme.SizeNameText, colorNameSecondaryText)
	styleText(r.minLabel, th, sizeNameTileDetailText, colorNameSecondaryText)
	styleText(r.min, th, sizeNameTileDetailText, theme.ColorNameForeground)
	styleText(r.maxLabel, th, sizeNameTileDetailText, colorNameSecondaryText)
	styleText(r.max, th, sizeNameTileDetailText, theme.ColorNameForeground)
	for _, line := range r.name {
		styleText(line, th, theme.SizeNameText, theme.ColorNameForeground)
	}

	r.Layout(r.tile.Size())
	r.background.Refresh()
	for _, t := range r.texts() {
		t.Refresh()
	}
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
