package main_test

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	ui "github.com/gavinwade12/ecLogger/cmd/logger-ui"
)

// chartWidget is what the test drives of the chart: the gestures it takes.
type chartWidget interface {
	fyne.Widget
	fyne.Tappable
	fyne.Draggable
	fyne.Scrollable
	desktop.Hoverable
}

// chart returns the Graphing tab's chart. Call it from ta.do.
func (ta *testApp) chart() chartWidget {
	ta.t.Helper()
	charts := findAll(ta.content(ui.TabGraphing), func(c chartWidget) bool { return fmt.Sprintf("%T", c) == "*main.laneChart" })
	if len(charts) != 1 {
		ta.t.Fatalf("found %d charts", len(charts))
	}
	return charts[0]
}

// readouts returns the cursor's readings and time, which are the chart's text
// that isn't its titles, ticks or axis labels, given those before the cursor
// was placed.
func readouts(now, before []string) (s []string) {
	seen := map[string]int{}
	for _, b := range before {
		seen[b]++
	}
	for _, n := range now {
		if seen[n] > 0 {
			seen[n]--
			continue
		}
		s = append(s, n)
	}
	return s
}

var elapsedLabel = regexp.MustCompile(`^(\d+):(\d\d(\.\d+)?)$`)

// elapsedSeconds returns the seconds in an elapsed time label like 1:02.5.
func elapsedSeconds(t *testing.T, label string) float64 {
	t.Helper()
	m := elapsedLabel.FindStringSubmatch(label)
	if m == nil {
		t.Fatalf("%q isn't an elapsed time", label)
	}
	mins, _ := strconv.ParseFloat(m[1], 64)
	secs, _ := strconv.ParseFloat(m[2], 64)
	return mins*60 + secs
}

// The chart's gestures, at x and y within it. Call them from ta.do.
func hoverAt(c chartWidget, x, y float32) {
	c.MouseMoved(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(x, y)}})
}

func tapAt(c chartWidget, x, y float32) {
	c.Tapped(&fyne.PointEvent{Position: fyne.NewPos(x, y)})
}

// cursorShown reports whether the chart draws its cursor line: the one
// rectangle a unit wide. Call it from ta.do.
func cursorShown(c chartWidget) bool {
	for _, o := range test.WidgetRenderer(c).Objects() {
		if r, ok := o.(*canvas.Rectangle); ok && r.Visible() && r.Size().Width == 1 {
			return true
		}
	}
	return false
}

// readingText returns the chart's shown text that reads s. Call it from
// ta.do.
func readingText(c chartWidget, s string) *canvas.Text {
	for _, o := range test.WidgetRenderer(c).Objects() {
		if t, ok := o.(*canvas.Text); ok && t.Visible() && t.Text == s {
			return t
		}
	}
	return nil
}

// openCursorLog opens a log of 41 rows, 50ms apart, in which lane j holds
// i*(j+1)+1 at row i, and returns the chart's text before any cursor.
func (ta *testApp) openCursorLog() (before []string) {
	ta.t.Helper()
	ta.resize(800, 480)
	ta.openGraph(writeLog(ta.t, "run.csv", graphColumns, 41))
	ta.do(func() { before = texts(ta.content(ui.TabGraphing)) })
	return before
}

// checkReadsOneRow checks readouts of openCursorLog's log, the time and then
// four lanes, are all the same row's, which is the row at that time, and
// returns the time.
func checkReadsOneRow(t *testing.T, got []string) (seconds float64) {
	t.Helper()
	if len(got) != 5 {
		t.Fatalf("readouts = %q, want a time and 4 values", got)
	}
	seconds = elapsedSeconds(t, got[0])
	row := -1.0
	for j, r := range got[1:] {
		v, err := strconv.ParseFloat(strings.Fields(r)[0], 64)
		if err != nil {
			t.Fatalf("readout %q: %v", r, err)
		}
		i := (v - 1) / float64(j+1)
		if i != float64(int(i)) {
			t.Errorf("lane %d reads %v, not a value of the file", j, v)
		}
		if row >= 0 && i != row {
			t.Errorf("lane %d reads row %v, others row %v", j, i, row)
		}
		row = i
	}
	if want := row * 0.05; seconds < want-0.03 || seconds > want+0.03 {
		t.Errorf("cursor at %vs reads row %v, which is at %vs", seconds, row, want)
	}
	return seconds
}

