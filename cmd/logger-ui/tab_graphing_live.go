package main

import (
	"fmt"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
	"github.com/gavinwade12/ecLogger/internal/logging"
	"github.com/gavinwade12/ecLogger/protocols/ssm2"
	"github.com/gavinwade12/ecLogger/units"
)

const (
	// disconnectedHint is shown when there's nothing to graph.
	disconnectedHint = "Open a log file to graph it, or connect to graph live values. " +
		"Log files are written from the Logging tab."

	// liveRefreshInterval is how often the live chart is redrawn: a few
	// times a second is plenty to read, and redrawing it is the costly
	// part of graphing.
	liveRefreshInterval = 220 * time.Millisecond

	// liveAheadDivisor sets how far past now the view runs: 1/40th of its
	// span, so the tick and label for now aren't at the very edge, where
	// they'd be cut off.
	liveAheadDivisor = 40

	// liveMinSpan is the narrowest the live chart's view gets. While the
	// data is younger than this, the trace grows rightward from the left
	// edge; after, it's stretched to fill the chart until it's as long as
	// the window.
	liveMinSpan = 10 * time.Second
)

// liveEntry is a parameter the logging session reads, which can be graphed
// live.
type liveEntry struct {
	id, name string
	unit     units.Unit
}

// live is the GraphingTab's live graphing state. Live mode is when the app
// is connected and no file is open.
type live struct {
	// window holds the recent values. It's fed for the whole connection,
	// not just while the tab is shown, so history is there when it is.
	window *logging.Window
	// now is the clock, replaced in tests
	now  func() time.Time
	span time.Duration

	entries []liveEntry
	// checkIDs are the ids of the parameter list's checks, when live
	checkIDs []string
	// liveChecked remembers which parameters are checked across changes to
	// the logged parameters, for the length of a connection
	liveChecked map[string]bool

	// lastUnits are the units values last arrived in, to tell when one
	// changes. It's used on the logging session's goroutine.
	unitsMu   sync.Mutex
	lastUnits map[string]units.Unit

	// selected is whether the Graphing tab is the one showing
	selected bool
	// lanesStale is when the lanes need building before they're shown,
	// since the checks changed while the tab was hidden
	lanesStale bool
	// stopTick stops the goroutine that redraws the chart, and is nil when
	// it isn't running
	stopTick chan struct{}
	// tickPending is set while a redraw waits for the main thread, so a
	// slow one isn't queued behind more
	tickPending atomic.Bool
}

func (t *GraphingTab) initLive() {
	t.now = time.Now
	t.window = logging.NewWindow()
	t.lastUnits = map[string]units.Unit{}
	t.liveChecked = map[string]bool{}
	t.span = t.app.config.LiveGraphWindow()
	t.window.SetWindow(t.span)
	t.app.LoggingTab.runner.Subscribe("graphing", t.onValues)
}

// onValues adds a set of values to the window. It runs on the logging
// session's goroutine.
func (t *GraphingTab) onValues(values map[string]ssm2.ParameterValue) {
	now := t.now()
	floats := make(map[string]float64, len(values))
	t.unitsMu.Lock()
	for id, v := range values {
		// A lane has one unit, so history in another one is dropped.
		if u, ok := t.lastUnits[id]; ok && u != v.Unit {
			t.window.Drop(id)
		}
		t.lastUnits[id] = v.Unit
		floats[id] = float64(v.Value)
	}
	t.unitsMu.Unlock()
	t.window.AppendAll(now, floats)
}

// SetClock replaces the clock live values are timed by and the live view
// follows, which is time.Now. It's for tests, and must be called before
// connecting.
func (t *GraphingTab) SetClock(now func() time.Time) { t.now = now }

// LiveWindow returns how much history the live graph shows.
func (t *GraphingTab) LiveWindow() time.Duration { return t.span }

