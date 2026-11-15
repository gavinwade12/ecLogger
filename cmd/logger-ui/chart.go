package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/gavinwade12/ecLogger/internal/logging"
	"github.com/gavinwade12/ecLogger/units"
	"golang.org/x/image/vector"
)

// A chartLane is one parameter drawn in its own lane. It holds functions
// rather than data so the chart doesn't care whether the values come from a
// log file or a live window.
type chartLane struct {
	Name string
	Unit units.Unit
	// Downsample reduces the lane's values in [from, to) to n buckets, as
	// logging.Downsample does.
	Downsample func(from, to time.Time, n int) logging.Downsampled
	// ValueAt returns the finite value nearest to t within tol of it, for
	// the cursor's readout. ok is false if there is none.
	ValueAt func(t time.Time, tol time.Duration) (v float64, ok bool)
}

// paramTitle names a parameter with its unit, like "Engine Speed (rpm)", or
// just its name when it has no unit.
func paramTitle(name string, unit units.Unit) string {
	if unit == "" {
		return name
	}
	return fmt.Sprintf("%s (%s)", name, unit)
}

// lanesFromLog returns a lane for each of l's columns listed in columns, in
// that order. Indexes out of range are skipped.
func lanesFromLog(l *logging.Log, columns []int) []chartLane {
	lanes := make([]chartLane, 0, len(columns))
	for _, i := range columns {
		if i < 0 || i >= len(l.Columns) {
			continue
		}
		c := l.Columns[i]
		lanes = append(lanes, chartLane{
			Name: c.Name,
			Unit: c.Unit,
			Downsample: func(from, to time.Time, n int) logging.Downsampled {
				return logging.Downsample(l.Times, c.Values, from, to, n)
			},
			ValueAt: func(t time.Time, tol time.Duration) (float64, bool) {
				return logging.ValueAt(l.Times, c.Values, t, tol)
			},
		})
	}
	return lanes
}

// A timeLabeler names a moment on the chart's time axis. step is the spacing
// of the ticks being labeled, so a label can show just enough precision. An
// empty name leaves the tick unlabeled.
type timeLabeler func(t time.Time, step time.Duration) string

// elapsedLabels labels times by how long after start they are, like 1:30 or
// 1:02:00, which is how a log file is read.
func elapsedLabels(start time.Time) timeLabeler {
	return func(t time.Time, step time.Duration) string {
		return formatElapsed(t.Sub(start), step)
	}
}

// formatElapsed formats d as m:ss, or h:mm:ss from an hour on, with a
// fraction of a second when step is shorter than a second. A negative d
// has a leading minus.
func formatElapsed(d, step time.Duration) string {
	sign := ""
	if d < 0 {
		sign, d = "-", -d
	}
	decimals := 0
	switch {
	case step > 0 && step < 10*time.Millisecond:
		decimals = 3
	case step > 0 && step < 100*time.Millisecond:
		decimals = 2
	case step > 0 && step < time.Second:
		decimals = 1
	}
	d = d.Round(time.Second / time.Duration(math.Pow10(decimals)))
	h, m, s := int(d/time.Hour), int(d/time.Minute%60), (d % time.Minute).Seconds()
	// the width of the seconds: two digits, a point and the decimals
	width := 2
	if decimals > 0 {
		width += 1 + decimals
	}
	secs := fmt.Sprintf("%0*.*f", width, decimals, s)
	if h > 0 {
		return fmt.Sprintf("%s%d:%02d:%s", sign, h, m, secs)
	}
	return fmt.Sprintf("%s%d:%s", sign, m, secs)
}

// timeSteps are the spacings time ticks may have.
var timeSteps = []time.Duration{
	100 * time.Millisecond, 200 * time.Millisecond, 500 * time.Millisecond,
	time.Second, 2 * time.Second, 5 * time.Second, 10 * time.Second,
	15 * time.Second, 30 * time.Second,
	time.Minute, 2 * time.Minute, 5 * time.Minute, 10 * time.Minute,
	15 * time.Minute, 30 * time.Minute,
	time.Hour, 2 * time.Hour, 3 * time.Hour, 6 * time.Hour, 12 * time.Hour,
	24 * time.Hour,
}

