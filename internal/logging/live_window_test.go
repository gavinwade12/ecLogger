package logging

import (
	"math"
	"sync"
	"testing"
	"time"
)

func TestWindowTrimsOldPoints(t *testing.T) {
	w := NewWindow()
	w.SetWindow(10 * time.Second)
	for i := 0; i <= 100; i++ {
		appendOne(w, "rpm", at(float64(i)), float64(i))
	}
	// Newest is 100s; the window keeps 90s..100s inclusive.
	if got := w.Len("rpm"); got != 11 {
		t.Fatalf("Len = %d, want 11", got)
	}
	w.View("rpm", func(times []time.Time, values []float64) {
		if !times[0].Equal(at(90)) || !times[len(times)-1].Equal(at(100)) ||
			values[0] != 90 || values[len(values)-1] != 100 {
			t.Errorf("held %v..%v", values[0], values[len(values)-1])
		}
	})
}

func TestWindowShrinkingDropsOnNextAppend(t *testing.T) {
	w := NewWindow()
	for i := 0; i < 100; i++ {
		appendOne(w, "a", at(float64(i)), 1)
	}
	w.SetWindow(5 * time.Second)
	appendOne(w, "a", at(100), 1)
	if got := w.Len("a"); got != 6 {
		t.Errorf("Len = %d, want 6", got)
	}
}

func TestWindowCompactionIsBounded(t *testing.T) {
	w := NewWindow()
	w.SetWindow(100 * time.Second)
	for i := 0; i < 100000; i++ {
		appendOne(w, "a", at(float64(i)), float64(i))
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	if c := cap(w.series["a"].times); c > 1000 {
		t.Errorf("cap = %d; backing array grew without bound", c)
	}
}

func TestWindowDropsOutOfOrder(t *testing.T) {
	w := NewWindow()
	appendOne(w, "a", at(5), 1)
	appendOne(w, "a", at(4), 2)
	appendOne(w, "a", at(5), 3) // equal is fine
	if got := w.Len("a"); got != 2 {
		t.Errorf("Len = %d, want 2", got)
	}
}

func TestWindowDownsampleAndRange(t *testing.T) {
	w := NewWindow()
	w.AppendAll(at(0), map[string]float64{"a": 1, "b": 10})
	w.AppendAll(at(1), map[string]float64{"a": 3, "b": math.NaN()})
	d := w.Downsample("a", at(0), at(2), 2)
	if len(d.Buckets) != 2 || d.Buckets[0].Last != 1 || d.Buckets[1].Last != 3 {
		t.Errorf("buckets = %+v", d.Buckets)
	}
	if d := w.Downsample("b", at(0), at(2), 2); d.Buckets[0].N != 1 || d.Buckets[1].N != 0 {
		t.Errorf("b buckets = %+v, want the NaN skipped", d.Buckets)
	}
	if d := w.Downsample("missing", at(0), at(2), 3); len(d.Buckets) != 3 || d.Buckets[0].N != 0 {
		t.Errorf("unknown id = %+v", d)
	}
	if min, max, ok := w.Range("b"); !ok || min != 10 || max != 10 {
		t.Errorf("Range(b) = %v, %v, %v", min, max, ok)
	}
	if _, _, ok := w.Range("missing"); ok {
		t.Error("Range of unknown id is ok")
	}
}

func TestWindowDropAndReset(t *testing.T) {
	w := NewWindow()
	appendOne(w, "a", at(0), 1)
	appendOne(w, "b", at(0), 1)
	w.Drop("a")
	if w.Len("a") != 0 || w.Len("b") != 1 {
		t.Error("Drop removed the wrong series")
	}
	w.Reset()
	if w.Len("a") != 0 || w.Len("b") != 0 {
		t.Error("Reset left data behind")
	}
	appendOne(w, "a", at(0), 1) // still usable
	if w.Len("a") != 1 {
		t.Error("appending after Reset failed")
	}
}

func TestWindowConcurrent(t *testing.T) {
	w := NewWindow()
	w.SetWindow(time.Second)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 5000; i++ {
			appendOne(w, "a", at(float64(i)/100), float64(i))
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			w.Downsample("a", at(0), at(60), 100)
			w.Range("a")
			w.Len("a")
			if i%100 == 0 {
				w.SetWindow(time.Second)
			}
		}
	}()
	wg.Wait()
}

func appendOne(w *Window, id string, t time.Time, v float64) {
	w.AppendAll(t, map[string]float64{id: v})
}