// SetLiveWindow sets how much history the live graph shows.
func (t *GraphingTab) SetLiveWindow(d time.Duration) {
	if d <= 0 {
		return
	}
	t.span = d
	t.window.SetWindow(d)
	if t.tickRunning() {
		t.refreshLive()
	}
}

// Refreshing reports whether the live chart is being redrawn on a timer,
// which it is only while it can be seen.
func (t *GraphingTab) Refreshing() bool { return t.tickRunning() }

func (t *GraphingTab) tickRunning() bool { return t.stopTick != nil }

// liveMode reports whether the tab graphs live values.
func (t *GraphingTab) liveMode() bool {
	return !t.fileMode && t.app.connection != nil
}

// onConnect starts a connection's history afresh.
func (t *GraphingTab) onConnect() {
	t.resetHistory()
}

func (t *GraphingTab) resetHistory() {
	t.unitsMu.Lock()
	clear(t.lastUnits)
	t.window.Reset()
	t.unitsMu.Unlock()
	clear(t.liveChecked)
}

// setSelected is told when the Graphing tab is shown or another tab is.
func (t *GraphingTab) setSelected(selected bool) {
	t.selected = selected
	if selected && t.lanesStale && t.liveMode() {
		t.updateChart()
	}
	t.syncTicker()
}

// onLoggedParamsChanged follows a change to what the session reads,
// including connecting and disconnecting.
func (t *GraphingTab) onLoggedParamsChanged() {
	var entries []liveEntry
	if t.app.connection != nil {
		for id, lp := range t.app.loggedParams.Snapshot() {
			var name string
			if lp.Derived {
				d, ok := ssm2.DerivedParameters[id]
				if !ok {
					continue
				}
				name = d.Name
			} else {
				p, ok := ssm2.Parameters[id]
				if !ok {
					continue
				}
				name = p.Name
			}
			entries = append(entries, liveEntry{id: id, name: name, unit: lp.Unit})
		}
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].name != entries[j].name {
				return entries[i].name < entries[j].name
			}
			return entries[i].id < entries[j].id
		})
	}

	if t.app.connection == nil {
		t.resetHistory()
	} else {
		// keep the history of what's still logged
		still := make(map[string]bool, len(entries))
		for _, e := range entries {
			still[e.id] = true
		}
		for _, old := range t.entries {
			if !still[old.id] {
				t.unitsMu.Lock()
				t.window.Drop(old.id)
				delete(t.lastUnits, old.id)
				t.unitsMu.Unlock()
			}
		}
	}
	t.entries = entries
	t.showCurrent()
}

// showCurrent shows what the tab should when nothing it's showing was chosen
// by the user: the live graph or the hint to open a file.
func (t *GraphingTab) showCurrent() {
	switch {
	case t.fileMode:
		t.updateView()
	case t.liveMode():
		t.rebuildLiveList()
	default:
		t.resetList()
		t.fileName.SetText("")
		t.setHint(disconnectedHint)
		t.updateView()
	}
	t.syncTicker()
}

// rebuildLiveList lists the logged parameters to choose from.
func (t *GraphingTab) rebuildLiveList() {
	t.resetList()
	t.fileName.SetText("Live")

	checked := 0
	for _, e := range t.entries {
		if t.liveChecked[e.id] {
			checked++
		}
	}
	for _, e := range t.entries {
		if _, known := t.liveChecked[e.id]; !known {
			t.liveChecked[e.id] = checked < defaultGraphedColumns
			if t.liveChecked[e.id] {
				checked++
			}
		}
	}
	for _, e := range t.entries {
		e := e
		check := widget.NewCheck(paramTitle(e.name, e.unit), nil)
		check.Checked = t.liveChecked[e.id]
		check.OnChanged = func(on bool) {
			t.liveChecked[e.id] = on
			t.updateChart()
		}
		t.checks = append(t.checks, check)
		t.checkIDs = append(t.checkIDs, e.id)
		t.checkList.Add(check)
	}
	t.checkList.Refresh()
	t.updateChart()
}