func TestGraphingTabTapReadsValuesAtThatTime(t *testing.T) {
	ta := newTestApp(t)
	before := ta.openCursorLog()

	var got []string
	ta.do(func() {
		c := ta.chart()
		tapAt(c, c.Size().Width*0.6, 10)
		got = readouts(texts(ta.content(ui.TabGraphing)), before)
	})
	checkReadsOneRow(t, got)

	// tapping on the cursor removes it
	ta.do(func() {
		c := ta.chart()
		tapAt(c, c.Size().Width*0.6, 10)
		if left := readouts(texts(ta.content(ui.TabGraphing)), before); len(left) != 0 || cursorShown(c) {
			t.Errorf("readouts remain after tapping the cursor: %q", left)
		}
	})
}

func TestGraphingTabHoverReadsEveryLane(t *testing.T) {
	ta := newTestApp(t)
	before := ta.openCursorLog()

	var got []string
	ta.do(func() {
		if cursorShown(ta.chart()) {
			t.Error("cursor shown before any hover")
		}
		c := ta.chart()
		hoverAt(c, c.Size().Width*0.5, 10)
		got = readouts(texts(ta.content(ui.TabGraphing)), before)
	})
	checkReadsOneRow(t, got)

	ta.do(func() {
		c := ta.chart()
		size := c.Size()
		// over the gutter or the time axis, the cursor goes
		hoverAt(c, 1, 10)
		if cursorShown(c) {
			t.Error("cursor over the gutter")
		}
		hoverAt(c, size.Width*0.5, size.Height-2)
		if cursorShown(c) {
			t.Error("cursor over the time axis")
		}
		hoverAt(c, size.Width*0.5, 10)
		c.MouseOut()
		if left := readouts(texts(ta.content(ui.TabGraphing)), before); len(left) != 0 || cursorShown(c) {
			t.Errorf("cursor after the mouse left: %q", left)
		}
	})
}

// A lane reads out only when a sample is within a few pixels of the cursor.
func TestGraphingTabCursorReadsOnlyNearASample(t *testing.T) {
	ta := newTestApp(t)
	ta.resize(800, 480)
	// one lane, a sample a second for 10s, holding 1 to 11
	csv := "Timestamp,Engine Speed (rpm)\n"
	start := time.Date(2026, 1, 2, 3, 4, 5, 0, time.Local)
	for i := range 11 {
		csv += fmt.Sprintf("%s,%d.0000\n", start.Add(time.Duration(i)*time.Second).Format("2006-01-02 15:04:05.999999999"), i+1)
	}
	path := filepath.Join(t.TempDir(), "sparse.csv")
	if err := os.WriteFile(path, []byte(csv), 0o644); err != nil {
		t.Fatal(err)
	}
	ta.openGraph(path)
	var before []string
	ta.do(func() { before = texts(ta.content(ui.TabGraphing)) })

	// sweep the plot, reading the time and any value at each point
	near, between := 0, 0
	ta.do(func() {
		c := ta.chart()
		for x := float32(0); x < c.Size().Width; x++ {
			hoverAt(c, x, 10)
			got := readouts(texts(ta.content(ui.TabGraphing)), before)
			if len(got) == 0 {
				continue // the gutter
			}
			secs := elapsedSeconds(t, got[0])
			whole := math.Round(secs)
			switch {
			case math.Abs(secs-whole) < 0.001: // within 0.05s of a sample
				near++
				if len(got) != 2 || got[1] != fmt.Sprintf("%v.0 rpm", whole+1) {
					t.Errorf("at %vs: %q, want the sample's value", secs, got)
				}
			case math.Abs(math.Abs(secs-whole)-0.5) < 0.001: // half way between
				between++
				if len(got) != 1 {
					t.Errorf("at %vs, between samples: %q, want only the time", secs, got)
				}
			}
		}
	})
	if near == 0 || between == 0 {
		t.Errorf("the sweep found %d points near samples and %d between them", near, between)
	}
}

