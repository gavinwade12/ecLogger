package main_test

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	ui "github.com/gavinwade12/ecLogger/cmd/logger-ui"
)

// liveParams are the names of parameters to log, in the order the live
// graph lists them: sorted by name.
var liveParams = []string{
	"A/F Correction #1", "Coolant Temperature", "Engine Speed",
	"Manifold Absolute Pressure", "Mass Airflow", "Vehicle Speed",
}

// connectLive connects and live logs the named parameters.
func (ta *testApp) connectLive(names ...string) {
	ta.t.Helper()
	ta.connect()
	for _, n := range names {
		ta.setLiveLog(n, true)
	}
	ta.show(ui.TabGraphing)
}

func (ta *testApp) refreshing() (r bool) {
	ta.do(func() { r = ta.GraphingTab.Refreshing() })
	return r
}

func TestGraphingTabShowsLiveValues(t *testing.T) {
	ta := newTestApp(t)
	ta.connectLive(liveParams...)

	ta.do(func() {
		if !slices.Contains(texts(ta.content(ui.TabGraphing)), "Live") {
			t.Errorf("no Live in the header: %v", texts(ta.content(ui.TabGraphing)))
		}
	})
	checks := ta.columnChecks()
	if len(checks) != len(liveParams) {
		t.Fatalf("got %d checks, want %d: %v", len(checks), len(liveParams), checkLabels(checks))
	}
	for i, c := range checks {
		if want := i < 4; c.Checked != want {
			t.Errorf("%q checked = %v, want %v", c.Text, c.Checked, want)
		}
		if want := liveParams[i]; len(c.Text) <= len(want) || c.Text[:len(want)] != want {
			t.Errorf("check %d is %q, want %q and its unit", i, c.Text, want)
		}
	}
	titles := ta.chartedTitles()
	if len(titles) != 4 {
		t.Errorf("charted %v, want 4 lanes", titles)
	}

	// values flow into the lanes, which get axes in place of "No data"
	ta.waitFor("data in the lanes", func() bool {
		return !slices.Contains(texts(ta.content(ui.TabGraphing)), "No data")
	})
	ta.do(func() {
		if !slices.Contains(texts(ta.content(ui.TabGraphing)), "now") {
			t.Errorf("no time label for now: %v", texts(ta.content(ui.TabGraphing)))
		}
	})
	if !ta.refreshing() {
		t.Error("the live chart isn't being refreshed while it's showing")
	}
}

func TestGraphingTabLiveRefreshOnlyWhileVisible(t *testing.T) {
	ta := newTestApp(t)
	if ta.refreshing() {
		t.Error("refreshing before connecting")
	}
	ta.connectLive("Engine Speed")
	if !ta.refreshing() {
		t.Fatal("not refreshing while live and showing")
	}

	ta.show(ui.TabLogging)
	if ta.refreshing() {
		t.Error("still refreshing with another tab showing")
	}
	// history is kept while it isn't shown
	time.Sleep(50 * time.Millisecond)
	ta.show(ui.TabGraphing)
	if !ta.refreshing() {
		t.Error("not refreshing after the tab is shown again")
	}
	ta.waitFor("data in the lane", func() bool {
		return !slices.Contains(texts(ta.content(ui.TabGraphing)), "No data")
	})

	ta.do(ta.OnDisconnect)
	if ta.refreshing() {
		t.Error("still refreshing after disconnecting")
	}
}

func TestGraphingTabLiveWithoutParameters(t *testing.T) {
	ta := newTestApp(t)
	ta.connect()
	ta.show(ui.TabGraphing)
	if !ta.hasTextWithPrefix(ui.TabGraphing, "No parameters are being logged") {
		t.Errorf("no hint about logging parameters: %v", ta.texts(ui.TabGraphing))
	}
	if ta.refreshing() {
		t.Error("refreshing with nothing to chart")
	}

	ta.tap(ui.TabGraphing, "Choose Parameters")
	ta.do(func() {
		if got := ta.tabs().SelectedIndex(); got != int(ui.TabParameters) {
			t.Errorf("selected tab = %d, want the Parameters tab", got)
		}
	})

	ta.setLiveLog("Engine Speed", true)
	ta.show(ui.TabGraphing)
	if ta.hasTextWithPrefix(ui.TabGraphing, "No parameters are being logged") {
		t.Error("still hinting after a parameter is logged")
	}
	if !ta.refreshing() {
		t.Error("not refreshing once a parameter is logged")
	}
}

