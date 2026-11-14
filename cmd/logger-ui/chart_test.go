package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"github.com/gavinwade12/ecLogger/internal/logging"
	"github.com/gavinwade12/ecLogger/units"
)

func TestFormatElapsed(t *testing.T) {
	for _, tt := range []struct {
		d, step time.Duration
		want    string
	}{
		{0, time.Second, "0:00"},
		{90 * time.Second, 30 * time.Second, "1:30"},
		{59*time.Minute + 59*time.Second, time.Second, "59:59"},
		{2*time.Hour + 0*time.Minute, time.Minute, "2:00:00"},
		{time.Hour + 2*time.Minute, time.Minute, "1:02:00"},
		{-30 * time.Second, time.Second, "-0:30"},
		{1500 * time.Millisecond, 500 * time.Millisecond, "0:01.5"},
		{61250 * time.Millisecond, 50 * time.Millisecond, "1:01.25"},
		{time.Minute - time.Millisecond, time.Second, "1:00"}, // rounds up, carrying
	} {
		if got := formatElapsed(tt.d, tt.step); got != tt.want {
			t.Errorf("formatElapsed(%v, %v) = %q, want %q", tt.d, tt.step, got, tt.want)
		}
	}
}

func TestTimeTicks(t *testing.T) {
	origin := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	ticks, step := timeTicks(origin, origin, origin.Add(2*time.Minute), 6)
	if step != 30*time.Second || len(ticks) != 5 {
		t.Fatalf("got %d ticks every %v, want 5 every 30s", len(ticks), step)
	}
	// Panning keeps ticks on whole steps from the origin
	ticks, step = timeTicks(origin, origin.Add(7*time.Second), origin.Add(37*time.Second), 7)
	if step != 5*time.Second || ticks[0] != origin.Add(10*time.Second) {
		t.Errorf("first tick %v step %v, want origin+10s every 5s", ticks[0].Sub(origin), step)
	}
	if ticks, _ := timeTicks(origin, origin, origin, 5); ticks != nil {
		t.Errorf("empty range has ticks: %v", ticks)
	}
}

func TestLanesFromLog(t *testing.T) {
	start := time.Now()
	l := &logging.Log{
		Times: []time.Time{start, start.Add(time.Second)},
		Columns: []logging.Column{
			{Name: "a", Unit: units.RPM, Values: []float64{1, 3}},
			{Name: "b", Values: []float64{math.NaN(), 5}},
		},
	}
	lanes := lanesFromLog(l, []int{1, 7, 0})
	if len(lanes) != 2 || lanes[0].Name != "b" || lanes[1].Unit != units.RPM {
		t.Fatalf("lanes = %+v", lanes)
	}
	if min, max, ok := lanes[1].Range(); !ok || min != 1 || max != 3 {
		t.Errorf("range = %v, %v, %v", min, max, ok)
	}
	d := lanes[0].Downsample(start, start.Add(2*time.Second), 2)
	if d.Buckets[0].N != 0 || d.Buckets[1].Max != 5 {
		t.Errorf("buckets = %+v", d.Buckets)
	}
}

// newTestChart returns a chart of the given size in the app's theme, with its
// renderer.
func newTestChart(t *testing.T, w, h float32, lanes []chartLane, from, to time.Time) (*laneChart, *laneChartRenderer) {
	t.Helper()
	app := test.NewTempApp(t)
	app.Settings().SetTheme(newAppTheme())
	c := newLaneChart()
	r := test.TempWidgetRenderer(t, c).(*laneChartRenderer)
	c.SetTimeRange(from, to)
	c.SetLanes(lanes)
	c.Resize(fyne.NewSize(w, h))
	return c, r
}

// seriesLane makes a lane from values sampled every dt from start.
func seriesLane(name string, start time.Time, dt time.Duration, values []float64) chartLane {
	times := make([]time.Time, len(values))
	for i := range times {
		times[i] = start.Add(time.Duration(i) * dt)
	}
	return lanesFromLog(&logging.Log{
		Times:   times,
		Columns: []logging.Column{{Name: name, Values: values}},
	}, []int{0})[0]
}