// A lane's reading sits right of the cursor, unless that would run off the
// chart, when it moves to the left.
func TestGraphingTabCursorReadingStaysOnTheChart(t *testing.T) {
	ta := newTestApp(t)
	before := ta.openCursorLog()

	ta.do(func() {
		c := ta.chart()
		width := c.Size().Width
		// the first reading as the cursor sits at x, and where it is
		reading := func(x float32) *canvas.Text {
			hoverAt(c, x, 10)
			got := readouts(texts(ta.content(ui.TabGraphing)), before)
			if len(got) < 2 {
				return nil
			}
			return readingText(c, got[1])
		}

		left := float32(0)
		for ; left < width && reading(left) == nil; left++ {
		}
		if r := reading(left); r == nil || r.Position().X < left {
			t.Errorf("near the left edge, the reading isn't right of the cursor at %v", left)
		}

		right := width
		for ; right > left && reading(right) == nil; right-- {
		}
		r := reading(right)
		if r == nil {
			t.Fatal("no reading near the right edge")
		}
		if end := r.Position().X + r.Size().Width; end > right || end > width {
			t.Errorf("near the right edge, the reading ends at %v, not left of the cursor at %v", end, right)
		}
	})
}

// With a mouse, clicking pins the cursor at that time while the pointer
// moves on, and clicking on it unpins it.
func TestGraphingTabClickPinsTheCursor(t *testing.T) {
	ta := newTestApp(t)
	before := ta.openCursorLog()

	// the cursor's time, with the pointer at x
	timeAt := func(c chartWidget, x float32) string {
		hoverAt(c, x, 10)
		got := readouts(texts(ta.content(ui.TabGraphing)), before)
		if len(got) == 0 {
			t.Fatalf("no cursor with the pointer at %v", x)
		}
		return got[0]
	}
	ta.do(func() {
		c := ta.chart()
		x1, x2 := c.Size().Width*0.4, c.Size().Width*0.8
		tapAt(c, 1, 10) // the gutter isn't the plot
		c.MouseOut()
		if cursorShown(c) {
			t.Error("a click in the gutter pinned the cursor")
		}

		pinned := timeAt(c, x1)
		tapAt(c, x1, 10)
		if got := timeAt(c, x2); got != pinned {
			t.Errorf("pinned at %s, the cursor followed the pointer to %s", pinned, got)
		}
		c.MouseOut()
		if !cursorShown(c) {
			t.Error("the pinned cursor went with the mouse")
		}

		timeAt(c, x1)
		tapAt(c, x1, 10) // on the cursor
		if got := timeAt(c, x2); got == pinned {
			t.Errorf("unpinned, the cursor stayed at %s with the pointer at %v", got, x2)
		}
	})
}

// A pinned cursor stays at its time, so on a live chart it leaves with the
// data at the left edge.
func TestGraphingTabPinnedCursorLeavesWithTheLiveView(t *testing.T) {
	ta := newTestApp(t)
	start := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	clock := &testClock{t: start}
	ta.do(func() { ta.GraphingTab.SetClock(clock.now) })
	sel := ta.liveWindowSelect()
	ta.do(func() { sel.SetSelected("1 minute") })
	ta.connectLive("Engine Speed")
	clock.set(start.Add(30 * time.Second))
	ta.waitFor("data in the lane", func() bool {
		return !slices.Contains(texts(ta.content(ui.TabGraphing)), "No data")
	})

	ta.do(func() {
		c := ta.chart()
		tapAt(c, c.Size().Width*0.5, 10)
		if !cursorShown(c) {
			t.Fatal("tapping didn't pin the cursor")
		}
	})
	// a minute and a half on, its time is off the left of the view
	clock.set(start.Add(2 * time.Minute))
	ta.waitFor("the cursor to leave", func() bool { return !cursorShown(ta.chart()) })
}

