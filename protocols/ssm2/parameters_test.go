package ssm2_test

import (
	"reflect"
	"testing"

	"github.com/gavinwade12/ssm2/protocols/ssm2"
	"github.com/gavinwade12/ssm2/units"
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