// linePixels reports whether the pixel at x, y is the trace's color, which
// tells it from a grid line or the empty background.
func isTrace(img image.Image, x, y int) bool {
	want := theme.DefaultTheme().Color(theme.ColorNamePrimary, theme.VariantDark)
	if fyne.CurrentApp().Settings().ThemeVariant() == theme.VariantLight {
		want = theme.DefaultTheme().Color(theme.ColorNamePrimary, theme.VariantLight)
	}
	r, g, b, a := img.At(x, y).RGBA()
	if a < 0x8000 {
		return false
	}
	wr, wg, wb, _ := want.RGBA()
	near := func(got, want uint32) bool {
		// got is premultiplied; compare against the color at full alpha
		got = got * 0xffff / a
		d := int(got) - int(want)
		return d > -0x1800 && d < 0x1800
	}
	return near(r, wr) && near(g, wg) && near(b, wb)
}

func TestChartGeometry(t *testing.T) {
	start := time.Now()
	wide := seriesLane("wide", start, time.Second, []float64{0, 123456})
	narrow := seriesLane("narrow", start, time.Second, []float64{0, 1})
	c, r := newTestChart(t, 600, 400, []chartLane{wide, narrow}, start, start.Add(time.Second))
	m := c.metrics()
	f := r.frame

	if len(f.rows) != 2 || len(f.axes) != 2 {
		t.Fatalf("got %d rows and %d axes, want 2", len(f.rows), len(f.axes))
	}
	if f.rows[0].bottom != f.rows[1].top {
		t.Errorf("lanes don't touch: %v then %v", f.rows[0], f.rows[1])
	}
	if last := f.rows[1].bottom; math.Abs(float64(400-m.timeAxisHeight-last)) > 0.01 {
		t.Errorf("lanes end at %v, want room for the %v time axis", last, m.timeAxisHeight)
	}
	for i, row := range f.rows {
		if row.plotTop < row.top+m.titleHeight || row.plotBottom > row.bottom {
			t.Errorf("lane %d plot %v..%v outside lane %v..%v below title", i, row.plotTop, row.plotBottom, row.top, row.bottom)
		}
	}
	// The narrow lane's plot starts where the wide lane's does, with the
	// gutter wide enough for the wide lane's labels.
	if f.axes[0].labelsWidth <= f.axes[1].labelsWidth {
		t.Fatalf("test needs lane 0's labels wider: %v vs %v", f.axes[0].labelsWidth, f.axes[1].labelsWidth)
	}
	if f.plotLeft < f.axes[0].labelsWidth+m.pad {
		t.Errorf("gutter %v too narrow for labels %v wide", f.plotLeft, f.axes[0].labelsWidth)
	}
	if f.plotRight != 600-m.pad {
		t.Errorf("plot ends at %v, want %v", f.plotRight, 600-m.pad)
	}
}

func TestChartMinSize(t *testing.T) {
	start := time.Now()
	c, _ := newTestChart(t, 600, 400, nil, start, start.Add(time.Second))
	m := c.metrics()
	empty := c.MinSize()
	c.SetLanes([]chartLane{{}, {}, {}})
	if got, want := c.MinSize().Height, empty.Height+3*m.laneMinHeight; got != want {
		t.Errorf("3 lanes MinSize height = %v, want %v", got, want)
	}
	if 4*m.laneMinHeight+m.timeAxisHeight > 380 {
		t.Errorf("4 lanes need %v, more than fits a 480px screen beside the tab bar", 4*m.laneMinHeight+m.timeAxisHeight)
	}
}