// While a live chart is still filling, its right part is the future: the
// cursor there has no time to show, so it shows no time box.
func TestGraphingTabCursorOverTheLiveFuture(t *testing.T) {
	ta := newTestApp(t)
	start := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	clock := &testClock{t: start}
	ta.do(func() { ta.GraphingTab.SetClock(clock.now) })
	ta.connectLive("Engine Speed")
	ta.waitFor("data in the lane", func() bool {
		return !slices.Contains(texts(ta.content(ui.TabGraphing)), "No data")
	})
	clock.set(start.Add(2 * time.Second)) // now is a fifth of the way along the 10s view

	ta.do(func() {
		c := ta.chart()
		hoverAt(c, c.Size().Width*0.9, 10)
		if !cursorShown(c) {
			t.Fatal("no cursor over the future")
		}
		for _, o := range test.WidgetRenderer(c).Objects() {
			if txt, ok := o.(*canvas.Text); ok && txt.Visible() && txt.Text == "" {
				t.Errorf("an empty label is shown at %v", txt.Position())
			}
		}
	})
}

// navButton returns the header's icon-only button whose icon is named
// like name, such as "zoom-in". Call it from ta.do.
func (ta *testApp) navButton(name string) *widget.Button {
	ta.t.Helper()
	for _, b := range findAll(ta.content(ui.TabGraphing), func(b *widget.Button) bool {
		return b.Text == "" && b.Icon != nil && strings.Contains(b.Icon.Name(), name)
	}) {
		return b
	}
	return nil
}

// tapNav taps a header button by icon name.
func (ta *testApp) tapNav(name string) {
	ta.t.Helper()
	ta.do(func() {
		b := ta.navButton(name)
		if b == nil {
			ta.t.Fatalf("no %q button", name)
		}
		test.Tap(b)
	})
}

// timeLabels returns the chart's time axis labels as seconds, ignoring the
// cursor's when there is one, in the order they're drawn.
func (ta *testApp) timeLabels() (secs []float64) {
	ta.do(func() {
		for _, s := range texts(ta.content(ui.TabGraphing)) {
			if !elapsedLabel.MatchString(s) {
				continue
			}
			parts := strings.Split(s, ":")
			m, _ := strconv.ParseFloat(parts[0], 64)
			sec, _ := strconv.ParseFloat(parts[1], 64)
			secs = append(secs, m*60+sec)
		}
	})
	return secs
}

func span(secs []float64) float64 {
	if len(secs) == 0 {
		return math.NaN()
	}
	return slices.Max(secs) - slices.Min(secs)
}

func TestGraphingTabZoomButtonsAndReset(t *testing.T) {
	ta := newTestApp(t)
	ta.resize(800, 480)
	ta.show(ui.TabGraphing)
	ta.do(func() {
		if b := ta.navButton("zoom-in"); b != nil && b.Visible() {
			t.Error("zoom buttons shown without a chart")
		}
	})
	ta.openGraph(writeLog(t, "run.csv", graphColumns, 801)) // 40s
	whole := span(ta.timeLabels())
	if whole < 20 {
		t.Fatalf("whole log's labels span %vs: %v", whole, ta.timeLabels())
	}

	ta.tapNav("zoom-in")
	zoomed := span(ta.timeLabels())
	if zoomed >= whole*0.75 {
		t.Errorf("labels span %vs after zooming in, was %vs", zoomed, whole)
	}
	ta.tapNav("zoom-in")
	if again := span(ta.timeLabels()); again >= zoomed {
		t.Errorf("labels span %vs after zooming in again, was %vs", again, zoomed)
	}
	ta.tapNav("zoom-out")
	ta.tapNav("zoom-out")
	if out := span(ta.timeLabels()); math.Abs(out-whole) > whole*0.25 {
		t.Errorf("labels span %vs after zooming out again, was %vs", out, whole)
	}

	ta.tapNav("zoom-in")
	ta.tapNav("zoom-in")
	ta.tapNav("zoom-in")
	ta.tapNav("zoom-fit")
	if got := span(ta.timeLabels()); math.Abs(got-whole) > 0.5 {
		t.Errorf("labels span %vs after reset, want %vs", got, whole)
	}

	// changing the parameters keeps the zoom, and a new file resets it
	ta.tapNav("zoom-in")
	zoomed = span(ta.timeLabels())
	ta.do(func() { test.Tap(ta.columnChecksNow()[5]) })
	if got := span(ta.timeLabels()); math.Abs(got-zoomed) > 0.5 {
		t.Errorf("labels span %vs after charting another parameter, was %vs", got, zoomed)
	}
	ta.openGraph(writeLog(t, "other.csv", graphColumns, 801))
	if got := span(ta.timeLabels()); math.Abs(got-whole) > 0.5 {
		t.Errorf("labels span %vs after opening a file, want %vs", got, whole)
	}
}

