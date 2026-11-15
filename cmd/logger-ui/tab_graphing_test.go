package main_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	ui "github.com/gavinwade12/ecLogger/cmd/logger-ui"
	"github.com/gavinwade12/ecLogger/internal/logging"
	"github.com/gavinwade12/ecLogger/protocols/ssm2"
	"github.com/gavinwade12/ecLogger/units"
)

// writeLog writes a log of the given parameters, with rows rows of values,
// to a new file and returns its path.
func writeLog(t *testing.T, name string, ids []string, rows int) string {
	t.Helper()
	var params []ssm2.Parameter
	for _, id := range ids {
		params = append(params, ssm2.Parameters[id])
	}
	path := filepath.Join(t.TempDir(), name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	w, err := logging.NewCSVWriter(f, params, nil, func(_ string, u units.Unit) units.Unit { return u })
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 1, 2, 3, 4, 5, 0, time.Local)
	for i := range rows {
		values := map[string]ssm2.ParameterValue{}
		for j, id := range ids {
			values[id] = ssm2.ParameterValue{Value: float32(i*(j+1)) + 1}
		}
		if err := w.Write(start.Add(time.Duration(i)*50*time.Millisecond), values); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

// graphColumns are the parameters of the logs the graphing tests write.
var graphColumns = []string{"P8", "P9", "P7", "P12", "P2", "P3"}

// openGraph opens a log in the Graphing tab and waits for it to load.
func (ta *testApp) openGraph(path string) {
	ta.t.Helper()
	ta.show(ui.TabGraphing)
	ta.do(func() { ta.GraphingTab.OpenFile(path) })
	ta.waitFor("the log to load", func() bool {
		return !hasTextWithPrefix(ta.content(ui.TabGraphing), "Loading")
	})
}

// columnChecks returns the checks of the log's columns, by label.
func (ta *testApp) columnChecks() (checks []*widget.Check) {
	ta.do(func() {
		checks = findAll(ta.content(ui.TabGraphing), func(*widget.Check) bool { return true })
	})
	return checks
}

// chartedTitles returns the titles of the lanes the chart draws. The chart
// is the scrolled area that holds no checks, and its titles are the
// column labels among its text.
func (ta *testApp) chartedTitles() (titles []string) {
	labels := checkLabels(ta.columnChecks())
	ta.do(func() {
		for _, sc := range findAll(ta.content(ui.TabGraphing), func(*container.Scroll) bool { return true }) {
			if len(findAll(sc, func(*widget.Check) bool { return true })) == 0 {
				for _, s := range texts(sc) {
					if slices.Contains(labels, s) {
						titles = append(titles, s)
					}
				}
			}
		}
	})
	return titles
}

func checkLabels(checks []*widget.Check) (labels []string) {
	for _, c := range checks {
		labels = append(labels, c.Text)
	}
	return labels
}

func TestGraphingTabEnabledWhileDisconnected(t *testing.T) {
	ta := newTestApp(t)
	if !ta.tabEnabled(ui.TabGraphing) {
		t.Fatal("the Graphing tab is disabled while disconnected")
	}
	if !ta.buttonShown(ui.TabGraphing, "Open…") {
		t.Error("no Open button before a file is open")
	}
	if !ta.hasTextWithPrefix(ui.TabGraphing, "Open a log file") {
		t.Errorf("no hint about opening a file: %v", ta.texts(ui.TabGraphing))
	}
	ta.connect()
	ta.do(ta.OnDisconnect)
	if !ta.tabEnabled(ui.TabGraphing) {
		t.Error("the Graphing tab is disabled after disconnecting")
	}
}

func TestGraphingTabChartsFirstColumns(t *testing.T) {
	ta := newTestApp(t)
	path := writeLog(t, "run.csv", graphColumns, 40)
	ta.openGraph(path)

	checks := ta.columnChecks()
	if len(checks) != len(graphColumns) {
		t.Fatalf("got %d column checks, want %d: %v", len(checks), len(graphColumns), checkLabels(checks))
	}
	for i, c := range checks {
		if want := i < 4; c.Checked != want {
			t.Errorf("%q checked = %v, want %v", c.Text, c.Checked, want)
		}
	}
	if !slices.Contains(ta.texts(ui.TabGraphing), "run.csv") {
		t.Errorf("file name not shown: %v", ta.texts(ui.TabGraphing))
	}
	titles := ta.chartedTitles()
	for _, c := range checks[:4] {
		if !slices.Contains(titles, c.Text) {
			t.Errorf("%q is not charted; titles %v", c.Text, titles)
		}
	}
	if slices.Contains(titles, checks[4].Text) {
		t.Errorf("unchecked %q is charted", checks[4].Text)
	}

	// changing checks updates the chart, in column order
	ta.do(func() {
		test.Tap(checks[1])
		test.Tap(checks[5])
	})
	titles = ta.chartedTitles()
	want := []string{checks[0].Text, checks[2].Text, checks[3].Text, checks[5].Text}
	if got := titles; !slices.Equal(got, want) {
		t.Errorf("charted %v, want %v", got, want)
	}

	ta.do(func() {
		for _, c := range checks {
			if c.Checked {
				test.Tap(c)
			}
		}
	})
	if !slices.Contains(ta.texts(ui.TabGraphing), "Choose parameters to graph") {
		t.Errorf("no prompt to choose parameters: %v", ta.texts(ui.TabGraphing))
	}
}

func TestGraphingTabOpenReplacesFile(t *testing.T) {
	ta := newTestApp(t)
	ta.openGraph(writeLog(t, "first.csv", graphColumns, 10))
	ta.openGraph(writeLog(t, "second.csv", graphColumns[:2], 10))

	if got := len(ta.columnChecks()); got != 2 {
		t.Errorf("got %d column checks, want 2", got)
	}
	if texts := ta.texts(ui.TabGraphing); !slices.Contains(texts, "second.csv") || slices.Contains(texts, "first.csv") {
		t.Errorf("file name not replaced: %v", texts)
	}
}

func TestGraphingTabOpenError(t *testing.T) {
	ta := newTestApp(t)
	ta.openGraph(filepath.Join(t.TempDir(), "missing.csv"))
	if !ta.hasTextWithPrefix(ui.TabGraphing, "Couldn't open missing.csv: ") {
		t.Errorf("no error shown: %v", ta.texts(ui.TabGraphing))
	}
	if n := len(ta.columnChecks()); n != 0 {
		t.Errorf("%d column checks shown after an error", n)
	}

	bad := filepath.Join(t.TempDir(), "bad.csv")
	if err := os.WriteFile(bad, []byte("not a log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ta.openGraph(bad)
	if !ta.hasTextWithPrefix(ui.TabGraphing, "Couldn't open bad.csv: ") {
		t.Errorf("no error shown for a bad file: %v", ta.texts(ui.TabGraphing))
	}
}

func TestGraphingTabEmptyLog(t *testing.T) {
	ta := newTestApp(t)
	ta.openGraph(writeLog(t, "empty.csv", graphColumns, 0))
	if !slices.Contains(ta.texts(ui.TabGraphing), "This log has no rows") {
		t.Errorf("no empty log hint: %v", ta.texts(ui.TabGraphing))
	}
}

func TestGraphingTabFitsSmallWindow(t *testing.T) {
	ta := newTestApp(t)
	ta.resize(800, 480)
	ta.openGraph(writeLog(t, "run.csv", graphColumns, 40))
	var chart fyne.Size
	ta.do(func() {
		for _, c := range findAll(ta.content(ui.TabGraphing), func(*widget.Check) bool { return true }) {
			if c.Size().Width > 800/2 {
				t.Errorf("check %q is %v wide", c.Text, c.Size().Width)
			}
		}
		chart = ta.content(ui.TabGraphing).Size()
	})
	if chart.Width == 0 {
		t.Error("tab has no size")
	}
}