// plotColumn returns the pixel x of the plot's column i, and the rows of
// lane's plot, in a frame drawn at scale 1.
func plotColumn(r *laneChartRenderer, lane, i int) (x, top, bottom int) {
	f := r.frame
	return int(math.Round(float64(f.plotLeft))) + i,
		int(math.Round(float64(f.rows[lane].plotTop))), int(math.Round(float64(f.rows[lane].plotBottom)))
}

func TestChartDrawsConstantAsHorizontalLine(t *testing.T) {
	start := time.Now()
	values := make([]float64, 200)
	for i := range values {
		values[i] = 5
	}
	_, r := newTestChart(t, 400, 200, []chartLane{seriesLane("c", start, 10*time.Millisecond, values)}, start, start.Add(2*time.Second))
	img := r.generate(400, 200)

	a := r.frame.axes[0]
	wantY := int(r.frame.axes[0].y(5, r.frame.rows[0]))
	for _, i := range []int{10, 100, 200} {
		x, _, _ := plotColumn(r, 0, i)
		if !isTrace(img, x, wantY) {
			t.Errorf("column %d: no trace at row %d (axis %v..%v)", i, wantY, a.lo, a.hi)
		}
		if isTrace(img, x, wantY-6) || isTrace(img, x, wantY+6) {
			t.Errorf("column %d: trace is thicker than a line", i)
		}
	}
}

func TestChartShowsSpikeAfterDownsampling(t *testing.T) {
	start := time.Now()
	values := make([]float64, 10000)
	values[5000] = 10
	c, r := newTestChart(t, 300, 200, []chartLane{seriesLane("s", start, time.Millisecond, values)}, start, start.Add(10*time.Second))
	// size the chart so the plot is 100px wide
	c.Resize(fyne.NewSize(100+r.frame.plotLeft+c.metrics().pad, 200))
	img := r.generate(int(c.Size().Width+0.5), 200)

	if pw := r.frame.plotRight - r.frame.plotLeft; math.Abs(float64(pw-100)) > 1 {
		t.Fatalf("plot is %v wide, want 100", pw)
	}
	_, top, bottom := plotColumn(r, 0, 0)
	// The joins up to and down from the spike make it a few pixels wide
	// at the top at most, around column 50.
	for i := 0; i < 100; i++ {
		x, _, _ := plotColumn(r, 0, i)
		got, want := isTrace(img, x, top+2), i >= 49 && i <= 51
		if got && !want || i == 50 && !got {
			t.Errorf("column %d: trace at the top = %v, want %v", i, got, want)
		}
	}
	x, _, _ := plotColumn(r, 0, 20)
	if !isTrace(img, x, bottom-1) {
		t.Errorf("baseline missing near the plot bottom (rows %d..%d)", top, bottom)
	}
}

func TestChartLeavesGapForNaNRun(t *testing.T) {
	start := time.Now()
	values := make([]float64, 1000)
	for i := range values {
		values[i] = 5
		if i >= 300 && i < 600 {
			values[i] = math.NaN()
		}
	}
	_, r := newTestChart(t, 400, 200, []chartLane{seriesLane("g", start, 10*time.Millisecond, values)}, start, start.Add(10*time.Second))
	img := r.generate(400, 200)

	y := int(r.frame.axes[0].y(5, r.frame.rows[0]))
	pw := int(r.frame.plotRight - r.frame.plotLeft)
	for _, frac := range []float64{0.1, 0.8} {
		x, _, _ := plotColumn(r, 0, int(frac*float64(pw)))
		if !isTrace(img, x, y) {
			t.Errorf("no trace at %v of the way across", frac)
		}
	}
	for _, frac := range []float64{0.35, 0.45, 0.55} {
		x, top, bottom := plotColumn(r, 0, int(frac*float64(pw)))
		for yy := top; yy < bottom; yy++ {
			if isTrace(img, x, yy) {
				t.Fatalf("trace at %v of the way across, inside the gap (row %d)", frac, yy)
			}
		}
	}
}