// columnChecksNow is columnChecks for use inside ta.do.
func (ta *testApp) columnChecksNow() []*widget.Check {
	return findAll(ta.content(ui.TabGraphing), func(*widget.Check) bool { return true })
}

func TestGraphingTabDragPans(t *testing.T) {
	ta := newTestApp(t)
	ta.resize(800, 480)
	ta.openGraph(writeLog(t, "run.csv", graphColumns, 801))
	ta.tapNav("zoom-in")
	ta.tapNav("zoom-in")
	before := ta.timeLabels()

	// dragging left shows later times, so the labels read higher
	ta.do(func() {
		c := ta.chart()
		c.Dragged(&fyne.DragEvent{Dragged: fyne.NewDelta(-c.Size().Width/4, 0)})
	})
	after := ta.timeLabels()
	if len(before) == 0 || len(after) == 0 || slices.Max(after) <= slices.Max(before) {
		t.Errorf("labels %v then %v after dragging left, want later times", before, after)
	}
	ta.do(func() {
		c := ta.chart()
		c.Dragged(&fyne.DragEvent{Dragged: fyne.NewDelta(c.Size().Width*3, 0)})
	})
	if got := ta.timeLabels(); slices.Min(got) > 10 {
		t.Errorf("labels %v after dragging back past the start", got)
	}
}

func TestGraphingTabVerticalScrollScrollsTheLanes(t *testing.T) {
	ta := newTestApp(t)
	ta.resize(800, 300)
	ta.openGraph(writeLog(t, "run.csv", graphColumns, 801))
	var offset float32
	var scroll *container.Scroll
	ta.do(func() {
		for _, sc := range findAll(ta.content(ui.TabGraphing), func(*container.Scroll) bool { return true }) {
			if len(findAll(sc, func(*widget.Check) bool { return true })) == 0 {
				scroll = sc
			}
		}
		if scroll == nil {
			t.Fatal("no scroll for the chart")
		}
		if scroll.Content.MinSize().Height <= scroll.Size().Height {
			t.Fatalf("chart %v tall fits its %v tall scroll", scroll.Content.MinSize().Height, scroll.Size().Height)
		}
		c := ta.chart()
		c.Scrolled(&fyne.ScrollEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(300, 20)}, Scrolled: fyne.NewDelta(0, -40)})
		offset = scroll.Offset.Y
	})
	if offset <= 0 {
		t.Errorf("lane list offset %v after scrolling over the chart", offset)
	}
}

