package logging

import "math"

// Stats tracks the minimum and maximum of a series of values.
type Stats struct {
	Min, Max float32
	// Count is the number of values seen.
	Count int
}

// Add records v.
func (s *Stats) Add(v float32) {
	if f := float64(v); math.IsNaN(f) || math.IsInf(f, 0) {
		return
	}

	s.Count++
	if s.Count == 1 || v < s.Min {
		s.Min = v
	}
	if s.Count == 1 || v > s.Max {
		s.Max = v
	}
}
