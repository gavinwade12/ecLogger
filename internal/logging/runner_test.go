package logging_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gavinwade12/ecLogger/internal/logging"
	"github.com/gavinwade12/ecLogger/protocols/ssm2"
	"github.com/gavinwade12/ecLogger/units"
)

// flakyConnection fails the first failRequests read address requests, then
// behaves like a fake connection.
type flakyConnection struct {
	ssm2.Connection
	failRequests int32
	requests     atomic.Int32
}

func (c *flakyConnection) SendReadAddressesRequest(ctx context.Context, addresses [][3]byte, continous bool) (ssm2.Packet, error) {
	if c.requests.Add(1) <= c.failRequests {
		return nil, errors.New("not connected")
	}
	return c.Connection.SendReadAddressesRequest(ctx, addresses, continous)
}

func noUnits(string) (units.Unit, bool) { return "", false }

// receive returns the next set of values from ch or fails the test.
func receive(t *testing.T, ch <-chan map[string]ssm2.ParameterValue) map[string]ssm2.ParameterValue {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for values")
		return nil
	}
}

func subscribe(r *logging.Runner) <-chan map[string]ssm2.ParameterValue {
	ch := make(chan map[string]ssm2.ParameterValue, 100)
	r.Subscribe("test", func(v map[string]ssm2.ParameterValue) {
		select {
		case ch <- v:
		default:
		}
	})
	return ch
}

func TestRunnerStopsWhileRetrying(t *testing.T) {
	conn := &flakyConnection{Connection: ssm2.NewFakeConnection(time.Millisecond), failRequests: 1 << 30}
	r := logging.NewRunner(noUnits, nil, logging.WithRetryDelay(time.Hour))

	r.Start(conn, []ssm2.Parameter{ssm2.Parameters["P8"]}, nil)
	if !r.Running() {
		t.Fatal("expected the runner to be running")
	}

	stopped := make(chan struct{})
	go func() {
		r.Stop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() hung while the session was failing to start")
	}
	if r.Running() {
		t.Fatal("expected the runner to be stopped")
	}
}

func TestRunnerUnsubscribe(t *testing.T) {
	r := logging.NewRunner(noUnits, nil)
	var mu sync.Mutex
	calls := 0
	r.Subscribe("count", func(map[string]ssm2.ParameterValue) {
		mu.Lock()
		calls++
		mu.Unlock()
	})
	values := subscribe(r)

	r.Start(ssm2.NewFakeConnection(time.Millisecond), []ssm2.Parameter{ssm2.Parameters["P8"]}, nil)
	defer r.Stop()
	receive(t, values)

	r.Unsubscribe("count")
	mu.Lock()
	before := calls
	mu.Unlock()
	receive(t, values)
	receive(t, values)

	mu.Lock()
	defer mu.Unlock()
	if calls != before {
		t.Fatalf("unsubscribed func was called %d more times", calls-before)
	}
}

func TestRunnerKeepsSubscribersAcrossSessions(t *testing.T) {
	r := logging.NewRunner(noUnits, nil)
	values := subscribe(r)
	params := []ssm2.Parameter{ssm2.Parameters["P8"]}

	r.Start(ssm2.NewFakeConnection(time.Millisecond), params, nil)
	defer r.Stop()
	receive(t, values)

	r.Start(ssm2.NewFakeConnection(time.Millisecond), params, nil)
	receive(t, values)
}

func TestRunnerStats(t *testing.T) {
	r := logging.NewRunner(noUnits, nil)
	if _, ok := r.Stats("P8"); ok {
		t.Fatal("expected no stats before a session is started")
	}
	values := subscribe(r)

	r.Start(ssm2.NewFakeConnection(time.Millisecond), []ssm2.Parameter{ssm2.Parameters["P8"]}, nil)
	defer r.Stop()
	receive(t, values)

	if st, ok := r.Stats("P8"); !ok || st.Count < 1 {
		t.Fatalf("Stats(P8) = %+v, %v; want at least one value", st, ok)
	}

	// stats are kept after stopping, and reset by the next session
	r.Stop()
	if _, ok := r.Stats("P8"); !ok {
		t.Fatal("expected stats to remain after Stop")
	}
	r.Start(ssm2.NewFakeConnection(time.Hour), []ssm2.Parameter{ssm2.Parameters["P8"]}, nil)
	if _, ok := r.Stats("P8"); ok {
		t.Fatal("expected a new session to start with no stats")
	}
}