func TestGraphingTabLiveZoomPausesFollowing(t *testing.T) {
	ta := newTestApp(t)
	ta.connectLive("Engine Speed", "Vehicle Speed")
	ta.waitFor("data in the lanes", func() bool {
		return !slices.Contains(texts(ta.content(ui.TabGraphing)), "No data")
	})
	if !ta.refreshing() {
		t.Fatal("not following to begin with")
	}
	ta.tapNav("zoom-fit") // already following: nothing to resume
	if !ta.refreshing() {
		t.Error("reset while following stopped it")
	}
	ta.do(func() {
		if !slices.Contains(texts(ta.content(ui.TabGraphing)), "Live") {
			t.Errorf("header %v", texts(ta.content(ui.TabGraphing)))
		}
	})

	ta.tapNav("zoom-in")
	if ta.refreshing() {
		t.Fatal("still refreshing after zooming in")
	}
	var frozen []string
	ta.do(func() { frozen = texts(ta.content(ui.TabGraphing)) })
	if !slices.Contains(frozen, "Live (paused)") {
		t.Errorf("no sign following is paused: %v", frozen)
	}
	// data keeps coming in, but the view stays put
	time.Sleep(3 * 220 * time.Millisecond)
	ta.do(func() {
		got := texts(ta.content(ui.TabGraphing))
		timeAxis := func(s []string) (r []string) {
			for _, x := range s {
				if x == "now" || strings.HasPrefix(x, "-") {
					r = append(r, x)
				}
			}
			return r
		}
		if !slices.Equal(timeAxis(got), timeAxis(frozen)) {
			t.Errorf("time axis moved while paused: %v then %v", timeAxis(frozen), timeAxis(got))
		}
	})

	// more zooming and panning stay paused
	ta.tapNav("zoom-out")
	if ta.refreshing() {
		t.Error("zooming out resumed following")
	}

	ta.tapNav("play") // the reset button is the resume button while paused
	if !ta.refreshing() {
		t.Error("the reset button didn't resume following")
	}
	ta.do(func() {
		if got := texts(ta.content(ui.TabGraphing)); slices.Contains(got, "Live (paused)") || !slices.Contains(got, "Live") || !slices.Contains(got, "now") {
			t.Errorf("after resuming: %v", got)
		}
	})

	// with the whole window showing there's nowhere to pan to, so
	// dragging doesn't stop following
	ta.do(func() {
		c := ta.chart()
		c.Dragged(&fyne.DragEvent{Dragged: fyne.NewDelta(40, 0)})
	})
	if !ta.refreshing() {
		t.Error("dragging a whole window paused following")
	}
}

// cursorParts returns the cursor's line's x, and the text of its labels: the
// time first, then the lanes' readings. The overlay is drawn after the rest
// of the chart, starting with the line. Call it from ta.do.
func cursorParts(c chartWidget) (x float32, labels []string, ok bool) {
	for _, o := range test.WidgetRenderer(c).Objects() {
		if r, isRect := o.(*canvas.Rectangle); isRect && r.Visible() && r.Size().Width == 1 && !ok {
			x, ok = r.Position().X+0.5, true
			continue
		}
		if t, isText := o.(*canvas.Text); ok && isText && t.Visible() {
			labels = append(labels, t.Text)
		}
	}
	return x, labels, ok
}

// axisNumbers returns the numbers on the chart's y-axes.
func (ta *testApp) axisNumbers() (nums []float64) {
	ta.do(func() {
		for _, s := range texts(ta.content(ui.TabGraphing)) {
			if v, err := strconv.ParseFloat(s, 64); err == nil {
				nums = append(nums, v)
			}
		}
	})
	return nums
}

