package main

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
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

func TestInitRetriesTimeoutsWithDelay(t *testing.T) {
	h := newTestApp(t)
	h.ConnectionTab.initRetryDelay = 20 * time.Millisecond
	conn := &failingInitConnection{err: ssm2.ErrReadTimeout}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err := h.ConnectionTab.initSSM2Connection(ctx, conn)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
	if n := conn.inits.Load(); n < 2 || n > 10 {
		t.Errorf("got %d init attempts in 100ms with a 20ms delay", n)
	}
	if !conn.closed.Load() {
		t.Error("expected the connection to be closed")
	}
}

func TestInitGivesUpOnOtherErrors(t *testing.T) {
	h := newTestApp(t)
	conn := &failingInitConnection{err: errors.New("port unplugged")}

	err := h.ConnectionTab.initSSM2Connection(context.Background(), conn)
	if err == nil {
		t.Fatal("expected an error")
	}
	if n := conn.inits.Load(); n != 1 {
		t.Errorf("got %d init attempts, want 1", n)
	}
	if !conn.closed.Load() {
		t.Error("expected the connection to be closed")
	}
}

func TestConnectShowsErrors(t *testing.T) {
	h := newTestApp(t)
	h.do(func() {
		h.openConnection = func(string) (ssm2.Connection, error) {
			return nil, errors.New("port busy")
		}
		test.Tap(h.ConnectionTab.connectBtn)
	})
	h.waitForState("Disconnected: port busy")

	// the next attempt doesn't keep showing the old error
	h.do(func() {
		h.openConnection = func(string) (ssm2.Connection, error) {
			return &failingInitConnection{err: ssm2.ErrReadTimeout}, nil
		}
		test.Tap(h.ConnectionTab.connectBtn)
	})
	h.waitForState("Initializing")

	// and canceling isn't shown as an error
	h.do(func() { test.Tap(h.ConnectionTab.cancelBtn) })
	h.waitForState("Disconnected")
}
