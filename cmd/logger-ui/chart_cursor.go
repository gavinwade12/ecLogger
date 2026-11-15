package main

import (
	"image/color"
	"math"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
)

// The cursor reads out every lane's value at one moment.
//
// With a mouse it follows the pointer while it's over the plot, and is gone
// when the pointer leaves. Clicking, or tapping on a touch screen, pins it at
// the time clicked, where it stays however the pointer moves, until the plot
// is clicked on the cursor itself, the view's span changes (zooming or
// fitting), or the view moves on until the cursor's time is off the chart.
// Once it's unpinned, it follows the pointer again.
//
// A hovering cursor stays at the pointer's x as a live chart scrolls, since
// that is where the pointer is. A pinned one stays at its time, so it drifts
// left with the data and leaves at the edge.
const (
	// cursorToleranceWidth is how far from the cursor, in Fyne units, a
	// sample can be and still be read out
	cursorToleranceWidth = 10
	// cursorTapSlop is how close to the cursor, in Fyne units, a tap has
	// to be to remove it rather than move it
	cursorTapSlop = 14
)

// cursorState is the cursor's place on the chart.
type cursorState struct {
	hovering bool
	hoverX   float32 // within the chart, in Fyne units

	tapped bool
	tapT   time.Time
}

// timeAtX returns the time at x on a plot running from left to right that
// shows [from, to].
func timeAtX(x, left, right float32, from, to time.Time) time.Time {
	if right <= left {
		return from
	}
	f := float64(x-left) / float64(right-left)
	return from.Add(time.Duration(f * float64(to.Sub(from))))
}

// xAtTime is the inverse of timeAtX.
func xAtTime(t time.Time, left, right float32, from, to time.Time) float32 {
	span := to.Sub(from)
	if span <= 0 {
		return left
	}
	return left + float32(float64(t.Sub(from))/float64(span))*(right-left)
}

// inPlot reports whether pos is over the plot: between the lanes' gutter
// and right edge, and above the time axis.
func (c *laneChart) inPlot(pos fyne.Position) bool {
	return len(c.lanes) > 0 && c.plotRight > c.plotLeft && c.to.After(c.from) &&
		pos.X >= c.plotLeft && pos.X <= c.plotRight && pos.Y >= 0 && pos.Y <= c.plotBottom
}

// MouseIn, MouseMoved and MouseOut make a chart a desktop.Hoverable.
func (c *laneChart) MouseIn(ev *desktop.MouseEvent)    { c.hover(ev.Position) }
func (c *laneChart) MouseMoved(ev *desktop.MouseEvent) { c.hover(ev.Position) }
func (c *laneChart) MouseOut() {
	c.hovering = false
	c.cursorChanged()
}

func (c *laneChart) hover(pos fyne.Position) {
	c.hovering = c.inPlot(pos)
	c.hoverX = pos.X
	c.cursorChanged()
}

// Tapped makes a chart a fyne.Tappable: it moves the cursor to the time
// tapped, or removes it if that's where it already is.
func (c *laneChart) Tapped(ev *fyne.PointEvent) {
	if !c.inPlot(ev.Position) {
		return
	}
	if c.tapped {
		x := xAtTime(c.tapT, c.plotLeft, c.plotRight, c.from, c.to)
		if math.Abs(float64(x-ev.Position.X)) <= cursorTapSlop {
			c.tapped = false
			c.cursorChanged()
			return
		}
	}
	c.tapped = true
	c.tapT = timeAtX(ev.Position.X, c.plotLeft, c.plotRight, c.from, c.to)
	c.cursorChanged()
}

// cursorAt returns where the cursor is, if it's showing: its time and its x
// within the chart.
func (c *laneChart) cursorAt() (t time.Time, x float32, ok bool) {
	if c.plotRight <= c.plotLeft || !c.to.After(c.from) {
		return t, 0, false
	}
	if c.tapped {
		if !c.tapT.Before(c.from) && !c.tapT.After(c.to) {
			return c.tapT, xAtTime(c.tapT, c.plotLeft, c.plotRight, c.from, c.to), true
		}
		c.tapped = false // the view has moved on
	}
	if c.hovering {
		x = min(max(c.hoverX, c.plotLeft), c.plotRight)
		return timeAtX(x, c.plotLeft, c.plotRight, c.from, c.to), x, true
	}
	return t, 0, false
}

// cursorChanged redraws the cursor alone, leaving the raster as it is.
func (c *laneChart) cursorChanged() {
	if r := c.rend; r != nil {
		r.layoutCursor(c.Size())
		r.cursor.refresh()
	}
}

// formatReading formats v for a lane's readout: a decimal more than the
// axis's labels have, so it says more than they do.
func formatReading(v float64, a laneAxis) string {
	if v == 0 {
		v = 0 // not "-0"
	}
	return strconv.FormatFloat(v, 'f', min(a.decimals+1, 3), 64)
}

// cursorOverlay is the objects that draw the cursor: a line down the plot,
// a time label under it and a value beside it in each lane. They're always
// in the chart's objects and hidden when not wanted, so the cursor moves
// without the chart changing its objects.
type cursorOverlay struct {
	line, timeBG   *canvas.Rectangle
	timeText       *canvas.Text
	readBG         []*canvas.Rectangle
	readText       []*canvas.Text
	all, withLanes []fyne.CanvasObject
}

