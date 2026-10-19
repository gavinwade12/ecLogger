package main

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/gavinwade12/ecLogger/internal/config"
	"github.com/gavinwade12/ecLogger/protocols/ssm2"
)

// mainThread stands in for Fyne's main goroutine in tests. The test driver
// runs fyne.Do calls inline on the calling goroutine, which races with the
// test; mainThread instead runs them one at a time on a worker, and tests
// touch widgets through run so they never overlap.
type mainThread struct {
	mu    sync.Mutex
	queue chan func()
}

func newMainThread(t *testing.T) *mainThread {
	m := &mainThread{queue: make(chan func(), 1024)}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for f := range m.queue {
			m.run(f)
		}
	}()
	t.Cleanup(func() {
		close(m.queue)
		<-done
	})
	return m
}

func (m *mainThread) run(f func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f()
}

type mainThreadDriver struct {
	fyne.Driver
	m *mainThread
}

func (d mainThreadDriver) DoFromGoroutine(f func(), wait bool) {
	if !wait {
		d.m.queue <- f
		return
	}
	done := make(chan struct{})
	d.m.queue <- func() {
		defer close(done)
		f()
	}
	<-done
}

type mainThreadApp struct {
	fyne.App
	driver fyne.Driver
}

func (a mainThreadApp) Driver() fyne.Driver { return a.driver }

type testHarness struct {
	*App
	t    *testing.T
	main *mainThread
}

// newTestApp returns an App on a test fyne app whose connections are fake.
func newTestApp(t *testing.T) *testHarness {
	t.Helper()
	cfg, err := config.Load(filepath.Join(t.TempDir(), "config"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.SelectedPort = "fake"
	cfg.LogDirectory = t.TempDir()

	m := newMainThread(t)
	base := test.NewTempApp(t)
	fa := mainThreadApp{App: base, driver: mainThreadDriver{Driver: base.Driver(), m: m}}
	fyne.SetCurrentApp(fa)

	h := &testHarness{t: t, main: m}
	open := func(string) (ssm2.Connection, error) {
		return ssm2.NewFakeConnection(time.Millisecond), nil
	}
	listPorts := func() ([]string, error) { return []string{"fake"}, nil }
	m.run(func() { h.App = NewApp(fa, cfg, open, listPorts) })

	t.Cleanup(func() {
		m.run(func() {
			h.LoggingTab.DisableLogging()
			h.ConnectionTab.stopSerialPortQuery()
		})
	})
	return h
}

// do runs f on the main thread.
func (h *testHarness) do(f func()) { h.main.run(f) }

// waitFor waits for cond, evaluated on the main thread, to be true.
func (h *testHarness) waitFor(what string, cond func() bool) {
	h.t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		var ok bool
		h.do(func() { ok = cond() })
		if ok {
			return
		}
		if time.Now().After(deadline) {
			h.t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

func (h *testHarness) waitForState(want string) {
	h.t.Helper()
	h.waitFor("connection state "+want, func() bool {
		got, _ := h.ConnectionTab.connectionState.Get()
		return got == want
	})
}

// connect taps Connect and waits for the connection to be initialized.
func (h *testHarness) connect() {
	h.t.Helper()
	h.do(func() { test.Tap(h.ConnectionTab.connectBtn) })
	h.waitForState("Connected")
}

func (h *testHarness) tabEnabled(tab TabType) (enabled bool) {
	h.do(func() { enabled = !h.tabItems.Items[tab].Disabled() })
	return enabled
}

var connectionTabs = []TabType{TabParameters, TabLogging, TabDTCs}

func TestConnectEnablesTabs(t *testing.T) {
	h := newTestApp(t)
	for _, tab := range connectionTabs {
		if h.tabEnabled(tab) {
			t.Errorf("tab %d enabled before connecting", tab)
		}
	}

	h.connect()
	for _, tab := range connectionTabs {
		if !h.tabEnabled(tab) {
			t.Errorf("tab %d disabled after connecting", tab)
		}
	}
	h.do(func() {
		if !h.ConnectionTab.connectBtn.Disabled() || !h.ConnectionTab.disconnectBtn.Visible() {
			t.Error("expected Connect disabled and Disconnect visible")
		}
	})
}
