package logging

import (
	"context"
	"sync"

	"github.com/gavinwade12/ecLogger/protocols/ssm2"
	"github.com/gavinwade12/ecLogger/units"
)

// A Runner manages the lifecycle of a Session: starting one, replacing it,
// and stopping it. Subscribers are kept across sessions, so they don't need
// to be re-registered when a new session is started.
type Runner struct {
	unitFor func(id string) (units.Unit, bool)
	log     ssm2.Logger

	opts []Option

	subscribersMu sync.Mutex
	subscribers   map[string]func(map[string]ssm2.ParameterValue)

	mu      sync.Mutex
	session *Session
	cancel  context.CancelFunc
	done    chan struct{}
}

// NewRunner returns a Runner whose sessions convert values to the units returned by unitFor.
// opts are passed on to each session.
func NewRunner(unitFor func(id string) (units.Unit, bool), log ssm2.Logger, opts ...Option) *Runner {
	return &Runner{
		unitFor:     unitFor,
		log:         log,
		opts:        opts,
		subscribers: map[string]func(map[string]ssm2.ParameterValue){},
	}
}

// Subscribe registers fn to receive values under key, replacing any existing
// subscriber with that key. fn is called from the session's goroutine and must
// not block on the UI goroutine. The values map is shared between subscribers
// and must not be modified. The session's Stats are up to date when fn is called.
func (r *Runner) Subscribe(key string, fn func(map[string]ssm2.ParameterValue)) {
	r.subscribersMu.Lock()
	defer r.subscribersMu.Unlock()
	r.subscribers[key] = fn
}

// Unsubscribe removes the subscriber with the given key. Once it returns, the
// subscriber won't be called again.
func (r *Runner) Unsubscribe(key string) {
	r.subscribersMu.Lock()
	defer r.subscribersMu.Unlock()
	delete(r.subscribers, key)
}

// Start stops any running session and starts a new one logging params and
// derived from conn.
func (r *Runner) Start(conn ssm2.Connection, params []ssm2.Parameter, derived []ssm2.DerivedParameter) {
	r.Stop()

	r.mu.Lock()
	defer r.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	session := NewSession(conn, params, derived, r.unitFor, r.log, r.opts...)
	r.session, r.cancel, r.done = session, cancel, done

	go func() {
		defer close(done)
		session.Run(ctx, r.publish)
	}()
}

// Stop stops the running session, if any, and waits for it to finish.
func (r *Runner) Stop() {
	r.mu.Lock()
	cancel, done := r.cancel, r.done
	r.cancel, r.done = nil, nil
	r.mu.Unlock()

	if cancel != nil {
		cancel()
		<-done
	}
}

// Running reports whether a session has been started and not stopped.
func (r *Runner) Running() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cancel != nil
}

// Stats returns the stats of the most recently started session for the
// parameter with the given ID. They remain available after the session is
// stopped, until the next one is started.
func (r *Runner) Stats(id string) (Stats, bool) {
	r.mu.Lock()
	session := r.session
	r.mu.Unlock()

	if session == nil {
		return Stats{}, false
	}
	return session.Stats(id)
}

func (r *Runner) publish(values map[string]ssm2.ParameterValue) {
	r.subscribersMu.Lock()
	defer r.subscribersMu.Unlock()
	for _, fn := range r.subscribers {
		fn(values)
	}
}
