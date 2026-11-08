package logging_test

import (
	"context"
	"testing"
	"time"

	"github.com/gavinwade12/ecLogger/internal/logging"
	"github.com/gavinwade12/ecLogger/protocols/ssm2"
	"github.com/gavinwade12/ecLogger/units"
)

// runSession runs a session until the test ends and returns the values it emits.
func runSession(t *testing.T, s *logging.Session) <-chan map[string]ssm2.ParameterValue {
	t.Helper()
	ch := make(chan map[string]ssm2.ParameterValue, 100)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.Run(ctx, func(v map[string]ssm2.ParameterValue) {
			select {
			case ch <- v:
			default:
			}
		})
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	return ch
}

func TestSessionConvertsValues(t *testing.T) {
	unitFor := func(id string) (units.Unit, bool) {
		if id == "P9" {
			return units.MPH, true
		}
		return "", false
	}
	s := logging.NewSession(ssm2.NewFakeConnection(time.Millisecond),
		[]ssm2.Parameter{ssm2.Parameters["P9"]}, nil, unitFor, nil)
	values := runSession(t, s)

	v := receive(t, values)
	if v["P9"].Unit != units.MPH {
		t.Fatalf("P9 unit = %s, want mph", v["P9"].Unit)
	}
}

func TestSessionRetriesFailedReads(t *testing.T) {
	conn := &flakyConnection{Connection: ssm2.NewFakeConnection(time.Millisecond), failRequests: 2}
	s := logging.NewSession(conn, []ssm2.Parameter{ssm2.Parameters["P8"]}, nil, noUnits, nil,
		logging.WithRetryDelay(time.Millisecond))
	values := runSession(t, s)

	receive(t, values)
	if got := conn.requests.Load(); got != 3 {
		t.Fatalf("got %d requests, want 3", got)
	}
}

func TestSessionStopsWhileRetrying(t *testing.T) {
	conn := &flakyConnection{Connection: ssm2.NewFakeConnection(time.Millisecond), failRequests: 1 << 30}
	s := logging.NewSession(conn, []ssm2.Parameter{ssm2.Parameters["P8"]}, nil, noUnits, nil,
		logging.WithRetryDelay(time.Hour))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.Run(ctx, func(map[string]ssm2.ParameterValue) {})
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run() hung while the reads were failing to start")
	}
}

func TestSessionHidesDerivedDependencies(t *testing.T) {
	// P242 needs P200 (derived), which needs P8 and P12, plus P11 and P7
	s := logging.NewSession(ssm2.NewFakeConnection(time.Millisecond), nil,
		[]ssm2.DerivedParameter{ssm2.DerivedParameters["P242"]}, noUnits, nil)
	values := runSession(t, s)

	v := receive(t, values)
	if _, ok := v["P242"]; !ok {
		t.Error("missing P242; its dependencies weren't read")
	}
	for _, id := range []string{"P200", "P7", "P8", "P11", "P12"} {
		if _, ok := v[id]; ok {
			t.Errorf("unrequested dependency %s was emitted", id)
		}
		if _, ok := s.Stats(id); ok {
			t.Errorf("unrequested dependency %s has stats", id)
		}
	}
}

func TestSessionTracksStats(t *testing.T) {
	s := logging.NewSession(ssm2.NewFakeConnection(time.Millisecond),
		[]ssm2.Parameter{ssm2.Parameters["P8"]}, nil, noUnits, nil)
	if _, ok := s.Stats("P8"); ok {
		t.Fatal("expected no stats before any values")
	}
	values := runSession(t, s)

	first := receive(t, values)["P8"].Value
	st, ok := s.Stats("P8")
	if !ok || st.Count < 1 || st.Min > first || st.Max < first {
		t.Fatalf("Stats(P8) = %+v, %v; want it to include %v", st, ok, first)
	}
}
