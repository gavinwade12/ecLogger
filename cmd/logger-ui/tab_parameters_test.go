package main

import (
	"reflect"
	"testing"

	"github.com/gavinwade12/ecLogger/units"
)

func TestUnitOptions(t *testing.T) {
	want := []string{"kPa", "bar", "hPa", "inHg", "mmHg", "psi"}
	for i := 0; i < 20; i++ { // map iteration order varies between runs
		if got := unitOptions(units.KPA); !reflect.DeepEqual(got, want) {
			t.Fatalf("unitOptions(kPa) = %v, want %v", got, want)
		}
	}
	if got := unitOptions(units.Gear); !reflect.DeepEqual(got, []string{"gear"}) {
		t.Fatalf("unitOptions(gear) = %v, want [gear]", got)
	}
}