// timeTicks returns the smallest step from timeSteps that gives at most
// maxTicks ticks across [from, to], and the ticks: the multiples of step
// after origin that fall in the range. Counting from origin rather than from
// the range keeps ticks put as the range pans.
func timeTicks(origin, from, to time.Time, maxTicks int) (ticks []time.Time, step time.Duration) {
	span := to.Sub(from)
	if span <= 0 || maxTicks < 1 {
		return nil, 0
	}
	step = timeSteps[len(timeSteps)-1]
	for _, s := range timeSteps {
		if int(span/s)+1 <= maxTicks {
			step = s
			break
		}
	}
	first := int64(math.Ceil(float64(from.Sub(origin)) / float64(step)))
	for k := first; ; k++ {
		t := origin.Add(time.Duration(k) * step)
		if t.After(to) {
			break
		}
		ticks = append(ticks, t)
	}
	return ticks, step
}

// laneChart draws parameters as lanes stacked top to bottom, each with its
// own y-axis and all sharing one time axis along the bottom.
//
// The traces and grid are drawn into one raster, one texture per frame
// however many samples there are. The text is canvas.Text so it follows the
// theme's font.
type laneChart struct {
	widget.BaseWidget
	lanes    []chartLane
	from, to time.Time
	origin   time.Time
	label    timeLabeler
	// ranges are the lanes' value ranges as of the last refresh. They are
	// kept so laying out again, which happens on every resize, doesn't walk
	// every value.
	ranges []laneRange

	cursorState
	chartNav
	// plotLeft, plotRight and plotBottom are where the plot was laid out,
	// for turning pointer positions into times
	plotLeft, plotRight, plotBottom float32
	rend                            *laneChartRenderer
}

type laneRange struct {
	min, max float64
	ok       bool
}

func newLaneChart() *laneChart {
	c := &laneChart{}
	c.ExtendBaseWidget(c)
	return c
}

// SetLanes sets the parameters to draw, top lane first.
func (c *laneChart) SetLanes(lanes []chartLane) {
	c.lanes = append([]chartLane(nil), lanes...)
	c.ranges = nil
	c.readRanges()
	c.Refresh()
}

// SetTimeRange sets the span of time shown across the chart's width.
func (c *laneChart) SetTimeRange(from, to time.Time) {
	c.from, c.to = from, to
	c.readRanges()
	c.Refresh()
}

// SetTimeAxis sets how the time axis is labeled. Ticks fall on whole steps
// after origin, and label names each one. For a log file, origin is the
// start of the log and label is elapsedLabels(origin).
func (c *laneChart) SetTimeAxis(origin time.Time, label timeLabeler) {
	c.origin, c.label = origin, label
	c.Refresh()
}

// SetView moves the chart on to [from, to] with a time axis anchored at
// origin, and reads each lane's value range again, redrawing once. It's how a
// live chart follows lanes whose values keep changing, without SetLanes's
// rebuilding of the lanes.
func (c *laneChart) SetView(from, to, origin time.Time, label timeLabeler) {
	c.from, c.to, c.origin, c.label = from, to, origin, label
	c.readRanges()
	c.Refresh()
}

// Rescale reads each lane's value range again and redraws, for when the
// lanes' values have changed under a view that hasn't.
func (c *laneChart) Rescale() {
	c.readRanges()
	c.Refresh()
}

// readRanges sets each lane's value range to that of the values in the view,
// so zooming in on part of a log scales the lanes to what's visible. It
// reads one bucket spanning the view, which costs the visible samples and
// no more.
func (c *laneChart) readRanges() {
	c.ranges = c.ranges[:0]
	for _, l := range c.lanes {
		var r laneRange
		if l.Downsample != nil && c.to.After(c.from) {
			r = visibleRange(l.Downsample(c.from, c.to, 1))
		}
		c.ranges = append(c.ranges, r)
	}
}

// visibleRange returns the range of the values in d, which is a single
// bucket. A view with no samples in it takes the samples either side, which
// are what the trace crossing it joins.
func visibleRange(d logging.Downsampled) laneRange {
	var r laneRange
	add := func(lo, hi float64) {
		if !r.ok {
			r = laneRange{lo, hi, true}
			return
		}
		r.min, r.max = math.Min(r.min, lo), math.Max(r.max, hi)
	}
	if len(d.Buckets) > 0 && d.Buckets[0].N > 0 {
		add(d.Buckets[0].Min, d.Buckets[0].Max)
		return r
	}
	if d.Before != nil {
		add(d.Before.V, d.Before.V)
	}
	if d.After != nil {
		add(d.After.V, d.After.V)
	}
	return r
}

