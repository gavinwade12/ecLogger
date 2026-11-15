package main_test

// Shared helpers for driving the app the way a user does: through the
// widgets it puts on screen. The tests only use what package main exports,
// so they find buttons, checks and text by walking the widget tree rather
// than reaching into the app's fields.

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	ui "github.com/gavinwade12/ecLogger/cmd/logger-ui"
	"github.com/gavinwade12/ecLogger/internal/config"
	"github.com/gavinwade12/ecLogger/protocols/ssm2"
)

const waitTimeout = 3 * time.Second

// mainThread stands in for Fyne's main goroutine in tests. The test driver
// runs fyne.Do calls inline on the calling goroutine, which races with the
// test; mainThread instead runs them one at a time on a worker, and tests
// touch widgets through run so they never overlap.
type mainThread struct {
	mu sync.Mutex
	// queue holds the calls waiting to run. Like Fyne's, it never fills
	// up: while a test holds the main thread, say to stop a session whose
	// goroutine is posting updates, those must not block.
	queueMu sync.Mutex
	queue   []func()
	wake    chan struct{}
	done    chan struct{}
	// exited is closed when the worker is done
	exited chan struct{}
}

func newMainThread() *mainThread {
	m := &mainThread{wake: make(chan struct{}, 1), done: make(chan struct{}), exited: make(chan struct{})}
	go func() {
		defer close(m.exited)
		for {
			m.queueMu.Lock()
			var f func()
			if len(m.queue) > 0 {
				f, m.queue = m.queue[0], m.queue[1:]
			}
			m.queueMu.Unlock()
			if f != nil {
				m.run(f)
				continue
			}
			select {
			case <-m.wake:
			case <-m.done:
				return
			}
		}
	}()
	return m
}

