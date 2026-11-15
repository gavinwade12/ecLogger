package main

import (
	"math"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

// Navigating the chart changes the span of time it shows, and only that: the
// lanes' y-axes follow the values in view.
//
//   - Dragging the plot pans, with the mouse or a finger. The cursor follows
//     the pointer as it drags, so it stays on the moment grabbed.
//   - Scrolling sideways, as with a trackpad, pans.
//   - Scrolling vertically while Ctrl or Cmd is held zooms about the pointer.
//   - Scrolling vertically otherwise isn't the chart's, and goes to the
//     scroll container the chart is in, so the lanes still scroll.
//   - ZoomBy, zoomAbout and Fit are for the buttons.
//
// The view can't be zoomed past minViewSpan or out beyond bounds, and panning
// stops at bounds.
const (
	// minViewSpan is the least time the chart zooms in to, unless the
	// bounds are shorter than that
	minViewSpan = time.Second
	// scrollZoomRate is the zoom per unit of vertical scrolling: a wheel
	// notch, about 10 units, zooms by about 10%
	scrollZoomRate = 0.01
)

// chartNav is what a chart needs to be navigated.
type chartNav struct {
	// bounds is the span of time the view stays within: the whole log, or
	// the live window. Nothing can be navigated without it.
	bounds func() (from, to time.Time)
	// beforeNav is called before the view changes by the user, and may
	// change the view itself. A live chart stops following the present
	// there.
	beforeNav func()
	// scroller is given the scrolling the chart doesn't use
	scroller fyne.Scrollable
}

// SetBounds sets the span of time the chart can be zoomed out to and panned
// within. It's asked each time, as a live chart's moves.
func (c *laneChart) SetBounds(f func() (from, to time.Time)) { c.bounds = f }

// OnNavigate sets a function called before the user changes the view.
func (c *laneChart) OnNavigate(f func()) { c.beforeNav = f }

// SetScroller sets where the scrolling that isn't the chart's goes.
func (c *laneChart) SetScroller(s fyne.Scrollable) { c.scroller = s }

func (c *laneChart) keyModifiers() fyne.KeyModifier {
	if d, ok := fyne.CurrentApp().Driver().(desktop.Driver); ok {
		return d.CurrentKeyModifiers()
	}
	return 0
}

// shiftInto moves [from, from+span] to lie within [lo, hi], as near its
// place as it can, or onto lo if it's longer than that.
func shiftInto(from time.Time, span time.Duration, lo, hi time.Time) (time.Time, time.Time) {
	if span >= hi.Sub(lo) {
		return lo, hi
	}
	if from.Before(lo) {
		from = lo
	}
	if from.Add(span).After(hi) {
		from = hi.Add(-span)
	}
	return from, from.Add(span)
}

// zoomView scales the span of [from, to] by factor, below 1 to zoom in,
// keeping the time anchor at the same place in the view, within [lo, hi] and
// no shorter than minSpan. A view pushed against an end by the limits moves
// along it rather than keeping anchor still.
func zoomView(from, to, anchor time.Time, factor float64, lo, hi time.Time, minSpan time.Duration) (time.Time, time.Time) {
	span, full := to.Sub(from), hi.Sub(lo)
	if span <= 0 || factor <= 0 || full <= 0 {
		return from, to
	}
	minSpan = min(minSpan, full)
	next := time.Duration(math.Min(math.Max(float64(span)*factor, float64(minSpan)), float64(full)))
	frac := float64(anchor.Sub(from)) / float64(span)
	return shiftInto(anchor.Add(-time.Duration(frac*float64(next))), next, lo, hi)
}

// panView moves [from, to] by delta, stopping at lo and hi.
func panView(from, to time.Time, delta time.Duration, lo, hi time.Time) (time.Time, time.Time) {
	return shiftInto(from.Add(delta), to.Sub(from), lo, hi)
}

// navigate changes the view to what next makes of it and the bounds, and
// redraws. Nothing happens, and beforeNav isn't called, if the view wouldn't
// change.
func (c *laneChart) navigate(next func(from, to, lo, hi time.Time) (time.Time, time.Time)) {
	if c.bounds == nil {
		return
	}
	apply := func() (changed bool) {
		lo, hi := c.bounds()
		if !hi.After(lo) {
			return false
		}
		from, to := next(c.from, c.to, lo, hi)
		if from.Equal(c.from) && to.Equal(c.to) {
			return false
		}
		if to.Sub(from) != c.to.Sub(c.from) {
			c.tapped = false // zooming moves what was tapped
		}
		c.from, c.to = from, to
		return true
	}
	// Try it on a copy of the view first, so a no-op doesn't make
	// beforeNav stop a live chart following
	from, to := c.from, c.to
	changed := apply()
	c.from, c.to = from, to
	if !changed {
		return
	}
	if c.beforeNav != nil {
		c.beforeNav()
	}
	if apply() {
		c.readRanges()
		c.Refresh()
	}
}

// ZoomBy zooms by factor about the middle of the view; below 1 zooms in.
func (c *laneChart) ZoomBy(factor float64) {
	c.zoomAbout(c.from.Add(c.to.Sub(c.from)/2), factor)
}

// zoomAbout zooms by factor keeping anchor where it is in the view, or about
// the middle if anchor isn't in view.
func (c *laneChart) zoomAbout(anchor time.Time, factor float64) {
	c.navigate(func(from, to, lo, hi time.Time) (time.Time, time.Time) {
		if anchor.Before(from) || anchor.After(to) {
			anchor = from.Add(to.Sub(from) / 2)
		}
		return zoomView(from, to, anchor, factor, lo, hi, minViewSpan)
	})
}

// Fit zooms out to the whole of the bounds.
func (c *laneChart) Fit() {
	c.navigate(func(_, _, lo, hi time.Time) (time.Time, time.Time) { return lo, hi })
}

// zoomAt zooms by factor about the pointer at x.
func (c *laneChart) zoomAt(x float32, factor float64) {
	x = min(max(x, c.plotLeft), c.plotRight)
	c.navigate(func(from, to, lo, hi time.Time) (time.Time, time.Time) {
		anchor := timeAtX(x, c.plotLeft, c.plotRight, from, to)
		return zoomView(from, to, anchor, factor, lo, hi, minViewSpan)
	})
}

// panBy pans the view by dx Fyne units of the plot, so what's under the
// pointer follows it: a positive dx shows earlier times.
func (c *laneChart) panBy(dx float32) {
	w := c.plotRight - c.plotLeft
	if w <= 0 {
		return
	}
	c.navigate(func(from, to, lo, hi time.Time) (time.Time, time.Time) {
		delta := -time.Duration(float64(dx) / float64(w) * float64(to.Sub(from)))
		return panView(from, to, delta, lo, hi)
	})
}

// Dragged and DragEnd make a chart a fyne.Draggable, which pans it. Fyne
// sends no hover events while a drag goes on, so the drag moves the cursor
// with the pointer itself; as the pan keeps the data under the pointer, the
// cursor stays on the same moment.
func (c *laneChart) Dragged(ev *fyne.DragEvent) {
	c.hovering, c.hoverX = c.inPlot(ev.Position), ev.Position.X
	c.panBy(ev.Dragged.DX)
	c.cursorChanged()
}

func (c *laneChart) DragEnd() {}

// Scrolled makes a chart a fyne.Scrollable. See the notes at the top for
// what it does with which scrolling.
func (c *laneChart) Scrolled(ev *fyne.ScrollEvent) {
	dx, dy := ev.Scrolled.DX, ev.Scrolled.DY
	switch {
	case dy != 0 && c.keyModifiers()&(fyne.KeyModifierControl|fyne.KeyModifierSuper) != 0:
		c.zoomAt(ev.Position.X, math.Exp(-float64(dy)*scrollZoomRate))
	case dx != 0 && math.Abs(float64(dx)) > math.Abs(float64(dy)):
		c.panBy(dx)
	case c.scroller != nil:
		c.scroller.Scrolled(ev)
	}
}