func TestChartHandlesEmptyLanesAndNoLanes(t *testing.T) {
	start := time.Now()
	empty := seriesLane("e", start, time.Second, []float64{math.NaN(), math.NaN()})
	noFuncs := chartLane{Name: "bare"}
	for name, lanes := range map[string][]chartLane{
		"none":  nil,
		"empty": {empty},
		"bare":  {noFuncs},
		"mixed": {empty, seriesLane("v", start, time.Second, []float64{1, 2, 3}), noFuncs},
	} {
		t.Run(name, func(t *testing.T) {
			_, r := newTestChart(t, 300, 200, lanes, start, start.Add(3*time.Second))
			r.generate(300, 200)
		})
	}
	// before any size or range is set
	c := newLaneChart()
	test.TempWidgetRenderer(t, c).(*laneChartRenderer).generate(0, 0)
	_, r := newTestChart(t, 0, 0, []chartLane{empty}, start, start)
	r.generate(0, 0)
}

func TestChartScalesToPixels(t *testing.T) {
	start := time.Now()
	values := []float64{5, 5, 5, 5}
	_, r := newTestChart(t, 300, 150, []chartLane{seriesLane("c", start, time.Second, values)}, start, start.Add(4*time.Second))
	// a 2x display draws twice as many pixels for the same layout
	img := r.generate(600, 300)
	y := int(r.frame.axes[0].y(5, r.frame.rows[0]) * 2)
	x := int(r.frame.plotLeft*2) + 100
	if !isTrace(img, x, y) {
		t.Errorf("no trace at 2x row %d", y)
	}
}

// TestRenderChartSamples writes PNGs of the chart with sample data in each
// theme variant to the directory in CHART_PNG_DIR, for looking at. It does
// nothing without it.
func TestRenderChartSamples(t *testing.T) {
	dir := os.Getenv("CHART_PNG_DIR")
	if dir == "" {
		t.Skip("set CHART_PNG_DIR to write sample renders")
	}
	start := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	const n, dt = 6000, 20 * time.Millisecond // two minutes
	rpm := make([]float64, n)
	noisy := make([]float64, n)
	gappy := make([]float64, n)
	for i := range rpm {
		x := float64(i) / n
		rpm[i] = 2500 + 2000*math.Sin(x*12) + 500*math.Sin(x*90)
		noisy[i] = 14.7 + 0.3*math.Sin(float64(i)*0.7)
		if i%997 == 0 {
			noisy[i] += 6
		}
		gappy[i] = 90 + 10*math.Sin(x*5)
		if i > 2400 && i < 3300 {
			gappy[i] = math.NaN()
		}
	}
	lanes := []chartLane{
		seriesLane("Engine Speed (rpm)", start, dt, rpm),
		seriesLane("Air/Fuel Sensor #1", start, dt, noisy),
		seriesLane("Coolant Temperature", start, dt, gappy),
		seriesLane("Knock Correction", start, dt, nil),
	}
	lanes[0].Name, lanes[0].Unit = "Engine Speed", units.RPM
	for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		app := test.NewTempApp(t)
		app.Settings().SetTheme(forcedVariant{newAppTheme(), variant})
		c := newLaneChart()
		c.SetLanes(lanes)
		c.SetTimeRange(start, start.Add(n*dt))
		c.SetTimeAxis(start, elapsedLabels(start))
		w := test.NewWindow(c)
		w.Resize(fyne.NewSize(800, 380))
		name := "light"
		if variant == theme.VariantDark {
			name = "dark"
		}
		f, err := os.Create(filepath.Join(dir, "chart_"+name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, w.Canvas().Capture()); err != nil {
			t.Fatal(err)
		}
		f.Close()
		w.Close()
	}
}

// forcedVariant is a theme that always gives one variant's colors, so a test
// can render both without a settings change.
type forcedVariant struct {
	fyne.Theme
	variant fyne.ThemeVariant
}

func (f forcedVariant) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	return f.Theme.Color(name, f.variant)
}