// labeler returns how the time axis is labeled, and where its ticks count
// from, with the defaults for a chart that hasn't been told.
func (c *laneChart) labeler() (timeLabeler, time.Time) {
	label, origin := c.label, c.origin
	if label == nil {
		label = elapsedLabels(c.from)
	}
	if origin.IsZero() {
		origin = c.from
	}
	return label, origin
}

// chartMetrics are the sizes the chart is laid out with, in Fyne units.
type chartMetrics struct {
	pad, lineWidth           float32
	titleSize, tickSize      float32
	titleHeight, tickHeight  float32
	laneMinHeight, minWidth  float32
	timeAxisHeight, titleRow float32
}

func (c *laneChart) metrics() chartMetrics {
	th := c.Theme()
	m := chartMetrics{
		pad:           th.Size(theme.SizeNamePadding),
		lineWidth:     th.Size(sizeNameChartLineWidth),
		titleSize:     th.Size(theme.SizeNameText),
		tickSize:      th.Size(theme.SizeNameCaptionText),
		laneMinHeight: th.Size(sizeNameChartLaneMinHeight),
		minWidth:      th.Size(sizeNameChartMinWidth),
	}
	m.titleHeight = fyne.MeasureText("0", m.titleSize, fyne.TextStyle{}).Height
	m.tickHeight = fyne.MeasureText("0", m.tickSize, fyne.TextStyle{}).Height
	// The title sits above the plot, and the plot's ends leave room for
	// half a tick label each, so the top and bottom labels aren't cut off.
	m.titleRow = m.titleHeight + m.pad
	m.timeAxisHeight = m.tickHeight + m.pad*2
	if floor := m.titleRow + m.tickHeight*4; m.laneMinHeight < floor {
		m.laneMinHeight = floor
	}
	return m
}

func (c *laneChart) MinSize() fyne.Size {
	m := c.metrics()
	return fyne.NewSize(m.minWidth, float32(len(c.lanes))*m.laneMinHeight+m.timeAxisHeight)
}

func (c *laneChart) CreateRenderer() fyne.WidgetRenderer {
	r := &laneChartRenderer{chart: c, z: vector.NewRasterizer(0, 0)}
	c.rend = r
	r.raster = canvas.NewRaster(r.generate)
	// Each raster pixel is a screen pixel, so don't blur it
	r.raster.ScaleMode = canvas.ImageScalePixels
	r.cursor = newCursorOverlay()
	r.objects = []fyne.CanvasObject{r.raster}
	return r
}

// A laneRow is the vertical extent of a lane, in Fyne units.
type laneRow struct {
	top, bottom float32 // the whole lane, title included
	plotTop     float32 // where the y-axis's highest value sits
	plotBottom  float32 // and its lowest
}

// laneRows divides height among n lanes, leaving room at the bottom for the
// time axis.
func laneRows(height float32, n int, m chartMetrics) []laneRow {
	if n == 0 {
		return nil
	}
	h := (height - m.timeAxisHeight) / float32(n)
	rows := make([]laneRow, n)
	for i := range rows {
		top := float32(i) * h
		rows[i] = laneRow{
			top:        top,
			bottom:     top + h,
			plotTop:    top + m.titleRow,
			plotBottom: top + h - m.tickHeight/2 - m.pad/2,
		}
	}
	return rows
}

// A laneAxis is a lane's y-axis: lo to hi, labeled every step.
type laneAxis struct {
	ok          bool
	decimals    int
	lo, hi      float64
	ticks       []float64
	labels      []string
	labelsWidth float32
}

