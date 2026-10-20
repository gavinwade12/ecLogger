package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// listLayout stacks objects at its full width, each as tall as it is at
// that width.
type listLayout struct {
	// width is the width last laid out at, which MinSize needs to know
	// how tall the objects are.
	width float32
	// gap is the space between objects.
	gap float32
	// minSize is the MinSize since the last layout, if measured is set.
	// A scroll asks for its content's MinSize several times each time it
	// scrolls, and measuring every object each time makes scrolling lag.
	// A container lays out again when its objects change or it's
	// refreshed, which clears it.
	minSize  fyne.Size
	measured bool
}

func newListLayout(gap float32) *listLayout {
	return &listLayout{gap: gap}
}

func (l *listLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	l.width = size.Width
	l.measured = false
	var y float32
	for _, o := range objects {
		if !o.Visible() {
			continue
		}
		height := tileHeight(o, size.Width)
		o.Move(fyne.NewPos(0, y))
		o.Resize(fyne.NewSize(size.Width, height))
		y += height + l.gap
	}
}

func (l *listLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if l.measured {
		return l.minSize
	}
	var size fyne.Size
	for _, o := range objects {
		if !o.Visible() {
			continue
		}
		min := o.MinSize()
		if size.Height > 0 {
			size.Height += l.gap
		}
		size.Width = max(size.Width, min.Width)
		size.Height += tileHeight(o, max(l.width, min.Width))
	}
	l.minSize, l.measured = size, true
	return size
}

// columnsLayout lays objects out side by side in columns. The column at
// flex takes the width the others leave, and the others are as wide as
// widths. Each object is as tall as its MinSize, centered in the row.
type columnsLayout struct {
	widths []float32
	flex   int
}

// columnWidths returns each column's width when the row is width wide.
func (l *columnsLayout) columnWidths(width float32) []float32 {
	widths := make([]float32, len(l.widths)+1)
	flexWidth := width - columnGap()*float32(len(l.widths))
	for i, j := 0, 0; i < len(widths); i++ {
		if i == l.flex {
			continue
		}
		widths[i] = l.widths[j]
		flexWidth -= l.widths[j]
		j++
	}
	widths[l.flex] = max(flexWidth, 0)
	return widths
}

func (l *columnsLayout) heightForWidth(objects []fyne.CanvasObject, width float32) float32 {
	var height float32
	for i, w := range l.columnWidths(width) {
		height = max(height, tileHeight(objects[i], w))
	}
	return height
}

func (l *columnsLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	var x float32
	for i, w := range l.columnWidths(size.Width) {
		o := objects[i]
		height := tileHeight(o, w)
		o.Move(fyne.NewPos(x, (size.Height-height)/2))
		o.Resize(fyne.NewSize(w, height))
		x += w + columnGap()
	}
}

func (l *columnsLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var width float32
	for _, w := range l.widths {
		width += w + columnGap()
	}
	width += objects[l.flex].MinSize().Width
	return fyne.NewSize(width, l.heightForWidth(objects, width))
}

// columnGap is the space between columns.
func columnGap() float32 {
	return theme.InnerPadding() * 2
}

// columnsRow is a row of cells laid out by a columnsLayout, inset from its
// background.
type columnsRow struct {
	widget.BaseWidget
	background      *canvas.Rectangle
	backgroundColor fyne.ThemeColorName
	layout          *columnsLayout
	cells           *fyne.Container
	// verticalInset is the space above and below the cells.
	verticalInset fyne.ThemeSizeName
}

// newColumnsRow returns a row of cells in columns as wide as widths, but
// for the column at flex, which takes the width the others leave. The
// background is drawn in backgroundColor, if it isn't empty.
func newColumnsRow(widths []float32, flex int, backgroundColor fyne.ThemeColorName, cells ...fyne.CanvasObject) *columnsRow {
	r := &columnsRow{
		background:      canvas.NewRectangle(nil),
		backgroundColor: backgroundColor,
		layout:          &columnsLayout{widths: widths, flex: flex},
		verticalInset:   theme.SizeNamePadding,
	}
	r.cells = container.New(r.layout, cells...)
	r.ExtendBaseWidget(r)
	return r
}

// inset is the space between the row's edges and its cells.
func (r *columnsRow) inset() fyne.Size {
	th := r.Theme()
	return fyne.NewSize(th.Size(theme.SizeNameInnerPadding), th.Size(r.verticalInset))
}

func (r *columnsRow) heightForWidth(width float32) float32 {
	inset := r.inset()
	return r.layout.heightForWidth(r.cells.Objects, width-inset.Width*2) + inset.Height*2
}

func (r *columnsRow) MinSize() fyne.Size {
	return r.cells.MinSize().Add(r.inset().AddWidthHeight(r.inset().Width, r.inset().Height))
}

func (r *columnsRow) CreateRenderer() fyne.WidgetRenderer {
	renderer := &columnsRowRenderer{row: r}
	renderer.Refresh()
	return renderer
}

type columnsRowRenderer struct {
	row *columnsRow
}

func (r *columnsRowRenderer) Layout(size fyne.Size) {
	inset := r.row.inset()
	r.row.background.Resize(size)
	r.row.cells.Move(fyne.NewPos(inset.Width, inset.Height))
	r.row.cells.Resize(size.SubtractWidthHeight(inset.Width*2, inset.Height*2))
}

func (r *columnsRowRenderer) MinSize() fyne.Size {
	return r.row.MinSize()
}

func (r *columnsRowRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.row.background, r.row.cells}
}

func (r *columnsRowRenderer) Refresh() {
	r.row.background.FillColor = color.Transparent
	if r.row.backgroundColor != "" {
		th, v := r.row.Theme(), fyne.CurrentApp().Settings().ThemeVariant()
		r.row.background.FillColor = th.Color(r.row.backgroundColor, v)
		r.row.background.CornerRadius = th.Size(theme.SizeNameInputRadius)
	}
	r.row.background.Refresh()
	r.row.cells.Refresh()
}

func (r *columnsRowRenderer) Destroy() {}

// maxWidthLayout lays objects out at its full height and the full width,
// up to sizeNameFormMaxWidth, on its leading edge.
type maxWidthLayout struct{}

// newMaxWidth returns content no wider than sizeNameFormMaxWidth.
func newMaxWidth(content fyne.CanvasObject) *fyne.Container {
	return container.New(maxWidthLayout{}, content)
}

func (maxWidthLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	width := min(size.Width, theme.Size(sizeNameFormMaxWidth))
	for _, o := range objects {
		o.Move(fyne.NewPos(0, 0))
		o.Resize(fyne.NewSize(width, size.Height))
	}
}

func (maxWidthLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var size fyne.Size
	for _, o := range objects {
		size = size.Max(o.MinSize())
	}
	return size
}