func TestGraphingTabLiveListFollowsLoggedParameters(t *testing.T) {
	ta := newTestApp(t)
	ta.connectLive(liveParams...)

	// the user's checks are kept, and parameters added later aren't
	// charted by default when four already are
	checks := ta.columnChecks()
	ta.do(func() {
		test.Tap(checks[0]) // A/F Correction off
		test.Tap(checks[5]) // Vehicle Speed on
	})
	ta.setLiveLog("Coolant Temperature", false)
	ta.show(ui.TabGraphing)

	checks = ta.columnChecks()
	got := map[string]bool{}
	for _, c := range checks {
		got[c.Text[:len(c.Text)-len(" ()")-len(unitOf(c.Text))]] = c.Checked
	}
	want := map[string]bool{
		"A/F Correction #1": false, "Engine Speed": true, "Manifold Absolute Pressure": true,
		"Mass Airflow": false, "Vehicle Speed": true,
	}
	if len(got) != len(want) {
		t.Fatalf("list is %v, want %v", got, want)
	}
	for name, on := range want {
		if got[name] != on {
			t.Errorf("%q checked = %v, want %v", name, got[name], on)
		}
	}
	if titles := ta.chartedTitles(); len(titles) != 3 {
		t.Errorf("charted %v, want 3 lanes", titles)
	}

	// file logged parameters are read by the session too
	ta.setFileLog("Coolant Temperature", true)
	ta.show(ui.TabGraphing)
	if n := len(ta.columnChecks()); n != 6 {
		t.Errorf("%d parameters listed after logging one to file, want 6", n)
	}
}

// unitOf returns the unit in a check's "Name (unit)" text.
func unitOf(text string) string {
	for i := len(text) - 1; i >= 0; i-- {
		if text[i] == '(' {
			return text[i+1 : len(text)-1]
		}
	}
	return ""
}

func TestGraphingTabLiveUsesConfiguredUnit(t *testing.T) {
	ta := newTestApp(t)
	ta.connect()
	ta.setLiveLog("Vehicle Speed", true)
	unit := ta.paramRow("Vehicle Speed").unit
	ta.do(func() { unit.SetSelected("mph") })
	ta.show(ui.TabGraphing)
	if labels := checkLabels(ta.columnChecks()); !slices.Equal(labels, []string{"Vehicle Speed (mph)"}) {
		t.Errorf("labels = %v", labels)
	}

	// also when it's logged to file only, which doesn't restart the session
	ta.setLiveLog("Vehicle Speed", false)
	ta.setFileLog("Vehicle Speed", true)
	ta.do(func() { unit.SetSelected("km/h") })
	ta.show(ui.TabGraphing)
	if labels := checkLabels(ta.columnChecks()); !slices.Equal(labels, []string{"Vehicle Speed (km/h)"}) {
		t.Errorf("labels after changing the unit = %v", labels)
	}
}

func TestGraphingTabOpenFileAndBackToLive(t *testing.T) {
	ta := newTestApp(t)
	ta.connectLive("Engine Speed", "Vehicle Speed")
	if ta.buttonShown(ui.TabGraphing, "Live") {
		t.Error("a Live button is shown in live mode")
	}

	ta.openGraph(writeLog(t, "run.csv", graphColumns, 40))
	if !slices.Contains(ta.texts(ui.TabGraphing), "run.csv") {
		t.Errorf("file name not shown: %v", ta.texts(ui.TabGraphing))
	}
	if n := len(ta.columnChecks()); n != len(graphColumns) {
		t.Errorf("%d columns listed for the file, want %d", n, len(graphColumns))
	}
	if ta.refreshing() {
		t.Error("refreshing the live chart with a file open")
	}

	ta.tap(ui.TabGraphing, "Live")
	if slices.Contains(ta.texts(ui.TabGraphing), "run.csv") || !slices.Contains(ta.texts(ui.TabGraphing), "Live") {
		t.Errorf("not back to live: %v", ta.texts(ui.TabGraphing))
	}
	if labels := checkLabels(ta.columnChecks()); len(labels) != 2 {
		t.Errorf("live list = %v, want the 2 logged parameters", labels)
	}
	if ta.buttonShown(ui.TabGraphing, "Live") {
		t.Error("the Live button is still shown")
	}
	if !ta.refreshing() {
		t.Error("not refreshing after going back to live")
	}
}