// yAxis builds the axis for a lane whose finite values span r, with room
// for about as many ticks as fit in plotHeight.
func yAxis(r laneRange, plotHeight float32, m chartMetrics) laneAxis {
	if !r.ok {
		return laneAxis{}
	}
	maxTicks := int(plotHeight / (m.tickHeight * 1.7))
	maxTicks = max(2, min(5, maxTicks))
	lo, hi, step := logging.NiceAxis(r.min, r.max, maxTicks)
	a := laneAxis{ok: true, lo: lo, hi: hi, ticks: logging.Ticks(lo, hi, step)}

	// Enough decimals that neighbouring ticks read differently
	decimals := max(0, -int(math.Floor(math.Log10(step)+1e-9)))
	a.decimals = decimals
	for _, t := range a.ticks {
		if t == 0 {
			t = 0 // not "-0"
		}
		l := strconv.FormatFloat(t, 'f', decimals, 64)
		a.labels = append(a.labels, l)
		a.labelsWidth = max(a.labelsWidth, fyne.MeasureText(l, m.tickSize, fyne.TextStyle{}).Width)
	}
	return a
}

// y returns where v sits between the lane's plotTop and plotBottom.
func (a laneAxis) y(v float64, row laneRow) float32 {
	f := float32((v - a.lo) / (a.hi - a.lo))
	return row.plotBottom - f*(row.plotBottom-row.plotTop)
}

// chartFrame is everything the raster needs to draw a frame, worked out when
// the chart is laid out.
type chartFrame struct {
	plotLeft, plotRight float32
	rows                []laneRow
	axes                []laneAxis
	timeTicks           []time.Time
	timeStep            time.Duration
}

type laneChartRenderer struct {
	chart   *laneChart
	raster  *canvas.Raster
	objects []fyne.CanvasObject
	// texts are the labels, reused between layouts
	texts []*canvas.Text
	// axisTexts are the labels of the time axis's ticks, which the
	// cursor's time takes the place of where they'd overlap
	axisTexts []*canvas.Text
	frame     chartFrame
	// cursor is drawn over the raster, so moving it doesn't redraw that
	cursor *cursorOverlay

	img *image.RGBA
	z   *vector.Rasterizer
}

func (r *laneChartRenderer) Destroy() {}

func (r *laneChartRenderer) MinSize() fyne.Size { return r.chart.MinSize() }

func (r *laneChartRenderer) Objects() []fyne.CanvasObject { return r.objects }

func (r *laneChartRenderer) Refresh() {
	// The theme may have changed, which changes sizes and colors
	r.Layout(r.chart.Size())
	canvas.Refresh(r.raster)
}

// text returns the next unused label, set up to show s at x, y.
func (r *laneChartRenderer) text(used *int, s string, size float32, c color.Color, x, y float32) *canvas.Text {
	if *used == len(r.texts) {
		r.texts = append(r.texts, canvas.NewText("", nil))
	}
	t := r.texts[*used]
	*used++
	t.Text, t.TextSize, t.Color = s, size, c
	t.Show()
	t.Move(fyne.NewPos(x, y))
	t.Resize(t.MinSize())
	return t
}

