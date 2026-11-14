package main_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	ui "github.com/gavinwade12/ecLogger/cmd/logger-ui"
	"github.com/gavinwade12/ecLogger/protocols/ssm2"
)

// failingInitConnection fails every InitECU call with err.
type failingInitConnection struct {
	ssm2.Connection
	err    error
	inits  atomic.Int32
	closed atomic.Bool
}

func (c *failingInitConnection) InitECU(ctx context.Context) (*ssm2.ECU, error) {
	c.inits.Add(1)
	return nil, c.err
}

func (c *failingInitConnection) Close() error {
	c.closed.Store(true)
	return nil
}

func opens(conn ssm2.Connection) ui.OpenFunc {
	return func(string) (ssm2.Connection, error) { return conn, nil }
}

func TestInitRetriesTimeoutsWithDelay(t *testing.T) {
	ta := newTestApp(t)
	conn := &failingInitConnection{err: ssm2.ErrReadTimeout}
	ta.setOpen(opens(conn))

	ta.tap(ui.TabConnection, "Connect")
	ta.waitForStatus("Initializing")
	ta.waitFor("the first init", func() bool { return conn.inits.Load() >= 1 })

	// timeouts, like the ignition being off, are retried, but not in a
	// tight loop
	time.Sleep(200 * time.Millisecond)
	if n := conn.inits.Load(); n != 1 {
		t.Errorf("got %d init attempts in 200ms, want a delay before the retry", n)
	}
	ta.waitFor("a retry", func() bool { return conn.inits.Load() >= 2 })

	ta.tap(ui.TabConnection, "Cancel")
	ta.waitForStatus("Disconnected")
	ta.waitFor("the connection to close", conn.closed.Load)
}

func TestInitGivesUpOnOtherErrors(t *testing.T) {
	ta := newTestApp(t)
	conn := &failingInitConnection{err: errors.New("port unplugged")}
	ta.setOpen(opens(conn))

	ta.tap(ui.TabConnection, "Connect")
	ta.waitForStatus("Disconnected: initializing ECU: port unplugged")
	if n := conn.inits.Load(); n != 1 {
		t.Errorf("got %d init attempts, want 1", n)
	}
	if !conn.closed.Load() {
		t.Error("expected the connection to be closed")
	}
}

func TestConnectShowsErrors(t *testing.T) {
	ta := newTestApp(t)
	ta.setOpen(func(string) (ssm2.Connection, error) { return nil, errors.New("port busy") })
	ta.tap(ui.TabConnection, "Connect")
	ta.waitForStatus("Disconnected: port busy")
	if !ta.buttonShown(ui.TabConnection, "Connect") {
		t.Error("expected to be able to try again")
	}

	// the next attempt doesn't keep showing the old error
	ta.setOpen(opens(&failingInitConnection{err: ssm2.ErrReadTimeout}))
	ta.tap(ui.TabConnection, "Connect")
	ta.waitForStatus("Initializing")

	// and canceling isn't shown as an error
	ta.tap(ui.TabConnection, "Cancel")
	ta.waitForStatus("Disconnected")
}

// TestConnectionStatusIsColoredByState checks the dot beside the status, and
// that a failure to connect is shown as an error.
func TestConnectionStatusIsColoredByState(t *testing.T) {
	ta := newTestApp(t)
	// dotColor must be called on the main thread
	dotColor := func() any {
		return findAll[*canvas.Circle](ta.content(ui.TabConnection), nil)[0].FillColor
	}
	check := func(state string, dot, text fyne.ThemeColorName) {
		t.Helper()
		ta.waitForStatus(state)
		ta.waitFor("the status dot to be "+string(dot), func() bool { return dotColor() == themeColor(dot) })
		ta.do(func() {
			if got, _ := textColor(ta.content(ui.TabConnection), state); got != text {
				t.Errorf("%q is %s, want %s", state, got, text)
			}
		})
	}
	check("Disconnected", theme.ColorNamePlaceHolder, theme.ColorNameForeground)

	proceed := make(chan struct{})
	ta.setOpen(func(string) (ssm2.Connection, error) {
		<-proceed
		return &failingInitConnection{err: ssm2.ErrReadTimeout}, nil
	})
	ta.tap(ui.TabConnection, "Connect")
	check("Connecting", theme.ColorNameWarning, theme.ColorNameForeground)
	close(proceed)
	check("Initializing", theme.ColorNameWarning, theme.ColorNameForeground)
	ta.tap(ui.TabConnection, "Cancel")
	ta.waitForStatus("Disconnected")

	ta.setOpen(func(string) (ssm2.Connection, error) { return nil, errors.New("a port is required") })
	ta.tap(ui.TabConnection, "Connect")
	check("Disconnected: a port is required", theme.ColorNameError, theme.ColorNameError)

	ta.setOpen(opens(ta.ecu))
	ta.connect()
	check("Connected", theme.ColorNameSuccess, theme.ColorNameForeground)
}
