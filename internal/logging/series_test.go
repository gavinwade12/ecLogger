package logging

import (
	"math"
	"testing"
	"time"
)

var t0 = time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

func at(sec float64) time.Time { return t0.Add(time.Duration(sec * float64(time.Second))) }

func TestDownsample(t *testing.T) {
	nan, inf := math.NaN(), math.Inf(1)
	times := []time.Time{at(-2), at(-1), at(0), at(1), at(2), at(3), at(4), at(5), at(10), at(11)}
	values := []float64{-20, -10, 5, 1, 9, nan, inf, 3, 100, 110}

	d := Downsample(times, values, at(0), at(10), 5) // 2s buckets
	want := []Bucket{
		{Min: 1, Max: 5, First: 5, Last: 1, N: 2},
		{Min: 9, Max: 9, First: 9, Last: 9, N: 1},
		{Min: 3, Max: 3, First: 3, Last: 3, N: 1}, // NaN and Inf at 3s, 4s are skipped
		{N: 0},
		{N: 0},
	}
	if len(d.Buckets) != len(want) {
		t.Fatalf("got %d buckets, want %d", len(d.Buckets), len(want))
	}
	for i := range want {
		if d.Buckets[i] != want[i] {
			t.Errorf("bucket %d = %+v, want %+v", i, d.Buckets[i], want[i])
		}
	}
	if d.Before == nil || d.Before.V != -10 {
		t.Errorf("Before = %+v, want -10", d.Before)
	}
	if d.After == nil || d.After.V != 100 {
		t.Errorf("After = %+v, want 100 (at == to is outside)", d.After)
	}
}

func TestDownsampleSpikeSurvives(t *testing.T) {
	var times []time.Time
	var values []float64
	for i := 0; i < 10000; i++ {
		times = append(times, at(float64(i)/10))
		values = append(values, 1)
	}
	values[5555] = 99
	d := Downsample(times, values, at(0), at(1000), 7)
	var max float64
	for _, b := range d.Buckets {
		max = math.Max(max, b.Max)
	}
	if max != 99 {
		t.Errorf("max = %v, want the spike", max)
	}
}

func TestDownsampleEdges(t *testing.T) {
	times := []time.Time{at(1), at(2)}
	values := []float64{1, 2}
	tests := []struct {
		name        string
		times       []time.Time
		values      []float64
		from, to    time.Time
		n           int
		wantBuckets int
		wantN       int
	}{
		{"n zero", times, values, at(0), at(3), 0, 0, 0},
		{"n negative", times, values, at(0), at(3), -1, 0, 0},
		{"from after to", times, values, at(3), at(0), 4, 0, 0},
		{"from equals to", times, values, at(1), at(1), 4, 0, 0},
		{"empty input", nil, nil, at(0), at(3), 4, 4, 0},
		{"all before", times, values, at(5), at(6), 4, 4, 0},
		{"all after", times, values, at(-5), at(-4), 4, 4, 0},
		{"short values", times, values[:1], at(0), at(3), 3, 3, 1},
		{"single bucket", times, values, at(0), at(3), 1, 1, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := Downsample(tt.times, tt.values, tt.from, tt.to, tt.n)
			if len(d.Buckets) != tt.wantBuckets {
				t.Fatalf("buckets = %d, want %d", len(d.Buckets), tt.wantBuckets)
			}
			total := 0
			for _, b := range d.Buckets {
				total += b.N
			}
			if total != tt.wantN {
				t.Errorf("total N = %d, want %d", total, tt.wantN)
			}
		})
	}
}

func TestDownsampleNeighboursSkipNaN(t *testing.T) {
	nan := math.NaN()
	times := []time.Time{at(0), at(1), at(2), at(3), at(4), at(5)}
	values := []float64{7, nan, nan, 1, nan, 8}
	d := Downsample(times, values, at(3), at(4), 2)
	if d.Before == nil || d.Before.V != 7 || !d.Before.T.Equal(at(0)) {
		t.Errorf("Before = %+v", d.Before)
	}
	if d.After == nil || d.After.V != 8 {
		t.Errorf("After = %+v", d.After)
	}
	d = Downsample(times, values, at(0), at(6), 2)
	if d.Before != nil || d.After != nil {
		t.Errorf("neighbours = %+v, %+v, want none", d.Before, d.After)
	}
}

func TestRange(t *testing.T) {
	nan := math.NaN()
	tests := []struct {
		name     string
		in       []float64
		min, max float64
		ok       bool
	}{
		{"empty", nil, 0, 0, false},
		{"only non-finite", []float64{nan, math.Inf(-1)}, 0, 0, false},
		{"mixed", []float64{3, nan, -2, math.Inf(1), 8}, -2, 8, true},
		{"single", []float64{4}, 4, 4, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			min, max, ok := Range(tt.in)
			if min != tt.min || max != tt.max || ok != tt.ok {
				t.Errorf("= %v, %v, %v; want %v, %v, %v", min, max, ok, tt.min, tt.max, tt.ok)
			}
		})
	}
}

func TestNiceAxis(t *testing.T) {
	tests := []struct {
		name         string
		min, max     float64
		maxTicks     int
		lo, hi, step float64
	}{
		{"rpm", 750, 6800, 8, 0, 7000, 1000},
		{"rpm few ticks", 750, 6800, 5, 0, 8000, 2000},
		{"percent", 0, 100, 5, 0, 100, 50}, // 25 is not a nice step, so 50
		{"constant", 5, 5, 5, 4.5, 5.5, 0.5},
		{"zero constant", 0, 0, 5, -1, 1, 0.5},
		{"negative", -12, 31, 6, -20, 40, 10},
		{"swapped", 10, 0, 3, 0, 10, 5},
		{"tiny maxTicks", 0, 10, 0, 0, 10, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lo, hi, step := NiceAxis(tt.min, tt.max, tt.maxTicks)
			if lo != tt.lo || hi != tt.hi || step != tt.step {
				t.Errorf("= %v, %v, %v; want %v, %v, %v", lo, hi, step, tt.lo, tt.hi, tt.step)
			}
		})
	}
}

func TestTicks(t *testing.T) {
	got := Ticks(0, 3000, 1000)
	want := []float64{0, 1000, 2000, 3000}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("tick %d = %v, want %v", i, got[i], want[i])
		}
	}
	got = Ticks(0.1, 0.5, 0.1)
	if len(got) != 5 || math.Abs(got[4]-0.5) > 1e-12 {
		t.Errorf("fractional ticks = %v", got)
	}
	if Ticks(0, 1, 0) != nil || Ticks(1, 0, 1) != nil {
		t.Error("bad input should give no ticks")
	}
}