func (r *laneChartRenderer) Layout(size fyne.Size) {
	c := r.chart
	m := c.metrics()
	th, v := c.Theme(), fyne.CurrentApp().Settings().ThemeVariant()
	fg, muted := th.Color(theme.ColorNameForeground, v), th.Color(theme.ColorNamePlaceHolder, v)

	r.raster.Move(fyne.NewPos(0, 0))
	r.raster.Resize(size)

	f := &r.frame
	f.rows = laneRows(size.Height, len(c.lanes), m)
	f.axes = f.axes[:0]
	var labelsWidth float32
	for i, row := range f.rows {
		a := yAxis(c.ranges[i], row.plotBottom-row.plotTop, m)
		f.axes = append(f.axes, a)
		labelsWidth = max(labelsWidth, a.labelsWidth)
	}
	// One gutter for every lane keeps their plots lined up
	f.plotLeft = labelsWidth + m.pad*2
	f.plotRight = size.Width - m.pad
	if f.plotRight < f.plotLeft {
		f.plotRight = f.plotLeft
	}
	c.plotLeft, c.plotRight, c.plotBottom = f.plotLeft, f.plotRight, 0
	if len(f.rows) > 0 {
		c.plotBottom = f.rows[len(f.rows)-1].bottom
	}

	used := 0
	for i, l := range c.lanes {
		row, a := f.rows[i], f.axes[i]
		r.text(&used, paramTitle(l.Name, l.Unit), m.titleSize, fg, f.plotLeft, row.top+m.pad/2)
		if !a.ok {
			t := r.text(&used, "No data", m.titleSize, muted, 0, 0)
			ts := t.MinSize()
			t.Move(fyne.NewPos(
				f.plotLeft+(f.plotRight-f.plotLeft-ts.Width)/2,
				(row.plotTop+row.plotBottom-ts.Height)/2))
			continue
		}
		for j, tick := range a.ticks {
			w := fyne.MeasureText(a.labels[j], m.tickSize, fyne.TextStyle{}).Width
			r.text(&used, a.labels[j], m.tickSize, muted,
				f.plotLeft-m.pad-w, a.y(tick, row)-m.tickHeight/2)
		}
	}

	// Time axis
	f.timeTicks, f.timeStep = nil, 0
	r.axisTexts = r.axisTexts[:0]
	if len(f.rows) > 0 && c.to.After(c.from) {
		label, origin := c.labeler()
		// Room for the widest label, which is about as wide as one with
		// all zeros, plus a gap
		widest := fyne.MeasureText("0:00:00.0", m.tickSize, fyne.TextStyle{}).Width
		maxTicks := int((f.plotRight - f.plotLeft) / (widest * 1.5))
		f.timeTicks, f.timeStep = timeTicks(origin, c.from, c.to, maxTicks)
		span := float32(c.to.Sub(c.from))
		y := f.rows[len(f.rows)-1].bottom + m.pad
		for _, t := range f.timeTicks {
			s := label(t, f.timeStep)
			if s == "" {
				continue // the labeler leaves this tick unlabeled
			}
			w := fyne.MeasureText(s, m.tickSize, fyne.TextStyle{}).Width
			x := f.plotLeft + float32(t.Sub(c.from))/span*(f.plotRight-f.plotLeft) - w/2
			if x < 0 || x+w > size.Width {
				continue // would be cut off at the edge
			}
			r.axisTexts = append(r.axisTexts, r.text(&used, s, m.tickSize, muted, x, y))
		}
	}

	r.objects = r.objects[:1]
	for _, t := range r.texts[:used] {
		r.objects = append(r.objects, t)
	}
	r.objects = append(r.objects, r.cursor.objects(len(c.lanes))...)
	r.layoutCursor(size)
}

// image returns a cleared w x h image, reusing the last frame's if the size
// hasn't changed.
func (r *laneChartRenderer) image(w, h int) *image.RGBA {
	if r.img == nil || r.img.Rect.Dx() != w || r.img.Rect.Dy() != h {
		r.img = image.NewRGBA(image.Rect(0, 0, w, h))
		return r.img
	}
	clear(r.img.Pix)
	return r.img
}

// generate draws the grid and traces. w and h are in pixels, not Fyne units,
// so everything laid out in units is scaled by their ratio.
func (r *laneChartRenderer) generate(w, h int) image.Image {
	img := r.image(max(w, 0), max(h, 0))
	c, f := r.chart, &r.frame
	size := r.raster.Size()
	if w <= 0 || h <= 0 || size.Width <= 0 || len(c.lanes) != len(f.rows) {
		return img
	}
	scale := float32(w) / size.Width
	m := c.metrics()

	th, v := c.Theme(), fyne.CurrentApp().Settings().ThemeVariant()
	lineColor := uniform(th.Color(theme.ColorNamePrimary, v))
	gridColor := uniform(th.Color(theme.ColorNameSeparator, v))
	halfWidth := m.lineWidth * scale / 2

	px := func(u float32) int { return int(math.Round(float64(u * scale))) }
	left, right := px(f.plotLeft), px(f.plotRight)
	pw := right - left
	lineSpan := c.to.Sub(c.from)

	for i, row := range f.rows {
		if i > 0 { // separator between lanes
			hline(img, gridColor, 0, w, px(row.top))
		}
		a := f.axes[i]
		if !a.ok || pw <= 0 || lineSpan <= 0 || c.lanes[i].Downsample == nil {
			continue
		}
		plot := image.Rect(left, px(row.plotTop), right, px(row.plotBottom))
		if plot.Dy() <= 0 {
			continue
		}
		for _, tick := range a.ticks {
			hline(img, gridColor, left, right, px(a.y(tick, row)))
		}
		for _, t := range f.timeTicks {
			x := left + int(float64(t.Sub(c.from))/float64(lineSpan)*float64(pw))
			draw.Draw(img, image.Rect(x, plot.Min.Y, x+1, plot.Max.Y), gridColor, image.Point{}, draw.Over)
		}

		d := c.lanes[i].Downsample(c.from, c.to, pw)
		r.trace(d, a, plot, halfWidth, lineSpan)
		r.z.Draw(img, plot, lineColor, image.Point{})
	}
	return img
}

