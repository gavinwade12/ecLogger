package ssm2_test

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/gavinwade12/ecLogger/protocols/ssm2"
	"github.com/gavinwade12/ecLogger/units"
)

func TestAddress_Add(t *testing.T) {
	type fields struct {
		Address [3]byte
		Length  int
		Bit     uint8
	}
	tests := []struct {
		name   string
		fields fields
		i      uint32
		want   [3]byte
	}{
		{
			"0x104F3A + 0",
			fields{[3]byte{0x10, 0x4F, 0x3A}, 1, 0},
			0,
			[3]byte{0x10, 0x4F, 0x3A},
		},
		{
			"0x000000 + 1",
			fields{[3]byte{0x00, 0x00, 0x00}, 1, 0},
			1,
			[3]byte{0x00, 0x00, 0x01},
		},
		{
			"0x1FFFFF + 36",
			fields{[3]byte{0x1F, 0xFF, 0xFF}, 1, 0},
			36,
			[3]byte{0x20, 0x00, 0x23},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := ssm2.Address{
				Address: tt.fields.Address,
				Length:  tt.fields.Length,
				Bit:     tt.fields.Bit,
			}
			if got := a.Add(tt.i); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Address.Add() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParameterValue_ConvertTo(t *testing.T) {
	type fields struct {
		Value float32
		Unit  units.Unit
	}
	tests := []struct {
		name    string
		fields  fields
		u       units.Unit
		want    *ssm2.ParameterValue
		wantErr bool
	}{
		{
			"Same unit",
			fields{10, units.AFR},
			units.AFR,
			&ssm2.ParameterValue{10, units.AFR},
			false,
		},
		{
			"Valid conversion",
			fields{25, units.MPH},
			units.KMH,
			&ssm2.ParameterValue{40.233498, units.KMH},
			false,
		},
		{
			"Invalid conversion",
			fields{25, units.MPH},
			units.Gear,
			nil,
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := ssm2.ParameterValue{
				Value: tt.fields.Value,
				Unit:  tt.fields.Unit,
			}
			got, err := v.ConvertTo(tt.u)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParameterValue.ConvertTo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParameterValue.ConvertTo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParameterValue_SafeConvertTo(t *testing.T) {
	type fields struct {
		Value float32
		Unit  units.Unit
	}
	tests := []struct {
		name   string
		fields fields
		u      units.Unit
		want   ssm2.ParameterValue
	}{
		{
			"Valid conversion",
			fields{25, units.MPH},
			units.KMH,
			ssm2.ParameterValue{40.233498, units.KMH},
		},
		{
			"Invalid conversion",
			fields{25, units.MPH},
			units.AFR,
			ssm2.ParameterValue{0, units.AFR},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := ssm2.ParameterValue{
				Value: tt.fields.Value,
				Unit:  tt.fields.Unit,
			}
			if got := v.SafeConvertTo(tt.u); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParameterValue.SafeConvertTo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAvailableDerivedParameters(t *testing.T) {
	params := []ssm2.Parameter{
		ssm2.Parameters["P7"],
		ssm2.Parameters["P8"],
		ssm2.Parameters["P12"],
		ssm2.Parameters["P21"],
		ssm2.Parameters["P160"],
	}
	want := []ssm2.DerivedParameter{
		ssm2.DerivedParameters["P200"],
		ssm2.DerivedParameters["P201"],
		ssm2.DerivedParameters["P237"],
	}

	got := ssm2.AvailableDerivedParameters(params)

	valid := len(want) == len(got)
	if valid {
		for _, w := range want {
			found := false
			for _, g := range got {
				if w.Id == g.Id {
					found = true
					break
				}
			}
			if !found {
				valid = false
				break
			}
		}
	}

	if !valid {
		t.Errorf("AvailableDerivedParameters() = %v, want %v", got, want)
	}
}

func TestAvailableDerivedParameters_DerivedDependencies(t *testing.T) {
	params := []ssm2.Parameter{
		ssm2.Parameters["P7"],
		ssm2.Parameters["P8"],
		ssm2.Parameters["P11"],
		ssm2.Parameters["P12"],
	}

	got := ssm2.AvailableDerivedParameters(params)

	index := map[string]int{}
	for i, p := range got {
		index[p.Id] = i
	}
	for _, id := range []string{"P200", "P242", "P243"} {
		if _, ok := index[id]; !ok {
			t.Fatalf("expected %s to be available. got: %v.", id, index)
		}
	}
	if index["P200"] > index["P242"] || index["P200"] > index["P243"] {
		t.Errorf("P200 must come before the parameters that depend on it. got: %v.", index)
	}
}

func TestDerivedParameter_P202(t *testing.T) {
	v, err := ssm2.DerivedParameters["P202"].Value(map[string]ssm2.ParameterValue{
		"P7":  {Value: 200, Unit: units.KPA}, // manifold absolute pressure
		"P24": {Value: 100, Unit: units.KPA}, // atmospheric pressure
	})
	if err != nil {
		t.Fatal(err)
	}
	if v.Unit != units.PSI {
		t.Fatalf("unit = %s, want psi", v.Unit)
	}
	if math.Abs(float64(v.Value)-14.5) > 0.05 {
		t.Fatalf("value = %f psi, want ~14.5 (100 kPa)", v.Value)
	}
}

func TestDerivedParameter_P203(t *testing.T) {
	// 60 mph on 20 g/s of air at stoichiometric: 20/14.7 = 1.36 g/s of fuel,
	// or 0.000472 gal/s, while covering 0.01667 miles/s = ~35.3 mpg
	v, err := ssm2.DerivedParameters["P203"].Value(map[string]ssm2.ParameterValue{
		"P9":  {Value: 96.5606, Unit: units.KMH}, // vehicle speed
		"P12": {Value: 20, Unit: units.GS},       // mass airflow
		"P58": {Value: 1, Unit: units.Lambda},    // A/F sensor #1
	})
	if err != nil {
		t.Fatal(err)
	}
	if v.Unit != units.MPGUS {
		t.Fatalf("unit = %s, want mpg (US)", v.Unit)
	}
	if math.Abs(float64(v.Value)-35.28) > 0.05 {
		t.Fatalf("value = %f mpg, want ~35.28", v.Value)
	}
}

func TestDerivedParametersWithZeroDivisors(t *testing.T) {
	tests := []struct {
		id     string
		params map[string]ssm2.ParameterValue
	}{
		{"P200", map[string]ssm2.ParameterValue{"P8": {Value: 0, Unit: units.RPM}, "P12": {Value: 0, Unit: units.GS}}},
		{"P200", map[string]ssm2.ParameterValue{"P8": {Value: 0, Unit: units.RPM}, "P12": {Value: 2, Unit: units.GS}}},
		{"P203", map[string]ssm2.ParameterValue{"P9": {Value: 0, Unit: units.KMH}, "P12": {Value: 0, Unit: units.GS}, "P58": {Value: 1, Unit: units.Lambda}}},
		{"P203", map[string]ssm2.ParameterValue{"P9": {Value: 50, Unit: units.KMH}, "P12": {Value: 0, Unit: units.GS}, "P58": {Value: 1, Unit: units.Lambda}}},
		{"P203", map[string]ssm2.ParameterValue{"P9": {Value: 50, Unit: units.KMH}, "P12": {Value: 20, Unit: units.GS}, "P58": {Value: 0, Unit: units.Lambda}}},
		{"P232", map[string]ssm2.ParameterValue{"P160": {Value: 0}, "P230": {Value: 0}}},
		{"P237", map[string]ssm2.ParameterValue{"P160": {Value: 1}, "P7": {Value: 0, Unit: units.KPA}}},
		{"P242", map[string]ssm2.ParameterValue{"P200": {Value: 1}, "P11": {Value: 20, Unit: units.C}, "P7": {Value: 0, Unit: units.KPA}}},
		{"P243", map[string]ssm2.ParameterValue{"P200": {Value: 1}, "P11": {Value: 20, Unit: units.C}, "P7": {Value: 0, Unit: units.KPA}}},
	}
	for _, tt := range tests {
		v, err := ssm2.DerivedParameters[tt.id].Value(tt.params)
		if !errors.Is(err, ssm2.ErrUndefinedValue) {
			t.Errorf("%s%v = %v, %v; want ErrUndefinedValue", tt.id, tt.params, v, err)
		}
	}
}

func TestDerivedParameter_P203Stopped(t *testing.T) {
	v, err := ssm2.DerivedParameters["P203"].Value(map[string]ssm2.ParameterValue{
		"P9":  {Value: 0, Unit: units.KMH},
		"P12": {Value: 3, Unit: units.GS},
		"P58": {Value: 1, Unit: units.Lambda},
	})
	if err != nil || v.Value != 0 {
		t.Fatalf("idling = %v, %v; want 0 mpg", v, err)
	}
}
