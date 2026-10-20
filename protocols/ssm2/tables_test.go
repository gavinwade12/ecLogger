package ssm2_test

import (
	"math"
	"testing"

	"github.com/gavinwade12/ecLogger/protocols/ssm2"
)

// These tests guard the large, hand-maintained lookup tables against
// copy/paste mistakes.

func TestParametersTable(t *testing.T) {
	for key, p := range ssm2.Parameters {
		t.Run(key, func(t *testing.T) {
			if p.Id != key {
				t.Errorf("Id = %q, want it to match its key", p.Id)
			}
			if p.Name == "" {
				t.Error("Name is empty")
			}
			if p.Address == nil {
				t.Fatal("Address is nil")
			}
			if p.Address.Length < 1 || p.Address.Length > 4 {
				t.Errorf("Address.Length = %d, want 1-4", p.Address.Length)
			}
			if p.Address.Bit > 7 {
				t.Errorf("Address.Bit = %d, want 0-7", p.Address.Bit)
			}
			if p.CapabilityBitIndex > 7 {
				t.Errorf("CapabilityBitIndex = %d, want 0-7", p.CapabilityBitIndex)
			}
			if p.Value == nil {
				t.Fatal("Value is nil")
			}

			// every byte pattern should produce a value in the default unit
			for _, b := range []byte{0x00, 0x01, 0x7f, 0x80, 0xff} {
				raw := make([]byte, p.Address.Length)
				for i := range raw {
					raw[i] = b
				}
				v := p.Value(raw)
				if v.Unit != p.DefaultUnit {
					t.Errorf("Value(%x).Unit = %q, want DefaultUnit %q", raw, v.Unit, p.DefaultUnit)
				}
				if math.IsNaN(float64(v.Value)) {
					t.Errorf("Value(%x) is NaN", raw)
				}
			}
		})
	}
}

func TestDerivedParametersTable(t *testing.T) {
	for key, p := range ssm2.DerivedParameters {
		t.Run(key, func(t *testing.T) {
			if p.Id != key {
				t.Errorf("Id = %q, want it to match its key", p.Id)
			}
			if p.Name == "" {
				t.Error("Name is empty")
			}
			if len(p.DependsOnParameters) == 0 {
				t.Error("DependsOnParameters is empty")
			}
			if p.Value == nil {
				t.Fatal("Value is nil")
			}

			// build inputs from the dependencies' default-unit values
			inputs := map[string]ssm2.ParameterValue{}
			for _, dep := range p.DependsOnParameters {
				if param, ok := ssm2.Parameters[dep]; ok {
					inputs[dep] = ssm2.ParameterValue{Value: 1, Unit: param.DefaultUnit}
				} else if derived, ok := ssm2.DerivedParameters[dep]; ok {
					inputs[dep] = ssm2.ParameterValue{Value: 1, Unit: derived.DefaultUnit}
				} else {
					t.Fatalf("depends on unknown parameter %q", dep)
				}
			}

			v, err := p.Value(inputs)
			if err != nil {
				t.Fatal(err)
			}
			if v.Unit != p.DefaultUnit {
				t.Errorf("Value().Unit = %q, want DefaultUnit %q", v.Unit, p.DefaultUnit)
			}
		})
	}
}

func TestDTCsTable(t *testing.T) {
	type location struct {
		addr [3]byte
		bit  uint8
	}
	setLocations := map[location]string{}
	storedLocations := map[location]string{}

	for key, dtc := range ssm2.DTCs {
		t.Run(key, func(t *testing.T) {
			if dtc.ID != key {
				t.Errorf("ID = %q, want it to match its key", dtc.ID)
			}
			if dtc.Name == "" {
				t.Error("Name is empty")
			}
			for name, a := range map[string]ssm2.Address{"TempAddress": dtc.TempAddress, "Address": dtc.Address} {
				if a.Length != 1 {
					t.Errorf("%s.Length = %d, want 1", name, a.Length)
				}
				if a.Bit > 7 {
					t.Errorf("%s.Bit = %d, want 0-7", name, a.Bit)
				}
			}
			if dtc.TempAddress.Bit != dtc.Address.Bit {
				t.Errorf("TempAddress.Bit = %d, want it to match Address.Bit (%d)", dtc.TempAddress.Bit, dtc.Address.Bit)
			}
		})

		set := location{dtc.TempAddress.Address, dtc.TempAddress.Bit}
		if other, ok := setLocations[set]; ok {
			t.Errorf("%s and %s share the same set address/bit", key, other)
		}
		setLocations[set] = key

		stored := location{dtc.Address.Address, dtc.Address.Bit}
		if other, ok := storedLocations[stored]; ok {
			t.Errorf("%s and %s share the same stored address/bit", key, other)
		}
		storedLocations[stored] = key
	}
}