func TestGraphingTabKeepsFileAcrossDisconnect(t *testing.T) {
	ta := newTestApp(t)
	ta.connectLive("Engine Speed")
	ta.openGraph(writeLog(t, "run.csv", graphColumns, 40))

	ta.do(ta.OnDisconnect)
	if !slices.Contains(ta.texts(ui.TabGraphing), "run.csv") {
		t.Errorf("file closed by disconnecting: %v", ta.texts(ui.TabGraphing))
	}
	// with nothing live to go back to, the button closes the file
	if !ta.buttonShown(ui.TabGraphing, "Close") {
		t.Fatal("no Close button")
	}
	ta.tap(ui.TabGraphing, "Close")
	if !ta.hasTextWithPrefix(ui.TabGraphing, "Open a log file") {
		t.Errorf("no hint after closing the file: %v", ta.texts(ui.TabGraphing))
	}
	if n := len(ta.columnChecks()); n != 0 {
		t.Errorf("%d checks listed with nothing to graph", n)
	}
}

func TestGraphingTabDisconnectShowsHint(t *testing.T) {
	ta := newTestApp(t)
	ta.connectLive("Engine Speed")
	ta.waitFor("data in the lane", func() bool {
		return !slices.Contains(texts(ta.content(ui.TabGraphing)), "No data")
	})

	ta.do(ta.OnDisconnect)
	if !ta.hasTextWithPrefix(ui.TabGraphing, "Open a log file to graph it, or connect") {
		t.Errorf("no hint after disconnecting: %v", ta.texts(ui.TabGraphing))
	}
	if slices.Contains(ta.texts(ui.TabGraphing), "Live") {
		t.Error("still shows Live when disconnected")
	}
	if n := len(ta.columnChecks()); n != 0 {
		t.Errorf("%d checks listed while disconnected", n)
	}
	if !ta.buttonShown(ui.TabGraphing, "Open…") {
		t.Error("no Open button while disconnected")
	}

	// connecting again starts afresh, with the parameters still logged
	ta.connect()
	ta.show(ui.TabGraphing)
	ta.waitFor("the logged parameter to be listed again", func() bool {
		return len(findAll(ta.content(ui.TabGraphing), func(*widget.Check) bool { return true })) == 1
	})
}

func TestSettingsLiveGraphWindowAppliesAtOnce(t *testing.T) {
	ta := newTestApp(t)
	ta.connectLive("Engine Speed")
	ta.do(func() {
		if got := ta.GraphingTab.LiveWindow(); got != 5*time.Minute {
			t.Errorf("window = %v, want 5m0s", got)
		}
	})
	sel := ta.liveWindowSelect()
	ta.do(func() { sel.SetSelected("1 minute") })
	ta.do(func() {
		if got := ta.GraphingTab.LiveWindow(); got != time.Minute {
			t.Errorf("window = %v, want 1m0s", got)
		}
	})
	// the time axis follows: labels reach back about a minute
	ta.show(ui.TabGraphing)
	ta.do(func() {
		for _, s := range texts(ta.content(ui.TabGraphing)) {
			if s == "-2:00" || s == "-4:00" {
				t.Errorf("time label %q on a 1 minute window", s)
			}
		}
	})
}

// testClock is a clock a test sets, for the live graph to time values by.
type testClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *testClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *testClock) set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = t
}

var timeLabelRe = regexp.MustCompile(`^(now|-(\d+)(\.\d)?s|-(\d+):(\d\d))$`)