// writeSparseLog writes a log of one lane, Engine Speed, holding 10i at 2i+1
// seconds for i from 0 to 19, so samples fall on odd seconds from 1 to 39.
func writeSparseLog(t *testing.T) string {
	t.Helper()
	csv := "Timestamp,Engine Speed (rpm)\n"
	start := time.Date(2026, 1, 2, 3, 4, 5, 0, time.Local)
	for i := range 20 {
		at := start.Add(time.Duration(2*i+1) * time.Second)
		csv += fmt.Sprintf("%s,%d.0000\n", at.Format("2006-01-02 15:04:05.999999999"), 10*i)
	}
	path := filepath.Join(t.TempDir(), "sparse.csv")
	if err := os.WriteFile(path, []byte(csv), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// The zoom buttons zoom about the middle, no further in than a second and no
// further out than the whole log.
func TestGraphingTabZoomLimits(t *testing.T) {
	ta := newTestApp(t)
	ta.resize(800, 480)
	ta.openGraph(writeLog(t, "run.csv", graphColumns, 801)) // 0 to 40s

	ta.tapNav("zoom-in") // 10 to 30s
	if got := ta.timeLabels(); slices.Min(got) < 10 || slices.Max(got) > 30 {
		t.Errorf("labels %v after zooming in once, want within 10 to 30s", got)
	}
	for range 10 {
		ta.tapNav("zoom-in")
	}
	inmost := ta.timeLabels()
	if s := span(inmost); s < 0.5 || s > 1 {
		t.Errorf("labels %v span %vs zoomed all the way in, want the 1s view's", inmost, s)
	}
	ta.tapNav("zoom-in")
	if got := ta.timeLabels(); !slices.Equal(got, inmost) {
		t.Errorf("zoomed in past the limit: %v then %v", inmost, got)
	}
	for range 10 {
		ta.tapNav("zoom-out")
	}
	if got := ta.timeLabels(); slices.Min(got) < 0 || slices.Max(got) > 40 || span(got) < 30 {
		t.Errorf("labels %v zoomed all the way out, want the whole 40s log", got)
	}
}

// Panning stops at the ends of the log.
func TestGraphingTabPanStopsAtTheData(t *testing.T) {
	ta := newTestApp(t)
	ta.resize(800, 480)
	ta.openGraph(writeLog(t, "run.csv", graphColumns, 801))
	ta.tapNav("zoom-in")
	ta.tapNav("zoom-in")                  // 10s
	for _, dx := range []float32{-1, 1} { // later, then earlier
		ta.do(func() {
			c := ta.chart()
			c.Dragged(&fyne.DragEvent{Dragged: fyne.NewDelta(dx*c.Size().Width*20, 0)})
		})
		got := ta.timeLabels()
		if slices.Min(got) < 0 || slices.Max(got) > 40 || span(got) > 10 {
			t.Errorf("labels %v after dragging far %+v, want a 10s view within the log", got, dx)
		}
	}
}

// Each lane's y-axis follows the values in view, and with none in view, the
// samples either side.
func TestGraphingTabYAxisFollowsTheVisibleValues(t *testing.T) {
	ta := newTestApp(t)
	ta.resize(800, 480)
	ta.openGraph(writeLog(t, "run.csv", []string{"P8"}, 801)) // 1 to 801 over 40s
	if top := slices.Max(ta.axisNumbers()); top < 800 {
		t.Fatalf("axis tops out at %v for the whole log", top)
	}
	ta.tapNav("zoom-in")
	ta.tapNav("zoom-in")
	ta.tapNav("zoom-in") // 17.5 to 22.5s: about 351 to 451
	if nums := ta.axisNumbers(); slices.Min(nums) < 300 || slices.Max(nums) > 500 {
		t.Errorf("axis %v zoomed in on the middle, want about 351 to 451", nums)
	}

	ta.openGraph(writeSparseLog(t))
	for range 10 {
		ta.tapNav("zoom-in") // to 19.5 to 20.5s, between the samples at 19 and 21s
	}
	// the samples either side hold 90 and 100; either alone would be padded
	// out past 85 to 105
	if nums := ta.axisNumbers(); len(nums) == 0 || slices.Min(nums) < 85 || slices.Max(nums) > 105 {
		t.Errorf("axis %v between samples, want about 90 to 100", nums)
	}
}

// Dragging moves the cursor with the pointer, holding it on the moment
// grabbed, and pins nothing.
func TestGraphingTabDragHoldsTheCursorOnTheMoment(t *testing.T) {
	ta := newTestApp(t)
	ta.resize(800, 480)
	ta.openGraph(writeLog(t, "run.csv", graphColumns, 801))
	ta.tapNav("zoom-in")
	ta.tapNav("zoom-in")
	ta.do(func() {
		c := ta.chart()
		x0 := c.Size().Width * 0.5
		hoverAt(c, x0, 10)
		_, labels, ok := cursorParts(c)
		if !ok {
			t.Fatal("no cursor")
		}
		grabbed := elapsedSeconds(t, labels[0])

		x1 := x0 - 60
		c.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(x1, 10)}, Dragged: fyne.NewDelta(-60, 0)})
		x, labels, ok := cursorParts(c)
		if !ok || math.Abs(float64(x-x1)) > 1 {
			t.Errorf("cursor at %v after dragging to %v", x, x1)
		}
		if got := elapsedSeconds(t, labels[0]); math.Abs(got-grabbed) > 0.05 {
			t.Errorf("cursor moved from %vs to %vs while dragging", grabbed, got)
		}
		c.DragEnd()
		c.MouseOut()
		if cursorShown(c) {
			t.Error("dragging pinned the cursor")
		}
	})
}

