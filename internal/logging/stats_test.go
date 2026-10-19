package logging_test

import (
	"math"
	"testing"

	"github.com/gavinwade12/ecLogger/internal/logging"
)

func TestStats(t *testing.T) {
	tests := []struct {
		name     string
		values   []float32
		min, max float32
	}{
		{"all positive", []float32{800, 3000, 1200}, 800, 3000},
		{"all negative", []float32{-4, -1, -8}, -8, -1},
		{"single", []float32{5}, 5, 5},
		{"NaN first", []float32{nan, 800, 3000}, 800, 3000},
		{"NaN in the middle", []float32{800, nan, 3000}, 800, 3000},
		{"infinite", []float32{inf, 800, float32(math.Inf(-1)), 3000}, 800, 3000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s logging.Stats
			for _, v := range tt.values {
				s.Add(v)
			}
			if s.Min != tt.min || s.Max != tt.max {
				t.Errorf("got %+v, want min %v max %v", s, tt.min, tt.max)
			}
		})
	}
}

var (
	nan = float32(math.NaN())
	inf = float32(math.Inf(1))
)

func TestStatsCountsFiniteValues(t *testing.T) {
	var s logging.Stats
	s.Add(nan)
	s.Add(1)
	if s.Count != 1 {
		t.Errorf("Count = %d, want 1", s.Count)
	}
}
