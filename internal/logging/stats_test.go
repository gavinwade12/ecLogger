package logging_test

import (
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s logging.Stats
			for _, v := range tt.values {
				s.Add(v)
			}
			if s.Min != tt.min || s.Max != tt.max || s.Count != len(tt.values) {
				t.Errorf("got %+v, want min %v max %v count %d", s, tt.min, tt.max, len(tt.values))
			}
		})
	}
}