// A pinned cursor stays through a pan that keeps its time in view, but not a
// zoom.
func TestGraphingTabPinnedCursorSurvivesPanButNotZoom(t *testing.T) {
	ta := newTestApp(t)
	ta.resize(800, 480)
	ta.openGraph(writeLog(t, "run.csv", graphColumns, 801))
	ta.tapNav("zoom-in")
	ta.do(func() {
		c := ta.chart()
		x := c.Size().Width * 0.5
		tapAt(c, x, 10)
		c.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(x-20, 10)}, Dragged: fyne.NewDelta(-20, 0)})
		c.DragEnd()
		c.MouseOut()
		if !cursorShown(c) {
			t.Error("pinned cursor lost to a pan that kept its time in view")
		}
	})
	ta.tapNav("zoom-in")
	ta.do(func() {
		if cursorShown(ta.chart()) {
			t.Error("pinned cursor survived zooming")
		}
	})
}

// Scrolling sideways pans, even with a little vertical in it, and leaves the
// lane list where it is.
func TestGraphingTabSidewaysScrollPans(t *testing.T) {
	ta := newTestApp(t)
	ta.resize(800, 300)
	ta.openGraph(writeLog(t, "run.csv", graphColumns, 801))
	ta.tapNav("zoom-in")
	ta.tapNav("zoom-in")
	for _, d := range []fyne.Delta{fyne.NewDelta(-100, 0), fyne.NewDelta(-100, 6)} {
		before := ta.timeLabels()
		var offset, was float32
		ta.do(func() {
			var scroll *container.Scroll
			for _, sc := range findAll(ta.content(ui.TabGraphing), func(*container.Scroll) bool { return true }) {
				if len(findAll(sc, func(*widget.Check) bool { return true })) == 0 {
					scroll = sc
				}
			}
			was = scroll.Offset.Y
			ta.chart().Scrolled(&fyne.ScrollEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(300, 20)}, Scrolled: d})
			offset = scroll.Offset.Y
		})
		if after := ta.timeLabels(); slices.Max(after) <= slices.Max(before) {
			t.Errorf("scrolling %v: labels %v then %v, want later times", d, before, after)
		}
		if offset != was {
			t.Errorf("scrolling %v moved the lane list from %v to %v", d, was, offset)
		}
	}
}

// On a live chart the zoom buttons zoom about now, so the present stays in
// view.
func TestGraphingTabLiveZoomButtonsKeepNow(t *testing.T) {
	ta := newTestApp(t)
	start := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	clock := &testClock{t: start}
	ta.do(func() { ta.GraphingTab.SetClock(clock.now) })
	ta.connectLive("Engine Speed")
	ta.waitFor("data in the lane", func() bool {
		return !slices.Contains(texts(ta.content(ui.TabGraphing)), "No data")
	})
	clock.set(start.Add(30 * time.Second))
	ta.waitFor("the view to reach back 30s", func() bool {
		return slices.Max(append(timeLabelsAgo(t, ta.content(ui.TabGraphing)), 0)) >= 20
	})

	for range 2 {
		ta.tapNav("zoom-in")
		ta.do(func() {
			if got := texts(ta.content(ui.TabGraphing)); !slices.Contains(got, "now") {
				t.Errorf("zooming in lost now: %v", timeLabelsAgo(t, ta.content(ui.TabGraphing)))
			}
		})
	}
}