// uniform returns c as a solid image to draw with.
func uniform(c color.Color) *image.Uniform { return image.NewUniform(c) }

// hline draws a 1px horizontal line from x0 to x1 on row y.
func hline(img *image.RGBA, c *image.Uniform, x0, x1, y int) {
	draw.Draw(img, image.Rect(x0, y, x1, y+1), c, image.Point{}, draw.Over)
}

// trace adds a lane's traces to r.z, whose coordinates are relative to plot's
// top left. Each bucket is a vertical span from its min to its max, and
// consecutive buckets are joined from the last value of one to the first of
// the next, unless the data has a gap between them.
func (r *laneChartRenderer) trace(d logging.Downsampled, a laneAxis, plot image.Rectangle, hw float32, span time.Duration) {
	z := r.z
	pw, ph := plot.Dx(), plot.Dy()
	z.Reset(pw, ph)
	// y within the plot, in pixels
	y := func(v float64) float32 {
		return float32(ph) - float32((v-a.lo)/(a.hi-a.lo))*float32(ph)
	}

	// How far apart samples are, to tell a gap in the data from sparse
	// samples. With a sample every pixelsPerSample pixels, a few missing in
	// a row is a gap; with several samples a pixel, any empty pixel is.
	samples := 0
	for _, b := range d.Buckets {
		samples += b.N
	}
	maxJoin := float32(3)
	if samples > 0 {
		maxJoin = max(maxJoin, 3*float32(pw)/float32(samples))
	}

	// prev is where the trace left off: the previous bucket's last value
	prevX, prevY, havePrev := float32(0), float32(0), false
	if d.Before != nil {
		prevX = timeX(d.Before.T, span, pw, r.chart.from)
		prevY, havePrev = y(d.Before.V), true
	}
	first := true
	for i, b := range d.Buckets {
		if b.N == 0 {
			continue
		}
		x := float32(i) + 0.5
		if havePrev && x-prevX <= maxJoin {
			segment(z, prevX, prevY, x, y(b.First), hw)
		}
		first = false
		segment(z, x, y(b.Max), x, y(b.Min), hw)
		prevX, prevY, havePrev = x, y(b.Last), true
	}
	if d.After != nil && havePrev {
		x := timeX(d.After.T, span, pw, r.chart.from)
		// With nothing visible, the samples either side are the only
		// evidence of how far apart they are, so join them
		if first || x-prevX <= maxJoin {
			segment(z, prevX, prevY, x, y(d.After.V), hw)
		}
	}
}

// timeX returns the pixel column t falls on, which is off the plot for a
// time outside the range.
func timeX(t time.Time, span time.Duration, pw int, from time.Time) float32 {
	return float32(float64(t.Sub(from)) / float64(span) * float64(pw))
}

// segment adds a line from (x0, y0) to (x1, y1) as a quad hw either side of
// it, extended by hw at each end so segments that meet leave no notch.
// Every quad winds the same way, so overlapping ones add up rather than
// cancel out.
func segment(z *vector.Rasterizer, x0, y0, x1, y1, hw float32) {
	dx, dy := x1-x0, y1-y0
	l := float32(math.Hypot(float64(dx), float64(dy)))
	if l < 1e-3 { // a lone sample: a dot
		dx, dy, l = 1, 0, 1
		x0, x1 = x0-hw, x0+hw
		y1 = y0
	} else {
		dx, dy = dx/l, dy/l
		x0, y0 = x0-dx*hw, y0-dy*hw
		x1, y1 = x1+dx*hw, y1+dy*hw
	}
	nx, ny := -dy*hw, dx*hw
	z.MoveTo(x0+nx, y0+ny)
	z.LineTo(x1+nx, y1+ny)
	z.LineTo(x1-nx, y1-ny)
	z.LineTo(x0-nx, y0-ny)
	z.ClosePath()
}
