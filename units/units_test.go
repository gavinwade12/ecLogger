package units_test

import (
	"errors"
	"math"
	"slices"
	"testing"

	"github.com/gavinwade12/ecLogger/units"
)

func approxEqual(a, b, tolerance float32) bool {
	return math.Abs(float64(a-b)) <= float64(tolerance)
}

func TestConvert(t *testing.T) {
	tests := []struct {
		value    float32
		from, to units.Unit
		want     float32
	}{
		{100, units.KMH, units.MPH, 62.1371},
		{60, units.MPH, units.KMH, 96.5604},
		{100, units.C, units.F, 212},
		{32, units.F, units.C, 0},
		{-40, units.C, units.F, -40},
		{100, units.KPA, units.BAR, 1},
		{100, units.KPA, units.PSI, 14.5038},
		{1, units.BAR, units.PSI, 14.5038},
		{14.5038, units.PSI, units.BAR, 1},
		{1, units.InHG, units.MmHG, 25.4},
		{1013.25, units.HPA, units.KPA, 101.325},
	}
	for _, tt := range tests {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			got, err := units.Convert(tt.value, tt.from, tt.to)
			if err != nil {
				t.Fatal(err)
			}
			if !approxEqual(got, tt.want, 0.01) {
				t.Errorf("Convert(%v, %s, %s) = %v, want %v", tt.value, tt.from, tt.to, got, tt.want)
			}
		})
	}
}

func TestConvertInvalid(t *testing.T) {
	for _, pair := range [][2]units.Unit{
		{units.MPH, units.C},    // known source, unknown target
		{units.Gear, units.MPH}, // no conversions at all
	} {
		if _, err := units.Convert(1, pair[0], pair[1]); !errors.Is(err, units.ErrorInvalidConversion) {
			t.Errorf("Convert(%s, %s) error = %v, want ErrorInvalidConversion", pair[0], pair[1], err)
		}
	}
}

// Every conversion should have an inverse that round-trips the value.
func TestConversionsRoundTrip(t *testing.T) {
	for from, targets := range units.UnitConversions {
		for to := range targets {
			t.Run(string(from)+"<->"+string(to), func(t *testing.T) {
				if _, ok := units.UnitConversions[to][from]; !ok {
					t.Fatalf("no inverse conversion from %s to %s", to, from)
				}

				for _, v := range []float32{-40, 0, 1, 100, 250} {
					there, _ := units.Convert(v, from, to)
					back, _ := units.Convert(there, to, from)
					if !approxEqual(back, v, 0.01+float32(math.Abs(float64(v)))*0.001) {
						t.Errorf("%v %s -> %v %s -> %v %s", v, from, there, to, back, from)
					}
				}
			})
		}
	}
}

func TestConvertibleTo(t *testing.T) {
	got := units.ConvertibleTo(units.KPA)
	want := []units.Unit{units.BAR, units.HPA, units.InHG, units.MmHG, units.PSI}
	if !slices.Equal(got, want) {
		t.Errorf("ConvertibleTo(kPa) = %v, want %v", got, want)
	}

	if got := units.ConvertibleTo(units.Gear); len(got) != 0 {
		t.Errorf("ConvertibleTo(gear) = %v, want none", got)
	}
}
