package logging

import (
	"math"
	"sort"
	"time"
)

// A Bucket summarizes the finite values that fall in one slice of time, which
// the chart draws as one pixel column.
//
// Min and Max are kept so a one-sample spike still reaches its full height
// however far the chart is zoomed out. First and Last are kept so the chart
// can join one column to the next with a line: from this bucket's Last to the
// next non-empty bucket's First. Without them the joins would have to guess
// between Min and Max and the trace would zigzag.
type Bucket struct {
	Min, Max, First, Last float64
	// N is the number of values in the bucket; 0 means the bucket is empty
	// and the other fields are meaningless.
	N int
}

// A Point is a single sample.
type Point struct {
	T time.Time
	V float64
}

// Downsampled is a series reduced to one Bucket per pixel column.
type Downsampled struct {
	Buckets []Bucket
	// Before is the last finite point before the range, and After the first
	// finite point at or after its end. They let the chart draw a line
	// entering from and leaving to off-screen, instead of starting the trace
	// at the first visible sample. Nil when there is no such point.
	Before, After *Point
}

// Downsample splits [from, to) into n equal buckets and summarizes the
// values of times in each. times must be sorted ascending; it is searched
// rather than scanned, so the cost is O(n + visible points) however long the
// series is. NaN and infinite values are skipped, so a gap in the data
// leaves empty buckets.
//
// If n <= 0 or from is not before to, the result has no buckets. times and
// values should be the same length; any excess of one is ignored.
func Downsample(times []time.Time, values []float64, from, to time.Time, n int) Downsampled {
	if len(values) < len(times) {
		times = times[:len(values)]
	}
	if n <= 0 || !from.Before(to) {
		return Downsampled{}
	}

	d := Downsampled{Buckets: make([]Bucket, n)}
	span := float64(to.Sub(from))

	lo := sort.Search(len(times), func(i int) bool { return !times[i].Before(from) })
	hi := lo + sort.Search(len(times)-lo, func(i int) bool { return !times[lo+i].Before(to) })

	for i := lo; i < hi; i++ {
		v := values[i]
		if !finite(v) {
			continue
		}
		// Float math: Duration*n could overflow for long spans and large n.
		idx := int(float64(times[i].Sub(from)) / span * float64(n))
		if idx >= n { // rounding at the very end of the range
			idx = n - 1
		}
		b := &d.Buckets[idx]
		if b.N == 0 {
			*b = Bucket{Min: v, Max: v, First: v, Last: v, N: 1}
			continue
		}
		b.Min = math.Min(b.Min, v)
		b.Max = math.Max(b.Max, v)
		b.Last = v
		b.N++
	}

	// Skipping non-finite values here is cheap in practice: it only walks as
	// far as the nearest real sample.
	for i := lo - 1; i >= 0; i-- {
		if finite(values[i]) {
			d.Before = &Point{times[i], values[i]}
			break
		}
	}
	for i := hi; i < len(times); i++ {
		if finite(values[i]) {
			d.After = &Point{times[i], values[i]}
			break
		}
	}
	return d
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// Range returns the smallest and largest finite values. ok is false if there
// are none.
func Range(values []float64) (min, max float64, ok bool) {
	for _, v := range values {
		if !finite(v) {
			continue
		}
		if !ok {
			min, max, ok = v, v, true
			continue
		}
		min = math.Min(min, v)
		max = math.Max(max, v)
	}
	return min, max, ok
}

// NiceAxis widens [min, max] to round bounds on multiples of a round step
// (1, 2 or 5 times a power of ten), such that there are at most maxTicks
// ticks from lo to hi inclusive. A zero-width range (a parameter that never
// changed) is padded so the line sits mid-lane rather than on an edge.
// maxTicks is raised to 2 if lower.
func NiceAxis(min, max float64, maxTicks int) (lo, hi, step float64) {
	if maxTicks < 2 {
		maxTicks = 2
	}
	if min > max {
		min, max = max, min
	}
	if min == max {
		pad := math.Abs(min) * 0.1
		if pad == 0 {
			pad = 1
		}
		min, max = min-pad, max+pad
	}

	step = niceStep((max - min) / float64(maxTicks-1))
	lo = math.Floor(min/step) * step
	hi = math.Ceil(max/step) * step
	return lo, hi, step
}

// niceStep rounds x up to 1, 2 or 5 times a power of ten.
func niceStep(x float64) float64 {
	exp := math.Pow(10, math.Floor(math.Log10(x)))
	switch f := x / exp; {
	case f <= 1:
		return exp
	case f <= 2:
		return 2 * exp
	case f <= 5:
		return 5 * exp
	default:
		return 10 * exp
	}
}

// Ticks returns the values from lo to hi at multiples of step, as returned by
// NiceAxis. Each is computed from lo rather than accumulated, so rounding
// error doesn't build up.
func Ticks(lo, hi, step float64) []float64 {
	if step <= 0 || hi < lo {
		return nil
	}
	n := int(math.Round((hi-lo)/step)) + 1
	ticks := make([]float64, n)
	for i := range ticks {
		ticks[i] = lo + float64(i)*step
	}
	return ticks
}