func newCursorOverlay() *cursorOverlay {
	o := &cursorOverlay{
		line:     canvas.NewRectangle(color.Transparent),
		timeBG:   canvas.NewRectangle(color.Transparent),
		timeText: canvas.NewText("", nil),
	}
	o.line.Hide()
	o.timeBG.Hide()
	o.timeText.Hide()
	return o
}

// objects returns the overlay's objects for a chart of lanes lanes.
func (o *cursorOverlay) objects(lanes int) []fyne.CanvasObject {
	for len(o.readText) < lanes {
		bg, t := canvas.NewRectangle(color.Transparent), canvas.NewText("", nil)
		bg.Hide()
		t.Hide()
		o.readBG, o.readText = append(o.readBG, bg), append(o.readText, t)
	}
	o.all = append(o.all[:0], o.line, o.timeBG, o.timeText)
	for i := range lanes {
		o.all = append(o.all, o.readBG[i], o.readText[i])
	}
	return o.all
}

func (o *cursorOverlay) hideAll() {
	for _, x := range o.all {
		x.Hide()
	}
}

func (o *cursorOverlay) refresh() {
	for _, x := range o.all {
		x.Refresh()
	}
}

// placeLabel puts text s in a box with its top left at x, y.
func placeLabel(bg *canvas.Rectangle, t *canvas.Text, s string, size float32, fg, fill color.Color, x, y, pad float32) float32 {
	t.Text, t.TextSize, t.Color = s, size, fg
	ts := t.MinSize()
	t.Resize(ts)
	t.Move(fyne.NewPos(x+pad, y))
	bg.FillColor = fill
	bg.Resize(fyne.NewSize(ts.Width+pad*2, ts.Height))
	bg.Move(fyne.NewPos(x, y))
	bg.Show()
	t.Show()
	return ts.Width + pad*2
}

// layoutCursor places the cursor for the current frame, or hides it. Beside
// the line, each lane's reading sits to its right unless there isn't room
// before the chart's edge, when it flips to the left.
func (r *laneChartRenderer) layoutCursor(size fyne.Size) {
	c, f, o := r.chart, &r.frame, r.cursor
	o.hideAll()
	for _, t := range r.axisTexts {
		t.Show()
	}
	if len(f.rows) != len(c.lanes) || len(f.rows) == 0 {
		return
	}
	t, x, ok := c.cursorAt()
	if !ok {
		return
	}
	m := c.metrics()
	th, v := c.Theme(), fyne.CurrentApp().Settings().ThemeVariant()
	fg := th.Color(theme.ColorNameForeground, v)
	br, bg, bb, _ := th.Color(theme.ColorNameBackground, v).RGBA()
	fill := color.NRGBA{uint8(br >> 8), uint8(bg >> 8), uint8(bb >> 8), 0xdd}

	bottom := f.rows[len(f.rows)-1].bottom
	o.line.FillColor = th.Color(theme.ColorNameForeground, v)
	o.line.Move(fyne.NewPos(x-0.5, 0))
	o.line.Resize(fyne.NewSize(1, bottom))
	o.line.Show()

	// each lane's value, nearest within a few pixels' worth of time
	tol := time.Duration(float64(c.to.Sub(c.from)) * cursorToleranceWidth / float64(f.plotRight-f.plotLeft))
	for i, l := range c.lanes {
		a := f.axes[i]
		if l.ValueAt == nil || !a.ok {
			continue
		}
		val, ok := l.ValueAt(t, tol)
		if !ok {
			continue
		}
		s := formatReading(val, a)
		if l.Unit != "" {
			s += " " + string(l.Unit)
		}
		w := fyne.MeasureText(s, m.titleSize, fyne.TextStyle{}).Width + m.pad*2
		bx := x + m.pad
		if bx+w > size.Width {
			bx = x - m.pad - w
		}
		placeLabel(o.readBG[i], o.readText[i], s, m.titleSize, fg, fill, bx, f.rows[i].plotTop, m.pad)
	}

	// the time, over the axis' labels
	text, _ := c.labeler()
	step := f.timeStep / 10
	if f.timeStep == 0 {
		step = c.to.Sub(c.from) / 100
	}
	s := text(t, max(step, time.Millisecond))
	if s == "" {
		return // a time the labeler leaves unlabeled, like the future on a live chart
	}
	w := fyne.MeasureText(s, m.tickSize, fyne.TextStyle{}).Width + m.pad*2
	tx := min(max(x-w/2, 0), size.Width-w)
	placeLabel(o.timeBG, o.timeText, s, m.tickSize, fg, th.Color(theme.ColorNameBackground, v), tx, bottom+m.pad, m.pad)
	// Axis labels the cursor's would cover part of are hidden whole,
	// rather than left cut off
	for _, t := range r.axisTexts {
		if l, e := t.Position().X, t.Position().X+t.Size().Width; e > tx-m.pad && l < tx+w+m.pad {
			t.Hide()
		}
	}
}