// timeLabelsAgo returns how long ago each of the time axis's labels is, in
// seconds, failing the test if a label isn't one a live graph should show.
// Other text is told apart by how it looks: no other label is "now" or
// starts with a minus.
func timeLabelsAgo(t *testing.T, root fyne.CanvasObject) (ago []float64) {
	t.Helper()
	all := texts(root)
	if n := countOf(all, "now"); n > 1 {
		t.Errorf("%d labels say now: %v", n, all)
	}
	for _, s := range all {
		if s != "now" && !strings.HasPrefix(s, "-") && !strings.HasPrefix(s, "+") {
			continue
		}
		m := timeLabelRe.FindStringSubmatch(s)
		switch {
		case m == nil:
			t.Errorf("time label %q", s)
		case s == "now":
			ago = append(ago, 0)
		case m[2] != "":
			secs, _ := strconv.ParseFloat(m[2]+m[3], 64)
			ago = append(ago, secs)
		default:
			mins, _ := strconv.Atoi(m[4])
			secs, _ := strconv.Atoi(m[5])
			ago = append(ago, float64(mins*60+secs))
		}
	}
	return ago
}

// The live view starts at the first value, so the trace runs from the left
// edge, and widens as data arrives until it's the whole window.
func TestGraphingTabLiveViewGrowsThenFollows(t *testing.T) {
	ta := newTestApp(t)
	start := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	clock := &testClock{t: start}
	ta.do(func() { ta.GraphingTab.SetClock(clock.now) })
	sel := ta.liveWindowSelect()
	ta.do(func() { sel.SetSelected("1 minute") })
	ta.connectLive("Engine Speed")
	ta.waitFor("data in the lane", func() bool {
		return !slices.Contains(texts(ta.content(ui.TabGraphing)), "No data")
	})

	// the oldest label shown, in seconds ago
	oldest := func() float64 {
		return slices.Max(append(timeLabelsAgo(t, ta.content(ui.TabGraphing)), 0))
	}
	for _, step := range []struct {
		name     string
		elapsed  time.Duration
		min, max float64
	}{
		// now is at the left edge, with only the future, unlabeled, after it
		{"just connected", 0, 0, 0},
		// the view is still its minimum 10s, starting at the first value
		{"young", 5 * time.Second, 1, 5},
		// the view runs from the first value to now
		{"growing", 30 * time.Second, 20, 30},
		// the view is the last minute, not all 90s since the first value
		{"full", 90 * time.Second, 50, 60},
	} {
		clock.set(start.Add(step.elapsed))
		ta.waitFor(step.name+" view", func() bool {
			got := oldest()
			return got >= step.min && got <= step.max
		})
	}
}

// A lane has one unit, so a parameter's history in the old one is dropped
// when its unit changes, and its axis only covers the new values.
func TestGraphingTabLiveDropsHistoryWhenUnitChanges(t *testing.T) {
	ta := newTestApp(t)
	ta.ecu.answerLiveWith(100) // 100 km/h
	ta.connectLive("Vehicle Speed")

	// the largest number on the lane's y-axis
	axisTop := func() (top float64) {
		for _, s := range texts(ta.content(ui.TabGraphing)) {
			if v, err := strconv.ParseFloat(s, 64); err == nil {
				top = max(top, v)
			}
		}
		return top
	}
	ta.waitFor("km/h values on the axis", func() bool { return axisTop() >= 100 })

	unit := ta.paramRow("Vehicle Speed").unit
	ta.do(func() { unit.SetSelected("mph") })
	ta.show(ui.TabGraphing)
	if labels := checkLabels(ta.columnChecks()); !slices.Equal(labels, []string{"Vehicle Speed (mph)"}) {
		t.Errorf("labels = %v", labels)
	}
	// 62 mph; with the km/h history kept the axis would reach 100
	ta.waitFor("only mph values on the axis", func() bool {
		top := axisTop()
		return top > 0 && top < 80
	})
}

func countOf(ss []string, s string) (n int) {
	for _, x := range ss {
		if x == s {
			n++
		}
	}
	return n
}