// updateLiveChart charts the checked parameters. The lanes are only built
// while the tab is shown; otherwise they wait for it to be.
func (t *GraphingTab) updateLiveChart() {
	defer t.syncTicker()
	n := 0
	for _, c := range t.checks {
		if c.Checked {
			n++
		}
	}
	t.charted, t.needsParams = n > 0, false
	switch {
	case len(t.entries) == 0:
		t.needsParams = true
		t.status.SetText("No parameters are being logged, so there is nothing to graph live.")
	case n == 0:
		t.status.SetText("Choose parameters to graph")
	default:
		t.status.SetText("")
	}

	if !t.selected {
		t.lanesStale = true
		t.updateView()
		return
	}
	t.lanesStale = false
	var lanes []chartLane
	for i, c := range t.checks {
		if c.Checked {
			lanes = append(lanes, t.liveLane(t.entries[i]))
		}
	}
	t.chart.SetLanes(lanes)
	t.refreshLive()
	t.updateView()
}

func (t *GraphingTab) liveLane(e liveEntry) chartLane {
	return chartLane{
		Name: e.name,
		Unit: e.unit,
		Downsample: func(from, to time.Time, n int) logging.Downsampled {
			return t.window.Downsample(e.id, from, to, n)
		},
		Range: func() (float64, float64, bool) { return t.window.Range(e.id) },
	}
}

// refreshLive moves the chart on to the present and redraws it.
func (t *GraphingTab) refreshLive() {
	now := t.now()
	from, to := t.liveView(now)
	t.chart.SetView(from, to, now, relativeLabels(now))
}

// liveView returns the span the live chart shows at now. It starts at the
// oldest value held, so the trace runs from the left edge and the view
// widens as data arrives, until it's the whole window and old data drops off
// the left. It's never narrower than liveMinSpan (or the window, if that's
// shorter), and it ends a little past now so now's label isn't cut off.
func (t *GraphingTab) liveView(now time.Time) (from, to time.Time) {
	from = now.Add(-t.span)
	if oldest, ok := t.window.Oldest(); !ok {
		from = now
	} else if oldest.After(from) {
		from = oldest
	}
	shown := max(now.Sub(from), min(liveMinSpan, t.span))
	return from, from.Add(shown + shown/liveAheadDivisor)
}

// syncTicker starts or stops the timer that redraws the live chart, so it
// runs only while the chart is live and can be seen.
func (t *GraphingTab) syncTicker() {
	want := t.selected && t.liveMode() && t.charted
	switch {
	case want && t.stopTick == nil:
		stop := make(chan struct{})
		t.stopTick = stop
		t.refreshLive()
		go t.runTicker(stop)
	case !want && t.stopTick != nil:
		close(t.stopTick)
		t.stopTick = nil
	}
}

func (t *GraphingTab) runTicker(stop chan struct{}) {
	ticker := time.NewTicker(liveRefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
		}
		if !t.tickPending.CompareAndSwap(false, true) {
			continue
		}
		fyne.Do(func() {
			t.tickPending.Store(false)
			select {
			case <-stop:
			default:
				t.refreshLive()
			}
		})
	}
}

// relativeLabels labels times by how long before now they were, like -30s,
// -1:00 or now, which is how a live graph is read. Times after now, which
// the view shows while it's still filling, aren't labeled.
func relativeLabels(now time.Time) timeLabeler {
	return func(t time.Time, step time.Duration) string {
		ago := now.Sub(t)
		switch {
		case ago <= -step/2:
			return ""
		case ago < step/2:
			return "now"
		case step < time.Second:
			return "-" + strconv.FormatFloat(ago.Seconds(), 'f', 1, 64) + "s"
		case ago < time.Minute:
			return fmt.Sprintf("-%ds", int(ago.Round(time.Second)/time.Second))
		}
		return formatElapsed(-ago, step)
	}
}