// post queues f to run on the main thread.
func (m *mainThread) post(f func()) {
	select {
	case <-m.done:
		return
	default:
	}
	m.queueMu.Lock()
	m.queue = append(m.queue, f)
	m.queueMu.Unlock()
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

func (m *mainThread) run(f func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f()
}

// stop ends the main thread. Goroutines the app leaves running can still
// call fyne.Do afterwards; those calls are dropped.
func (m *mainThread) stop() {
	close(m.done)
	<-m.exited
}

type mainThreadDriver struct {
	fyne.Driver
	m *mainThread
}

func (d mainThreadDriver) DoFromGoroutine(f func(), wait bool) {
	if !wait {
		d.m.post(f)
		return
	}
	done := make(chan struct{})
	d.m.post(func() { defer close(done); f() })
	select {
	case <-done:
	case <-d.m.done:
	}
}

type mainThreadApp struct {
	fyne.App
	driver fyne.Driver
}

func (a mainThreadApp) Driver() fyne.Driver { return a.driver }

// testApp is the app under test, shown in a window on a test fyne app, with
// an ECU connection that's fake.
type testApp struct {
	*ui.App
	t      *testing.T
	main   *mainThread
	cfg    *config.Config
	ecu    *testConn
	window fyne.Window

	openMu sync.Mutex
	open   ui.OpenFunc
}

// newTestApp returns an app whose connections open ta.ecu.
func newTestApp(t *testing.T) *testApp {
	t.Helper()
	cfg, err := config.Load(filepath.Join(t.TempDir(), "config"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.SelectedPort = "fake"
	cfg.LogDirectory = t.TempDir()

	m := newMainThread()
	base := test.NewTempApp(t)
	// stopped before the test app is reset, since the app can't draw
	// without its theme
	t.Cleanup(m.stop)
	fa := mainThreadApp{App: base, driver: mainThreadDriver{Driver: base.Driver(), m: m}}
	fyne.SetCurrentApp(fa)

	ta := &testApp{t: t, main: m, cfg: cfg, ecu: newTestConn()}
	ta.open = func(string) (ssm2.Connection, error) { return ta.ecu, nil }
	open := func(port string) (ssm2.Connection, error) {
		ta.openMu.Lock()
		f := ta.open
		ta.openMu.Unlock()
		return f(port)
	}
	// the Connection tab polls for ports every 30 seconds and never stops,
	// and once this test is over its fyne.Do calls would land on another
	// test's app, so stop giving it anything to show
	var over atomic.Bool
	t.Cleanup(func() { over.Store(true) })
	listPorts := func() ([]string, error) {
		if over.Load() {
			return nil, errors.New("test is over")
		}
		return []string{"fake"}, nil
	}
	m.run(func() {
		ta.App = ui.NewApp(fa, cfg, open, listPorts)
		ta.window = test.NewWindow(ta.Content())
		ta.window.Resize(fyne.NewSize(900, 600))
	})

	t.Cleanup(func() {
		ta.ecu.releaseAll()
		ta.do(func() {
			// give up on a connection that's still being made
			if b := button(ta.Content(), "Cancel"); b != nil {
				test.Tap(b)
			}
			ta.OnDisconnect()
		})
	})
	return ta
}

// setOpen replaces how connections are opened.
func (ta *testApp) setOpen(open ui.OpenFunc) {
	ta.openMu.Lock()
	defer ta.openMu.Unlock()
	ta.open = open
}

// do runs f on the main thread.
func (ta *testApp) do(f func()) { ta.main.run(f) }

// resize resizes the window.
func (ta *testApp) resize(width, height float32) {
	ta.do(func() { ta.window.Resize(fyne.NewSize(width, height)) })
}

// waitFor waits for cond, evaluated on the main thread, to be true.
func (ta *testApp) waitFor(what string, cond func() bool) {
	ta.t.Helper()
	deadline := time.Now().Add(waitTimeout)
	for {
		var ok bool
		ta.do(func() { ok = cond() })
		if ok {
			return
		}
		if time.Now().After(deadline) {
			ta.t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

func (ta *testApp) tabs() *container.AppTabs { return ta.Content().(*container.AppTabs) }

// content returns what a tab shows, whether or not the tab is selected.
func (ta *testApp) content(tab ui.TabType) fyne.CanvasObject {
	return ta.tabs().Items[tab].Content
}

// show selects a tab, like clicking it, and returns what it shows.
func (ta *testApp) show(tab ui.TabType) fyne.CanvasObject {
	ta.do(func() { ta.SelectTab(tab) })
	return ta.content(tab)
}

// texts returns the text a tab shows, in the order it's drawn.
func (ta *testApp) texts(tab ui.TabType) (s []string) {
	ta.do(func() { s = texts(ta.content(tab)) })
	return s
}

// hasTextWithPrefix returns whether a tab shows text starting with prefix.
func (ta *testApp) hasTextWithPrefix(tab ui.TabType, prefix string) (has bool) {
	ta.do(func() { has = hasTextWithPrefix(ta.content(tab), prefix) })
	return has
}

func (ta *testApp) tabEnabled(tab ui.TabType) (enabled bool) {
	ta.do(func() { enabled = !ta.tabs().Items[tab].Disabled() })
	return enabled
}

// tap taps the shown button with the given text, like clicking it.
func (ta *testApp) tap(tab ui.TabType, text string) {
	ta.t.Helper()
	ta.do(func() {
		b := button(ta.content(tab), text)
		if b == nil {
			ta.t.Fatalf("no %q button is shown", text)
		}
		test.Tap(b)
	})
}

// buttonShown returns whether a button with the given text is shown.
func (ta *testApp) buttonShown(tab ui.TabType, text string) (shown bool) {
	ta.do(func() { shown = button(ta.content(tab), text) != nil })
	return shown
}

// waitForText waits for a tab to show text, or to stop showing it.
func (ta *testApp) waitForText(tab ui.TabType, text string, shown bool) {
	ta.t.Helper()
	what := "the " + tabName(tab) + " tab to show " + text
	if !shown {
		what = "the " + tabName(tab) + " tab to stop showing " + text
	}
	ta.waitFor(what, func() bool {
		return slices.Contains(texts(ta.content(tab)), text) == shown
	})
}

// waitForStatus waits for the connection status to be state.
func (ta *testApp) waitForStatus(state string) {
	ta.t.Helper()
	ta.waitForText(ui.TabConnection, state, true)
}

func tabName(tab ui.TabType) string {
	return [...]string{"Connection", "Parameters", "Logging", "Graphing", "Log Files", "DTCs", "Settings"}[tab]
}

// connect taps Connect and waits for the connection to be initialized.
func (ta *testApp) connect() {
	ta.t.Helper()
	ta.tap(ui.TabConnection, "Connect")
	ta.waitForStatus("Connected")
}

// paramRow is the widgets of a parameter on the Parameters tab.
type paramRow struct {
	name             string
	fileLog, liveLog *widget.Check
	unit             *widget.Select
	// cells holds the row's columns, and row is what the list lays out.
	cells, row fyne.CanvasObject
}

// paramRow finds a parameter's row on the Parameters tab by its name.
func (ta *testApp) paramRow(name string) paramRow {
	ta.t.Helper()
	var r paramRow
	ta.do(func() {
		cells, ancestors := deepest(ta.content(ui.TabParameters), nil, func(o fyne.CanvasObject) bool {
			return slices.Contains(texts(o), name) &&
				len(findAll[*widget.Check](o, nil)) >= 2 && len(findAll[*widget.Select](o, nil)) >= 1
		})
		if cells == nil {
			ta.t.Fatalf("no parameter named %q is shown", name)
		}
		checks := findAll[*widget.Check](cells, nil)
		r = paramRow{
			name: name, fileLog: checks[0], liveLog: checks[1],
			unit:  findAll[*widget.Select](cells, nil)[0],
			cells: cells, row: ancestors[len(ancestors)-1],
		}
	})
	return r
}

// setLiveLog checks or unchecks a parameter's Live Log.
func (ta *testApp) setLiveLog(name string, on bool) {
	ta.t.Helper()
	ta.show(ui.TabParameters)
	row := ta.paramRow(name)
	ta.do(func() {
		if row.liveLog.Checked != on {
			test.Tap(row.liveLog)
		}
	})
}

// setFileLog checks or unchecks a parameter's Log To File.
func (ta *testApp) setFileLog(name string, on bool) {
	ta.t.Helper()
	ta.show(ui.TabParameters)
	row := ta.paramRow(name)
	ta.do(func() {
		if row.fileLog.Checked != on {
			test.Tap(row.fileLog)
		}
	})
}

// assertLiveReadsStopped checks that nothing is reading live values from the
// ECU. A read that was already under way when logging stopped can still
// land, so it gives those a moment first.
func (ta *testApp) assertLiveReadsStopped() {
	ta.t.Helper()
	time.Sleep(30 * time.Millisecond)
	before := ta.ecu.liveReads.Load()
	time.Sleep(30 * time.Millisecond)
	if after := ta.ecu.liveReads.Load(); after != before {
		ta.t.Errorf("%d live reads while logging should be stopped", after-before)
	}
}

// waitForLiveReads waits for something to read live values from the ECU.
func (ta *testApp) waitForLiveReads() {
	ta.t.Helper()
	start := ta.ecu.liveReads.Load()
	ta.waitFor("live reads", func() bool { return ta.ecu.liveReads.Load() > start })
}

// testConn is the ECU connection the tests connect to. It's a fake
// connection with some control over what it answers and when.
type testConn struct {
	ssm2.Connection

	// liveReads counts the reads of live values.
	liveReads atomic.Int32

	mu                  sync.Mutex
	setDTCs, storedDTCs []ssm2.DTC
	dtcErr              error
	dtcGate, liveGate   chan struct{}
	liveByte            int // -1 to answer with the fake connection's values
	liveAddresses       int
	dtcAddresses        map[[3]byte]bool
}

func newTestConn() *testConn {
	c := &testConn{
		Connection:   ssm2.NewFakeConnection(time.Millisecond),
		liveByte:     -1,
		dtcAddresses: map[[3]byte]bool{},
	}
	for _, dtc := range ssm2.DTCs {
		c.dtcAddresses[dtc.Address.Address] = true
		c.dtcAddresses[dtc.TempAddress.Address] = true
	}
	return c
}

// stubDTCs makes the ECU report the given DTCs, or fail with err.
func (c *testConn) stubDTCs(set, stored []ssm2.DTC, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.setDTCs, c.storedDTCs, c.dtcErr = set, stored, err
}

// holdDTCs makes reading DTCs wait for releaseDTCs.
func (c *testConn) holdDTCs() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dtcGate = make(chan struct{})
}

func (c *testConn) releaseDTCs() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.dtcGate != nil {
		close(c.dtcGate)
		c.dtcGate = nil
	}
}

// holdLive makes reading live values wait for releaseLive.
func (c *testConn) holdLive() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.liveGate = make(chan struct{})
}

func (c *testConn) releaseLive() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.liveGate != nil {
		close(c.liveGate)
		c.liveGate = nil
	}
}

func (c *testConn) releaseAll() {
	c.releaseDTCs()
	c.releaseLive()
}

// answerLiveWith makes every byte of a live read b.
func (c *testConn) answerLiveWith(b byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.liveByte = int(b)
}

func (c *testConn) wait(ctx context.Context, gate chan struct{}) error {
	if gate == nil {
		return nil
	}
	select {
	case <-gate:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *testConn) SendReadAddressesRequest(ctx context.Context, addresses [][3]byte, continuous bool) (ssm2.Packet, error) {
	c.mu.Lock()
	isDTC := slices.ContainsFunc(addresses, func(a [3]byte) bool { return c.dtcAddresses[a] })
	dtcGate, liveGate := c.dtcGate, c.liveGate
	setDTCs, storedDTCs, dtcErr := c.setDTCs, c.storedDTCs, c.dtcErr
	c.liveAddresses = len(addresses)
	c.mu.Unlock()

	if isDTC {
		if err := c.wait(ctx, dtcGate); err != nil {
			return nil, err
		}
		if dtcErr != nil {
			return nil, dtcErr
		}
		data := make([]byte, len(addresses))
		for i, addr := range addresses {
			for _, dtc := range setDTCs {
				if dtc.TempAddress.Address == addr {
					data[i] |= 1 << dtc.TempAddress.Bit
				}
			}
			for _, dtc := range storedDTCs {
				if dtc.Address.Address == addr {
					data[i] |= 1 << dtc.Address.Bit
				}
			}
		}
		return ssm2.NewPacket(ssm2.DeviceEngine, ssm2.DeviceDiagnosticTool, ssm2.CommandReadAddressesResponse, data), nil
	}

	c.liveReads.Add(1)
	if err := c.wait(ctx, liveGate); err != nil {
		return nil, err
	}
	// the fake connection answers the first read too
	p, err := c.Connection.SendReadAddressesRequest(ctx, addresses, continuous)
	if err != nil {
		return nil, err
	}
	return c.liveAnswer(p), nil
}

func (c *testConn) NextPacket(ctx context.Context) (ssm2.Packet, error) {
	c.mu.Lock()
	liveGate := c.liveGate
	c.mu.Unlock()

	c.liveReads.Add(1)
	if err := c.wait(ctx, liveGate); err != nil {
		return nil, err
	}
	p, err := c.Connection.NextPacket(ctx)
	if err != nil {
		return nil, err
	}
	return c.liveAnswer(p), nil
}

// liveAnswer replaces the fake connection's values with the ones the test
// asked for.
func (c *testConn) liveAnswer(p ssm2.Packet) ssm2.Packet {
	c.mu.Lock()
	b, n := c.liveByte, c.liveAddresses
	c.mu.Unlock()
	if b < 0 || len(p) <= ssm2.PacketHeaderSize+1 {
		return p
	}
	data := make([]byte, n)
	for i := range data {
		data[i] = byte(b)
	}
	return ssm2.NewPacket(ssm2.DeviceDiagnosticTool, ssm2.DeviceEngine, ssm2.CommandReadAddressesResponse, data)
}

// childrenOf returns the objects drawn inside o.
func childrenOf(o fyne.CanvasObject) []fyne.CanvasObject {
	var children []fyne.CanvasObject
	switch v := o.(type) {
	case *fyne.Container:
		children = v.Objects
	case *container.AppTabs:
		for _, item := range v.Items {
			children = append(children, item.Content)
		}
	case fyne.Widget:
		children = test.WidgetRenderer(v).Objects()
	}
	var shown []fyne.CanvasObject
	for _, c := range children {
		if c.Visible() {
			shown = append(shown, c)
		}
	}
	return shown
}

// walk calls visit for root and everything shown inside it, with the objects
// above each one, outermost first.
func walk(root fyne.CanvasObject, visit func(o fyne.CanvasObject, ancestors []fyne.CanvasObject)) {
	var rec func(o fyne.CanvasObject, ancestors []fyne.CanvasObject)
	rec = func(o fyne.CanvasObject, ancestors []fyne.CanvasObject) {
		visit(o, ancestors)
		ancestors = append(ancestors[:len(ancestors):len(ancestors)], o)
		for _, c := range childrenOf(o) {
			rec(c, ancestors)
		}
	}
	rec(root, nil)
}

// findAll returns the objects of type T shown in root that keep accepts.
func findAll[T any](root fyne.CanvasObject, keep func(T) bool) []T {
	var found []T
	walk(root, func(o fyne.CanvasObject, _ []fyne.CanvasObject) {
		if v, ok := o.(T); ok && (keep == nil || keep(v)) {
			found = append(found, v)
		}
	})
	return found
}

// deepest returns the innermost object in root that satisfies pred, and the
// objects above it. Objects inside one that doesn't are never checked, so
// pred should be true of anything that holds what it's looking for.
func deepest(o fyne.CanvasObject, ancestors []fyne.CanvasObject, pred func(fyne.CanvasObject) bool) (fyne.CanvasObject, []fyne.CanvasObject) {
	if !pred(o) {
		return nil, nil
	}
	inner := append(ancestors[:len(ancestors):len(ancestors)], o)
	for _, c := range childrenOf(o) {
		if d, a := deepest(c, inner, pred); d != nil {
			return d, a
		}
	}
	return o, ancestors
}

// button returns the shown button with the given text, or nil.
func button(root fyne.CanvasObject, text string) *widget.Button {
	found := findAll(root, func(b *widget.Button) bool { return b.Text == text })
	if len(found) == 0 {
		return nil
	}
	return found[0]
}

// texts returns the text shown in root, a line at a time, in the order it's
// drawn.
func texts(root fyne.CanvasObject) []string {
	var s []string
	for _, t := range findAll[*canvas.Text](root, nil) {
		if t.Text != "" {
			s = append(s, t.Text)
		}
	}
	return s
}

func hasTextWithPrefix(root fyne.CanvasObject, prefix string) bool {
	return slices.ContainsFunc(texts(root), func(s string) bool { return strings.HasPrefix(s, prefix) })
}

// textColor returns the color of the shown text that has the given prefix.
func textColor(root fyne.CanvasObject, prefix string) (fyne.ThemeColorName, bool) {
	for _, t := range findAll[*canvas.Text](root, nil) {
		if !strings.HasPrefix(t.Text, prefix) {
			continue
		}
		for _, name := range []fyne.ThemeColorName{
			theme.ColorNameForeground, theme.ColorNameError, theme.ColorNameWarning,
			theme.ColorNameSuccess, theme.ColorNamePlaceHolder,
		} {
			if t.Color == themeColor(name) {
				return name, true
			}
		}
	}
	return "", false
}

func themeColor(name fyne.ThemeColorName) any {
	return fyne.CurrentApp().Settings().Theme().Color(name, fyne.CurrentApp().Settings().ThemeVariant())
}
