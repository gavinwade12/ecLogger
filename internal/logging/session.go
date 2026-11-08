package logging

import (
	"context"
	"sync"
	"time"

	"github.com/gavinwade12/ecLogger/protocols/ssm2"
	"github.com/gavinwade12/ecLogger/units"
)

// DefaultRetryDelay is how long a Session waits before restarting failed reads.
const DefaultRetryDelay = time.Second

// An Option configures a Session or a Runner.
type Option func(*options)

type options struct {
	retryDelay time.Duration
}

// WithRetryDelay sets how long to wait before restarting failed reads.
// The default is DefaultRetryDelay.
func WithRetryDelay(d time.Duration) Option {
	return func(o *options) { o.retryDelay = d }
}

// A Session logs a fixed set of parameters from a connection. It restarts
// the reads when they fail, converts the values to the configured units,
// and tracks the min and max of each value. A Session is run once; use a
// Runner to replace one with another.
type Session struct {
	conn    ssm2.Connection
	params  []ssm2.Parameter
	derived []ssm2.DerivedParameter
	// requested is the IDs the caller asked for, which excludes the parameters
	// that are only read to calculate derived parameters.
	requested map[string]bool

	// unitFor returns the unit a parameter's values should be converted to.
	unitFor func(id string) (units.Unit, bool)
	log     ssm2.Logger

	retryDelay time.Duration

	statsMu sync.Mutex
	stats   map[string]*Stats
}

// NewSession returns a Session that logs params and derived from conn,
// converting values to the units returned by unitFor. Any other parameters
// the derived parameters depend on are read too, but their values are never
// emitted.
func NewSession(conn ssm2.Connection, params []ssm2.Parameter, derived []ssm2.DerivedParameter,
	unitFor func(id string) (units.Unit, bool), log ssm2.Logger, opts ...Option) *Session {
	if log == nil {
		log = ssm2.NopLogger
	}
	o := options{retryDelay: DefaultRetryDelay}
	for _, opt := range opts {
		opt(&o)
	}

	requested := make(map[string]bool, len(params)+len(derived))
	for _, p := range params {
		requested[p.Id] = true
	}
	for _, d := range derived {
		requested[d.Id] = true
	}

	allParams, allDerived := withDependencies(params, derived)
	return &Session{
		conn:       conn,
		params:     allParams,
		derived:    allDerived,
		requested:  requested,
		unitFor:    unitFor,
		log:        log,
		retryDelay: o.retryDelay,
		stats:      make(map[string]*Stats, len(requested)),
	}
}

// Run reads values until ctx is canceled, passing each set to emit. The
// values only contain the requested parameters. emit's map must not be
// modified.
func (s *Session) Run(ctx context.Context, emit func(map[string]ssm2.ParameterValue)) {
	for {
		reads, err := ssm2.ContinuousReads(ctx, s.conn, s.params, s.derived)
		if err != nil {
			s.log.Debugf("starting continuous reads: %v\n", err)
		} else {
			for values := range reads {
				s.removeDependencies(values)
				s.convert(values)
				s.track(values)
				emit(values)
			}
			// the reads end when ctx is canceled or after too many errors
			if ctx.Err() == nil {
				s.log.Debug("continuous reads ended; restarting")
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(s.retryDelay):
		}
	}
}

// Stats returns the min and max of the values emitted so far for the
// parameter with the given ID. The values are in the configured unit at
// the time they were emitted. It reports false if no value has been emitted.
func (s *Session) Stats(id string) (Stats, bool) {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()

	st, ok := s.stats[id]
	if !ok {
		return Stats{}, false
	}
	return *st, true
}

func (s *Session) removeDependencies(values map[string]ssm2.ParameterValue) {
	for id := range values {
		if !s.requested[id] {
			delete(values, id)
		}
	}
}

// convert converts values to their configured units in place.
func (s *Session) convert(values map[string]ssm2.ParameterValue) {
	for id, val := range values {
		u, ok := s.unitFor(id)
		if !ok || u == val.Unit {
			continue
		}

		converted, err := val.ConvertTo(u)
		if err != nil {
			s.log.Debugf("converting %s from %s to %s: %v\n", id, val.Unit, u, err)
			continue
		}
		values[id] = *converted
	}
}

func (s *Session) track(values map[string]ssm2.ParameterValue) {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()

	for id, val := range values {
		st, ok := s.stats[id]
		if !ok {
			st = &Stats{}
			s.stats[id] = st
		}
		st.Add(val.Value)
	}
}

// withDependencies returns params and derived extended with any parameters
// the derived parameters depend on, directly or through other derived
// parameters. The given parameters keep their order; dependencies are appended.
func withDependencies(params []ssm2.Parameter, derived []ssm2.DerivedParameter) ([]ssm2.Parameter, []ssm2.DerivedParameter) {
	seen := map[string]bool{}
	outParams := make([]ssm2.Parameter, 0, len(params))
	outDerived := make([]ssm2.DerivedParameter, 0, len(derived))
	for _, p := range params {
		if !seen[p.Id] {
			seen[p.Id] = true
			outParams = append(outParams, p)
		}
	}

	var addDerived func(d ssm2.DerivedParameter)
	addDerived = func(d ssm2.DerivedParameter) {
		if seen[d.Id] {
			return
		}
		seen[d.Id] = true
		outDerived = append(outDerived, d)

		for _, dep := range d.DependsOnParameters {
			if seen[dep] {
				continue
			}
			if p, ok := ssm2.Parameters[dep]; ok {
				seen[dep] = true
				outParams = append(outParams, p)
			} else if dd, ok := ssm2.DerivedParameters[dep]; ok {
				addDerived(dd)
			}
		}
	}
	for _, d := range derived {
		addDerived(d)
	}

	return outParams, outDerived
}
