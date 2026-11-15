package logging

import (
	"sync"
	"time"
)

// DefaultWindow is how much live history a Window keeps unless SetWindow says
// otherwise.
const DefaultWindow = 5 * time.Minute

// A Window holds the recent values of each parameter while logging, so a live
// chart has history to draw. It is safe for concurrent use: the logger's
// goroutine appends and the UI goroutine reads.
type Window struct {
	mu     sync.RWMutex
	window time.Duration
	series map[string]*liveSeries
}

// liveSeries is one parameter's samples. Points before start have expired but
// haven't been compacted away yet; see trim.
type liveSeries struct {
	times  []time.Time
	values []float64
	start  int
}

// NewWindow returns an empty Window keeping DefaultWindow of history.
func NewWindow() *Window {
	return &Window{window: DefaultWindow, series: map[string]*liveSeries{}}
}

// SetWindow changes how much history is kept. Older points are dropped on
// the next Append.
func (w *Window) SetWindow(d time.Duration) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.window = d
}

// AppendAll records every value in values, keyed by id, at t, under one lock.
// t is passed in so callers (and tests) control the clock. Times must not go
// backwards for one id; a point earlier than the last is dropped so the series
// stays sorted for searching. Points more than the window older than t are
// discarded.
func (w *Window) AppendAll(t time.Time, values map[string]float64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for id, v := range values {
		w.append(id, t, v)
	}
}

func (w *Window) append(id string, t time.Time, value float64) {
	s := w.series[id]
	if s == nil {
		s = &liveSeries{}
		w.series[id] = s
	}
	if n := len(s.times); n > s.start && t.Before(s.times[n-1]) {
		return
	}
	s.times = append(s.times, t)
	s.values = append(s.values, value)
	s.trim(t.Add(-w.window))
}

// trim expires points before cutoff. Expiry just advances start; the slices
// are only shifted down once the dead prefix is at least as big as the live
// part, so each point is copied O(1) times on average.
func (s *liveSeries) trim(cutoff time.Time) {
	for s.start < len(s.times) && s.times[s.start].Before(cutoff) {
		s.start++
	}
	if s.start == 0 || s.start < len(s.times)-s.start {
		return
	}
	n := copy(s.times, s.times[s.start:])
	copy(s.values, s.values[s.start:])
	// Zero the tail so the dead copies are not kept reachable.
	clear(s.times[n:])
	s.times, s.values, s.start = s.times[:n], s.values[:n], 0
}

// View calls fn with id's current times and values, held under a read lock so
// nothing is copied. fn must not retain the slices or call back into w, and
// should be quick: it blocks appends. fn is not called for an unknown id.
func (w *Window) View(id string, fn func(times []time.Time, values []float64)) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if s := w.series[id]; s != nil {
		fn(s.times[s.start:], s.values[s.start:])
	}
}

// Downsample is the package-level Downsample over id's live values. The
// result is freshly allocated, so it is safe to use after the call. An
// unknown id gives empty buckets.
func (w *Window) Downsample(id string, from, to time.Time, n int) Downsampled {
	var d Downsampled
	w.View(id, func(times []time.Time, values []float64) {
		d = Downsample(times, values, from, to, n)
	})
	if d.Buckets == nil && n > 0 && from.Before(to) {
		d.Buckets = make([]Bucket, n)
	}
	return d
}

// Range returns the smallest and largest finite values held for id. ok is
// false if there are none.
func (w *Window) Range(id string) (min, max float64, ok bool) {
	w.View(id, func(_ []time.Time, values []float64) {
		min, max, ok = Range(values)
	})
	return min, max, ok
}

// Len returns the number of points held for id.
func (w *Window) Len(id string) int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if s := w.series[id]; s != nil {
		return len(s.times) - s.start
	}
	return 0
}

// Drop forgets id, for when it is no longer being logged.
func (w *Window) Drop(id string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.series, id)
}

// Reset forgets everything, for when the unit or the set of parameters
// changes and the old history no longer lines up.
func (w *Window) Reset() {
	w.mu.Lock()
	defer w.mu.Unlock()
	clear(w.series)
}
