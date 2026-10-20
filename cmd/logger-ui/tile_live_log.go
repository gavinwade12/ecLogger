package main

import (
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
	name  *wrappedText
}

func newLiveLogTile(m *liveLogModel) *liveLogTile {
	t := &liveLogTile{
		model: m,
		name:  newWrappedText(m.Name, theme.SizeNameText, theme.ColorNameForeground),
	}
	t.ExtendBaseWidget(t)
	return t
}

// tileMetrics are the sizes a tile is laid out with.
type tileMetrics struct {
	padding, gap              float32
	textSize, valueSize       float32
	detailSize                float32
	valueHeight, detailHeight float32
	minTileWidth              float32
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
	m.valueHeight = fyne.MeasureText("0", m.valueSize, valueStyle).Height
	m.detailHeight = fyne.MeasureText("0", m.detailSize, fyne.TextStyle{}).Height
	return m
}

var valueStyle = fyne.TextStyle{Bold: true}

// heightForWidth returns how tall the tile is at width, which depends on
// how many lines its name wraps to.
func (t *liveLogTile) heightForWidth(width float32) float32 {
	m := t.metrics()
	return m.padding + t.name.heightForWidth(width-m.padding*2) + m.gap*2 +
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
	tile                         *liveLogTile
	background                   *canvas.Rectangle
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

	nameWidth := size.Width - m.padding*2
	r.tile.name.Move(fyne.NewPos(m.padding, m.padding))
	r.tile.name.Resize(fyne.NewSize(nameWidth, r.tile.name.heightForWidth(nameWidth)))

	// the value and details sit at the bottom, so they line up across a
	// row of tiles whose names wrap to different numbers of lines
	detailY := size.Height - m.padding - m.detailHeight
	r.minLabel.Move(fyne.NewPos(m.padding, detailY))
	r.min.Move(fyne.NewPos(m.padding+r.minLabel.MinSize().Width+m.gap, detailY))
	maxX := size.Width - m.padding - r.max.MinSize().Width
	r.max.Move(fyne.NewPos(maxX, detailY))
	r.maxLabel.Move(fyne.NewPos(maxX-m.gap-r.maxLabel.MinSize().Width, detailY))

	// the value and unit are right-aligned, so neither moves as the value
	// gains or loses digits
	valueY := detailY - m.gap - m.valueHeight
	unitX := size.Width - m.padding - r.unit.MinSize().Width
	valueX := unitX - r.value.MinSize().Width
	if r.unit.Text != "" {
		valueX -= m.gap
	}
	r.value.Move(fyne.NewPos(valueX, valueY))
	// line the unit's baseline up with the value's
	driver := fyne.CurrentApp().Driver()
	_, valueBaseline := driver.RenderedTextSize(r.value.Text, m.valueSize, valueStyle, nil)
	_, unitBaseline := driver.RenderedTextSize(r.unit.Text, m.textSize, fyne.TextStyle{}, nil)
	r.unit.Move(fyne.NewPos(unitX, valueY+valueBaseline-unitBaseline))

	for _, o := range r.texts() {
		o.Resize(o.MinSize())
	}
}

func (r *liveLogTileRenderer) texts() []*canvas.Text {
	return []*canvas.Text{r.value, r.unit, r.minLabel, r.min, r.maxLabel, r.max}
}

func (r *liveLogTileRenderer) MinSize() fyne.Size {
	return r.tile.MinSize()
}

func (r *liveLogTileRenderer) Objects() []fyne.CanvasObject {
	objects := []fyne.CanvasObject{r.background, r.tile.name}
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

	r.Layout(r.tile.Size())
	r.background.Refresh()
	for _, t := range r.texts() {
		t.Refresh()
	}
}
