package ssm2

import (
	"encoding/binary"

	"github.com/gavinwade12/ssm2/units"
)

// Parameter represents a parameter that can be read from the ECU.
type Parameter struct {
	Id          string
	Name        string
	Description string

	// CapabilityByteIndex points to the capability byte containing the parameter's flag.
	CapabilityByteIndex uint
	// CapabilityBitIndex is the index of the bit flag within the byte containing the parameter's flag.
	CapabilityBitIndex uint8

	// Address is present when the parameter value is read from RAM instead of calculated
	Address *ParameterAddress

	Value func(v []byte) ParameterValue
}

// DerivedParameter is a parameter derived from other calculated parameters instead of from ECU values.
type DerivedParameter struct {
	Id          string
	Name        string
	Description string

	DependsOnParameters []string

	Value func(parameters map[string]ParameterValue) (*ParameterValue, error)
}

// ParameterAddress describes the address(es) containing the value for the parameter
// with an optional bit for switch parameters.
type ParameterAddress struct {
	Address [3]byte
	Length  int // used when the value takes more than 1 address e.g. a 32-bit value on a 16-bit ECU
}

// Parameter value stores a parameter's value with its current unit.
type ParameterValue struct {
	Value float32
	Unit  units.Unit
}

// ConvertTo converts a parameter value from its current unit to the given unit.
func (v ParameterValue) ConvertTo(u units.Unit) (*ParameterValue, error) {
	if u == v.Unit {
		return &ParameterValue{v.Value, v.Unit}, nil
	}

	val, err := units.Convert(v.Value, v.Unit, u)
	if err != nil {
		return nil, err
	}

	return &ParameterValue{Value: val, Unit: u}, nil
}

func (v ParameterValue) SafeConvertTo(u units.Unit) ParameterValue {
	pv, _ := v.ConvertTo(u)
	if pv != nil {
		return *pv
	}
	return ParameterValue{Unit: u}
}

func AvailableDerivedParameters(params map[string]*Parameter) map[string]*DerivedParameter {
	derived := make(map[string]*DerivedParameter)

	for _, p := range DerivedParameters {
		available := true
		for _, d := range p.DependsOnParameters {
			if params[d] == nil {
				available = false
				break
			}
		}
		if available {
			derived[p.Id] = &p
		}
	}

	return derived
}

// Parameters defines all the parameters supported by the SSM2 protocol.
var Parameters = map[string]Parameter{
	"P1": {
		Id:                  "P1",
		Name:                "Engine Load (Relative)",
		Description:         "P1",
		CapabilityByteIndex: 8,
		CapabilityBitIndex:  7,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x7},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 100 / 255, units.Percent}
		},
	},
	"P2": {
		Id:                  "P2",
		Name:                "Coolant Temperature",
		Description:         "P2",
		CapabilityByteIndex: 8,
		CapabilityBitIndex:  6,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x8},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) - 40, units.C}
		},
	},
	"P3": {
		Id:                  "P3",
		Name:                "A/F Correction #1",
		Description:         "P3",
		CapabilityByteIndex: 8,
		CapabilityBitIndex:  5,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x9},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) * 100 / 128, units.Percent}
		},
	},
	"P4": {
		Id:                  "P4",
		Name:                "A/F Learning #1",
		Description:         "P4",
		CapabilityByteIndex: 8,
		CapabilityBitIndex:  4,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0xa},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) * 100 / 128, units.Percent}
		},
	},
	"P5": {
		Id:                  "P5",
		Name:                "A/F Correction #2",
		Description:         "P5",
		CapabilityByteIndex: 8,
		CapabilityBitIndex:  3,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0xb},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) * 100 / 128, units.Percent}
		},
	},
	"P6": {
		Id:                  "P6",
		Name:                "A/F Learning #2",
		Description:         "P6",
		CapabilityByteIndex: 8,
		CapabilityBitIndex:  2,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0xc},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) * 100 / 128, units.Percent}
		},
	},
	"P7": {
		Id:                  "P7",
		Name:                "Manifold Absolute Pressure",
		Description:         "P7-Pressure value calculated from the manifold absolute pressure sensor (absolute value)",
		CapabilityByteIndex: 8,
		CapabilityBitIndex:  1,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0xd},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.KPA}
		},
	},
	"P8": {
		Id:                  "P8",
		Name:                "Engine Speed",
		Description:         "P8",
		CapabilityByteIndex: 8,
		CapabilityBitIndex:  0,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0xe},
			Length:  2,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(binary.BigEndian.Uint32(v)) / 4, units.RPM}
		},
	},
	"P9": {
		Id:                  "P9",
		Name:                "Vehicle Speed",
		Description:         "P9",
		CapabilityByteIndex: 9,
		CapabilityBitIndex:  7,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x10},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.KMH}
		},
	},
	"P10": {
		Id:                  "P10",
		Name:                "Ignition Total Timing",
		Description:         "P10",
		CapabilityByteIndex: 9,
		CapabilityBitIndex:  6,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x11},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) / 2, units.Degress}
		},
	},
	"P11": {
		Id:                  "P11",
		Name:                "Intake Air Temperature",
		Description:         "P11",
		CapabilityByteIndex: 9,
		CapabilityBitIndex:  5,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x12},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) - 40, units.C}
		},
	},
	"P12": {
		Id:                  "P12",
		Name:                "Mass Airflow",
		Description:         "P12",
		CapabilityByteIndex: 9,
		CapabilityBitIndex:  4,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x13},
			Length:  2,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(binary.BigEndian.Uint32(v)) / 100, units.GS}
		},
	},
	"P13": {
		Id:                  "P13",
		Name:                "Throttle Opening Angle",
		Description:         "P13-Engine throttle opening angle.",
		CapabilityByteIndex: 9,
		CapabilityBitIndex:  3,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x15},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 100 / 255, units.Percent}
		},
	},
	"P14": {
		Id:                  "P14",
		Name:                "Front O2 Sensor #1",
		Description:         "P14",
		CapabilityByteIndex: 9,
		CapabilityBitIndex:  2,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x16},
			Length:  2,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(binary.BigEndian.Uint32(v)) / 200, units.Volts}
		},
	},
	"P15": {
		Id:                  "P15",
		Name:                "Rear O2 Sensor",
		Description:         "P15",
		CapabilityByteIndex: 9,
		CapabilityBitIndex:  1,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x18},
			Length:  2,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(binary.BigEndian.Uint32(v)) / 200, units.Volts}
		},
	},
	"P16": {
		Id:                  "P16",
		Name:                "Front O2 Sensor #2",
		Description:         "P16",
		CapabilityByteIndex: 9,
		CapabilityBitIndex:  0,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x1a},
			Length:  2,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(binary.BigEndian.Uint32(v)) / 200, units.Volts}
		},
	},
	"P17": {
		Id:                  "P17",
		Name:                "Battery Voltage",
		Description:         "P17",
		CapabilityByteIndex: 10,
		CapabilityBitIndex:  7,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x1c},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 8 / 100, units.Volts}
		},
	},
	"P18": {
		Id:                  "P18",
		Name:                "Mass Airflow Sensor Voltage",
		Description:         "P18",
		CapabilityByteIndex: 10,
		CapabilityBitIndex:  6,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x1d},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 50, units.Volts}
		},
	},
	"P19": {
		Id:                  "P19",
		Name:                "Throttle Sensor Voltage",
		Description:         "P19",
		CapabilityByteIndex: 10,
		CapabilityBitIndex:  5,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x1e},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 50, units.Volts}
		},
	},
	"P20": {
		Id:                  "P20",
		Name:                "Differential Pressure Sensor Voltage",
		Description:         "P20",
		CapabilityByteIndex: 10,
		CapabilityBitIndex:  4,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x1f},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 50, units.Volts}
		},
	},
	"P21": {
		Id:                  "P21",
		Name:                "Fuel Injector #1 Pulse Width",
		Description:         "P21-This parameter includes injector latency.",
		CapabilityByteIndex: 10,
		CapabilityBitIndex:  3,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x20},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 256, units.US}
		},
	},
	"P22": {
		Id:                  "P22",
		Name:                "Fuel Injector #2 Pulse Width",
		Description:         "P22-This parameter includes injector latency.",
		CapabilityByteIndex: 10,
		CapabilityBitIndex:  2,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x21},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 256, units.US}
		},
	},
	"P23": {
		Id:                  "P23",
		Name:                "Knock Correction Advance",
		Description:         "P23-Retard amount when knocking has occurred. Partial learned value of the learned ignition timing.",
		CapabilityByteIndex: 10,
		CapabilityBitIndex:  1,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x22},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) / 2, units.Degress}
		},
	},
	"P24": {
		Id:                  "P24",
		Name:                "Atmospheric Pressure",
		Description:         "P24",
		CapabilityByteIndex: 10,
		CapabilityBitIndex:  0,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x23},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.KPA}
		},
	},
	"P25": {
		Id:                  "P25",
		Name:                "Manifold Relative Pressure",
		Description:         "P25-Manifold Absolute Pressure [P7] minus current Atmospheric Pressure [P24].",
		CapabilityByteIndex: 11,
		CapabilityBitIndex:  7,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x24},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) - 128, units.KPA}
		},
	},
	"P26": {
		Id:                  "P26",
		Name:                "Pressure Differential Sensor",
		Description:         "P26",
		CapabilityByteIndex: 11,
		CapabilityBitIndex:  6,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x25},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) - 128, units.KPA}
		},
	},
	"P27": {
		Id:                  "P27",
		Name:                "Fuel Tank Pressure",
		Description:         "P27",
		CapabilityByteIndex: 11,
		CapabilityBitIndex:  5,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x26},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) / 4, units.HPA}
		},
	},
	"P28": {
		Id:                  "P28",
		Name:                "CO Adjustment",
		Description:         "P28",
		CapabilityByteIndex: 11,
		CapabilityBitIndex:  4,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x27},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 50, units.Volts}
		},
	},
	"P29": {
		Id:                  "P29",
		Name:                "Learned Ignition Timing",
		Description:         "P29-Advance or retard amount when knocking has occurred.",
		CapabilityByteIndex: 11,
		CapabilityBitIndex:  3,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x28},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) / 2, units.Degress}
		},
	},
	"P30": {
		Id:                  "P30",
		Name:                "Accelerator Pedal Angle",
		Description:         "P30-Accelerator pedal angle.",
		CapabilityByteIndex: 11,
		CapabilityBitIndex:  2,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x29},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 100 / 255, units.Percent}
		},
	},
	"P31": {
		Id:                  "P31",
		Name:                "Fuel Temperature",
		Description:         "P31",
		CapabilityByteIndex: 11,
		CapabilityBitIndex:  1,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x2a},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) - 40, units.C}
		},
	},
	"P32": {
		Id:                  "P32",
		Name:                "Front O2 Heater Current #1",
		Description:         "P32",
		CapabilityByteIndex: 11,
		CapabilityBitIndex:  0,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x2b},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 1004 / 25600, units.Amps}
		},
	},
	"P33": {
		Id:                  "P33",
		Name:                "Rear O2 Heater Current",
		Description:         "P33",
		CapabilityByteIndex: 12,
		CapabilityBitIndex:  7,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x2c},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 1004 / 25600, units.Amps}
		},
	},
	"P34": {
		Id:                  "P34",
		Name:                "Front O2 Heater Current #2",
		Description:         "P34",
		CapabilityByteIndex: 12,
		CapabilityBitIndex:  6,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x2d},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 1004 / 25600, units.Amps}
		},
	},
	"P35": {
		Id:                  "P35",
		Name:                "Fuel Level",
		Description:         "P35",
		CapabilityByteIndex: 12,
		CapabilityBitIndex:  5,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x2e},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 50, units.Volts}
		},
	},
	"P36": {
		Id:                  "P36",
		Name:                "Primary Wastegate Duty Cycle",
		Description:         "P36-Trubo Control Valve Duty Cycle",
		CapabilityByteIndex: 12,
		CapabilityBitIndex:  3,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x30},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 100 / 255, units.Percent}
		},
	},
	"P37": {
		Id:                  "P37",
		Name:                "Secondary Wastegate Duty Cycle",
		Description:         "P37",
		CapabilityByteIndex: 12,
		CapabilityBitIndex:  2,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x31},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 100 / 255, units.Percent}
		},
	},
	"P38": {
		Id:                  "P38",
		Name:                "CPC Valve Duty Ratio",
		Description:         "P38",
		CapabilityByteIndex: 12,
		CapabilityBitIndex:  1,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x32},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 100 / 255, units.Percent}
		},
	},
	"P39": {
		Id:                  "P39",
		Name:                "Tumble Valve Position Sensor Right",
		Description:         "P39",
		CapabilityByteIndex: 12,
		CapabilityBitIndex:  0,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x33},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 50, units.Volts}
		},
	},
	"P40": {
		Id:                  "P40",
		Name:                "Tumble Valve Position Sensor Left",
		Description:         "P40",
		CapabilityByteIndex: 13,
		CapabilityBitIndex:  7,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x34},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 50, units.Volts}
		},
	},
	"P41": {
		Id:                  "P41",
		Name:                "Idle Speed Control Valve Duty Ratio",
		Description:         "P41",
		CapabilityByteIndex: 13,
		CapabilityBitIndex:  6,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x35},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 2, units.Percent}
		},
	},
	"P42": {
		Id:                  "P42",
		Name:                "A/F Lean Correction",
		Description:         "P42",
		CapabilityByteIndex: 13,
		CapabilityBitIndex:  5,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x36},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 100 / 255, units.Percent}
		},
	},
	"P43": {
		Id:                  "P43",
		Name:                "A/F Heater Duty",
		Description:         "P43",
		CapabilityByteIndex: 13,
		CapabilityBitIndex:  4,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x37},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 100 / 255, units.Percent}
		},
	},
	"P44": {
		Id:                  "P44",
		Name:                "Idle Speed Control Valve Step",
		Description:         "P44",
		CapabilityByteIndex: 13,
		CapabilityBitIndex:  3,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x38},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.Steps}
		},
	},
	"P45": {
		Id:                  "P45",
		Name:                "Number of Exh. Gas Recirc. Steps",
		Description:         "P45",
		CapabilityByteIndex: 13,
		CapabilityBitIndex:  2,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x39},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.Steps}
		},
	},
	"P46": {
		Id:                  "P46",
		Name:                "Alternator Duty",
		Description:         "P46",
		CapabilityByteIndex: 13,
		CapabilityBitIndex:  1,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x3a},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.Percent}
		},
	},
	"P47": {
		Id:                  "P47",
		Name:                "Fuel Pump Duty",
		Description:         "P47",
		CapabilityByteIndex: 13,
		CapabilityBitIndex:  0,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x3b},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 100 / 255, units.Percent}
		},
	},
	"P48": {
		Id:                  "P48",
		Name:                "Intake VVT Advance Angle Right",
		Description:         "P48",
		CapabilityByteIndex: 14,
		CapabilityBitIndex:  7,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x3c},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) - 50, units.Degress}
		},
	},
	"P49": {
		Id:                  "P49",
		Name:                "Intake VVT Advance Angle Left",
		Description:         "P49",
		CapabilityByteIndex: 14,
		CapabilityBitIndex:  6,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x3d},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) - 50, units.Degress}
		},
	},
	"P50": {
		Id:                  "P50",
		Name:                "Intake OCV Duty Right",
		Description:         "P50",
		CapabilityByteIndex: 14,
		CapabilityBitIndex:  5,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x3e},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 100 / 255, units.Percent}
		},
	},
	"P51": {
		Id:                  "P51",
		Name:                "Intake OCV Duty Left",
		Description:         "P51",
		CapabilityByteIndex: 14,
		CapabilityBitIndex:  4,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x3f},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 100 / 255, units.Percent}
		},
	},
	"P52": {
		Id:                  "P52",
		Name:                "Intake OCV Current Right",
		Description:         "P52",
		CapabilityByteIndex: 14,
		CapabilityBitIndex:  3,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x40},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 32, units.Milliamps}
		},
	},
	"P53": {
		Id:                  "P53",
		Name:                "Intake OCV Current Left",
		Description:         "P53",
		CapabilityByteIndex: 14,
		CapabilityBitIndex:  2,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x41},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 32, units.Milliamps}
		},
	},
	"P54": {
		Id:                  "P54",
		Name:                "A/F Sensor #1 Current",
		Description:         "P54",
		CapabilityByteIndex: 14,
		CapabilityBitIndex:  1,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x42},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) / 8, units.Milliamps}
		},
	},
	"P55": {
		Id:                  "P55",
		Name:                "A/F Sensor #2 Current",
		Description:         "P55",
		CapabilityByteIndex: 14,
		CapabilityBitIndex:  0,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x43},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) / 8, units.Milliamps}
		},
	},
	"P56": {
		Id:                  "P56",
		Name:                "A/F Sensor #1 Resistance",
		Description:         "P56",
		CapabilityByteIndex: 15,
		CapabilityBitIndex:  7,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x44},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.Ohms}
		},
	},
	"P57": {
		Id:                  "P57",
		Name:                "A/F Sensor #2 Resistance",
		Description:         "P57",
		CapabilityByteIndex: 15,
		CapabilityBitIndex:  6,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x45},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.Ohms}
		},
	},
	"P58": {
		Id:                  "P58",
		Name:                "A/F Sensor #1",
		Description:         "P58",
		CapabilityByteIndex: 15,
		CapabilityBitIndex:  5,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x46},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 128, units.Lambda}
		},
	},
	"P59": {
		Id:                  "P59",
		Name:                "A/F Sensor #2",
		Description:         "P59",
		CapabilityByteIndex: 15,
		CapabilityBitIndex:  4,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x47},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 128, units.Lambda}
		},
	},
	"P60": {
		Id:                  "P60",
		Name:                "Gear Position",
		Description:         "P60",
		CapabilityByteIndex: 16,
		CapabilityBitIndex:  5,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x4a},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) + 1, units.Gear}
		},
	},
	"P61": {
		Id:                  "P61",
		Name:                "A/F Sensor #1 Heater Current",
		Description:         "P61",
		CapabilityByteIndex: 17,
		CapabilityBitIndex:  4,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x53},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 10, units.Amps}
		},
	},
	"P62": {
		Id:                  "P62",
		Name:                "A/F Sensor #2 Heater Current",
		Description:         "P62",
		CapabilityByteIndex: 17,
		CapabilityBitIndex:  3,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x54},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 10, units.Amps}
		},
	},
	"P63": {
		Id:                  "P63",
		Name:                "Roughness Monitor Cylinder #1",
		Description:         "P63",
		CapabilityByteIndex: 55,
		CapabilityBitIndex:  7,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0xce},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.MisfireCount}
		},
	},
	"P64": {
		Id:                  "P64",
		Name:                "Roughness Monitor Cylinder #2",
		Description:         "P64",
		CapabilityByteIndex: 55,
		CapabilityBitIndex:  6,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0xcf},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.MisfireCount}
		},
	},
	"P65": {
		Id:                  "P65",
		Name:                "A/F Correction #3 (16-bit ECU)",
		Description:         "P65",
		CapabilityByteIndex: 15,
		CapabilityBitIndex:  3,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0xd0},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) * 100 / 128, units.Percent}
		},
	},
	"P66": {
		Id:                  "P66",
		Name:                "A/F Learning #3",
		Description:         "P66",
		CapabilityByteIndex: 15,
		CapabilityBitIndex:  2,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0xd1},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) * 100 / 128, units.Percent}
		},
	},
	"P67": {
		Id:                  "P67",
		Name:                "Rear O2 Heater Voltage",
		Description:         "P67",
		CapabilityByteIndex: 15,
		CapabilityBitIndex:  1,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0xd2},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 50, units.Volts}
		},
	},
	"P68": {
		Id:                  "P68",
		Name:                "A/F Adjustment Voltage",
		Description:         "P68",
		CapabilityByteIndex: 15,
		CapabilityBitIndex:  0,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0xd3},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 50, units.Volts}
		},
	},
	"P69": {
		Id:                  "P69",
		Name:                "Roughness Monitor Cylinder #3",
		Description:         "P69",
		CapabilityByteIndex: 55,
		CapabilityBitIndex:  5,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0xd8},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.MisfireCount}
		},
	},
	"P70": {
		Id:                  "P70",
		Name:                "Roughness Monitor Cylinder #4",
		Description:         "P70",
		CapabilityByteIndex: 55,
		CapabilityBitIndex:  4,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0xd9},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.MisfireCount}
		},
	},
	"P71": {
		Id:                  "P71",
		Name:                "Throttle Motor Duty",
		Description:         "P71",
		CapabilityByteIndex: 38,
		CapabilityBitIndex:  5,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0xfa},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) * 100 / 128, units.Percent}
		},
	},
	"P72": {
		Id:                  "P72",
		Name:                "Throttle Motor Voltage",
		Description:         "P72",
		CapabilityByteIndex: 38,
		CapabilityBitIndex:  4,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0xfb},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 8 / 100, units.Volts}
		},
	},
	"P73": {
		Id:                  "P73",
		Name:                "Sub Throttle Sensor",
		Description:         "P73",
		CapabilityByteIndex: 40,
		CapabilityBitIndex:  7,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x0},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 50, units.Volts}
		},
	},
	"P74": {
		Id:                  "P74",
		Name:                "Main Throttle Sensor",
		Description:         "P74",
		CapabilityByteIndex: 40,
		CapabilityBitIndex:  6,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x1},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 50, units.Volts}
		},
	},
	"P75": {
		Id:                  "P75",
		Name:                "Sub Accelerator Sensor",
		Description:         "P75",
		CapabilityByteIndex: 40,
		CapabilityBitIndex:  5,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x2},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 50, units.Volts}
		},
	},
	"P76": {
		Id:                  "P76",
		Name:                "Main Accelerator Sensor",
		Description:         "P76",
		CapabilityByteIndex: 40,
		CapabilityBitIndex:  4,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x3},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 50, units.Volts}
		},
	},
	"P77": {
		Id:                  "P77",
		Name:                "Brake Booster Pressure",
		Description:         "P77",
		CapabilityByteIndex: 40,
		CapabilityBitIndex:  3,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x4},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.KPA}
		},
	},
	"P78": {
		Id:                  "P78",
		Name:                "Fuel Pressure (High)",
		Description:         "P78",
		CapabilityByteIndex: 40,
		CapabilityBitIndex:  2,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x5},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 25, units.MPA}
		},
	},
	"P79": {
		Id:                  "P79",
		Name:                "Exhaust Gas Temperature",
		Description:         "P79-Exhaust gas temperature reading.",
		CapabilityByteIndex: 40,
		CapabilityBitIndex:  1,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x6},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) + 40) * 5, units.C}
		},
	},
	"P80": {
		Id:                  "P80",
		Name:                "Cold Start Injector (Air Pump)",
		Description:         "P80",
		CapabilityByteIndex: 41,
		CapabilityBitIndex:  7,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x8},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 256, units.US}
		},
	},
	"P81": {
		Id:                  "P81",
		Name:                "SCV Step",
		Description:         "P81",
		CapabilityByteIndex: 41,
		CapabilityBitIndex:  6,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x9},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.Steps}
		},
	},
	"P82": {
		Id:                  "P82",
		Name:                "Memorised Cruise Speed",
		Description:         "P82",
		CapabilityByteIndex: 41,
		CapabilityBitIndex:  5,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0xa},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.KMH}
		},
	},
	"P83": {
		Id:                  "P83",
		Name:                "Exhaust VVT Advance Angle Right",
		Description:         "P83",
		CapabilityByteIndex: 43,
		CapabilityBitIndex:  7,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x18},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) - 50, units.Degress}
		},
	},
	"P84": {
		Id:                  "P84",
		Name:                "Exhaust VVT Advance Angle Left",
		Description:         "P84",
		CapabilityByteIndex: 43,
		CapabilityBitIndex:  6,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x19},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) - 50, units.Degress}
		},
	},
	"P85": {
		Id:                  "P85",
		Name:                "Exhaust OCV Duty Right",
		Description:         "P85",
		CapabilityByteIndex: 43,
		CapabilityBitIndex:  5,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x1a},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 100 / 255, units.Percent}
		},
	},
	"P86": {
		Id:                  "P86",
		Name:                "Exhaust OCV Duty Left",
		Description:         "P86",
		CapabilityByteIndex: 43,
		CapabilityBitIndex:  4,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x1b},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 100 / 255, units.Percent}
		},
	},
	"P87": {
		Id:                  "P87",
		Name:                "Exhaust OCV Current Right",
		Description:         "P87",
		CapabilityByteIndex: 43,
		CapabilityBitIndex:  3,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x1c},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 32, units.Milliamps}
		},
	},
	"P88": {
		Id:                  "P88",
		Name:                "Exhaust OCV Current Left",
		Description:         "P88",
		CapabilityByteIndex: 43,
		CapabilityBitIndex:  2,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x1d},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 32, units.Milliamps}
		},
	},
	"P89": {
		Id:                  "P89",
		Name:                "A/F Correction #3 (32-bit ECU)",
		Description:         "P89",
		CapabilityByteIndex: 15,
		CapabilityBitIndex:  3,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0xd0},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) * .078125) - 5, units.Percent}
		},
	},
	"P90": {
		Id:                  "P90",
		Name:                "IAM",
		Description:         "P90",
		CapabilityByteIndex: 55,
		CapabilityBitIndex:  0,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0xf9},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 16, units.Multiplier}
		},
	},
	"P91": {
		Id:                  "P91",
		Name:                "Fine Learning Knock Correction",
		Description:         "P91",
		CapabilityByteIndex: 55,
		CapabilityBitIndex:  0,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x99},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) * 0.25) - 32, units.Degress}
		},
	},
	"P92": {
		Id:                  "P92",
		Name:                "Radiator Fan Control",
		Description:         "P92",
		CapabilityByteIndex: 12,
		CapabilityBitIndex:  4,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x2f},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.Percent}
		},
	},
	"P93": {
		Id:                  "P93",
		Name:                "Front Wheel Speed",
		Description:         "P93",
		CapabilityByteIndex: 16,
		CapabilityBitIndex:  7,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x48},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.KMH}
		},
	},
	"P94": {
		Id:                  "P94",
		Name:                "ATF Temperature",
		Description:         "P94-0=-60C/-76F,1=-60C/-76F,2=-51C/-60F,3=-45C/-49F,4=-40C/-40F,5=-37C/-35F,6=-34C/-29F,etc",
		CapabilityByteIndex: 16,
		CapabilityBitIndex:  6,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x49},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.Index}
		},
	},
	"P95": {
		Id:                  "P95",
		Name:                "Line Pressure Duty Ratio",
		Description:         "P95",
		CapabilityByteIndex: 16,
		CapabilityBitIndex:  4,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x4b},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 2, units.Percent}
		},
	},
	"P96": {
		Id:                  "P96",
		Name:                "Lock Up Duty Ratio",
		Description:         "P96",
		CapabilityByteIndex: 16,
		CapabilityBitIndex:  3,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x4c},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 2, units.Percent}
		},
	},
	"P97": {
		Id:                  "P97",
		Name:                "Transfer Duty Ratio",
		Description:         "P97",
		CapabilityByteIndex: 16,
		CapabilityBitIndex:  2,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x4d},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 2, units.Percent}
		},
	},
	"P98": {
		Id:                  "P98",
		Name:                "Throttle Sensor Voltage",
		Description:         "P98",
		CapabilityByteIndex: 16,
		CapabilityBitIndex:  1,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x4e},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 45, units.Volts}
		},
	},
	"P99": {
		Id:                  "P99",
		Name:                "Turbine Revolution Speed",
		Description:         "P99",
		CapabilityByteIndex: 16,
		CapabilityBitIndex:  0,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x4f},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 32, units.RPM}
		},
	},
	"P100": {
		Id:                  "P100",
		Name:                "Brake Clutch Duty Ratio",
		Description:         "P100",
		CapabilityByteIndex: 17,
		CapabilityBitIndex:  7,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x50},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 2, units.Percent}
		},
	},
	"P101": {
		Id:                  "P101",
		Name:                "Rear Wheel Speed",
		Description:         "P101",
		CapabilityByteIndex: 17,
		CapabilityBitIndex:  6,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x51},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.KMH}
		},
	},
	"P102": {
		Id:                  "P102",
		Name:                "Manifold Pressure Sensor Voltage",
		Description:         "P102",
		CapabilityByteIndex: 17,
		CapabilityBitIndex:  5,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x52},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 50, units.Volts}
		},
	},
	"P103": {
		Id:                  "P103",
		Name:                "Lateral G Sensor Voltage",
		Description:         "P103",
		CapabilityByteIndex: 17,
		CapabilityBitIndex:  2,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x55},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 50, units.Volts}
		},
	},
	"P104": {
		Id:                  "P104",
		Name:                "ATF Temperature",
		Description:         "P104",
		CapabilityByteIndex: 17,
		CapabilityBitIndex:  1,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x56},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) - 50, units.C}
		},
	},
	"P105": {
		Id:                  "P105",
		Name:                "Low Clutch Duty",
		Description:         "P105",
		CapabilityByteIndex: 17,
		CapabilityBitIndex:  0,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x57},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 2, units.Percent}
		},
	},
	"P106": {
		Id:                  "P106",
		Name:                "High Clutch Duty",
		Description:         "P106",
		CapabilityByteIndex: 18,
		CapabilityBitIndex:  7,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x58},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 2, units.Percent}
		},
	},
	"P107": {
		Id:                  "P107",
		Name:                "Load and Reverse Brake (L and RB) Duty",
		Description:         "P107",
		CapabilityByteIndex: 18,
		CapabilityBitIndex:  6,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x59},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 2, units.Percent}
		},
	},
	"P108": {
		Id:                  "P108",
		Name:                "ATF Temperature 2",
		Description:         "P108",
		CapabilityByteIndex: 18,
		CapabilityBitIndex:  5,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x5a},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) - 50, units.C}
		},
	},
	"P109": {
		Id:                  "P109",
		Name:                "Voltage Center Differential Switch",
		Description:         "P109",
		CapabilityByteIndex: 18,
		CapabilityBitIndex:  4,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x5b},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 51, units.Volts}
		},
	},
	"P110": {
		Id:                  "P110",
		Name:                "AT Turbine Speed 1",
		Description:         "P110",
		CapabilityByteIndex: 18,
		CapabilityBitIndex:  3,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x5c},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 32, units.RPM}
		},
	},
	"P111": {
		Id:                  "P111",
		Name:                "AT Turbine Speed 2",
		Description:         "P111",
		CapabilityByteIndex: 18,
		CapabilityBitIndex:  2,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x5d},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 32, units.RPM}
		},
	},
	"P112": {
		Id:                  "P112",
		Name:                "Center Differential Real Current",
		Description:         "P112",
		CapabilityByteIndex: 18,
		CapabilityBitIndex:  1,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x5e},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 32, units.Amps}
		},
	},
	"P113": {
		Id:                  "P113",
		Name:                "Center Differential Indicate Current",
		Description:         "P113",
		CapabilityByteIndex: 18,
		CapabilityBitIndex:  0,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x5f},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 32, units.Amps}
		},
	},
	"P114": {
		Id:                  "P114",
		Name:                "SI-Drive Mode",
		Description:         "P114-0=---,1=S,2=S#,3=I,8=S#,16=I",
		CapabilityByteIndex: 38,
		CapabilityBitIndex:  7,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x6a},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.Index}
		},
	},
	"P115": {
		Id:                  "P115",
		Name:                "Throttle Sensor Closed Voltage",
		Description:         "P115",
		CapabilityByteIndex: 38,
		CapabilityBitIndex:  6,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x6b},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 50, units.Volts}
		},
	},
	"P116": {
		Id:                  "P116",
		Name:                "Exhaust Gas Temperature 2",
		Description:         "P116",
		CapabilityByteIndex: 40,
		CapabilityBitIndex:  0,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x7},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0])*5 + 200, units.C}
		},
	},
	"P117": {
		Id:                  "P117",
		Name:                "Air/Fuel Correction #4",
		Description:         "P117",
		CapabilityByteIndex: 41,
		CapabilityBitIndex:  4,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0xb},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 64) / 128 * 10, units.Percent}
		},
	},
	"P118": {
		Id:                  "P118",
		Name:                "Air/Fuel Learning #4",
		Description:         "P118",
		CapabilityByteIndex: 41,
		CapabilityBitIndex:  3,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0xc},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) / 128 * 100, units.Percent}
		},
	},
	"P119": {
		Id:                  "P119",
		Name:                "Fuel Level Sensor Resistance",
		Description:         "P119",
		CapabilityByteIndex: 41,
		CapabilityBitIndex:  2,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0xd},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 4 / 2, units.Ohms}
		},
	},
	"P120": {
		Id:                  "P120",
		Name:                "Estimated odometer",
		Description:         "P120-Estimated odometer - increments every 2km",
		CapabilityByteIndex: 41,
		CapabilityBitIndex:  1,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0xe},
			Length:  2,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(binary.BigEndian.Uint32(v)) * 2, units.Kilometers}
		},
	},
	"P121": {
		Id:                  "P121",
		Name:                "Fuel Tank Air Pressure",
		Description:         "P121",
		CapabilityByteIndex: 41,
		CapabilityBitIndex:  0,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x72},
			Length:  2,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(binary.BigEndian.Uint32(v)) / 10, units.BAR}
		},
	},
	"P122": {
		Id:                  "P122",
		Name:                "Oil Temperature",
		Description:         "P122",
		CapabilityByteIndex: 42,
		CapabilityBitIndex:  4,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x13},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) - 40, units.C}
		},
	},
	"P123": {
		Id:                  "P123",
		Name:                "Oil Switching Solenoid Valve (OSV) Duty (Right)",
		Description:         "P123",
		CapabilityByteIndex: 42,
		CapabilityBitIndex:  3,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x14},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 255 * 100, units.Percent}
		},
	},
	"P124": {
		Id:                  "P124",
		Name:                "Oil Switching Solenoid Valve (OSV) Duty (Left)",
		Description:         "P124",
		CapabilityByteIndex: 42,
		CapabilityBitIndex:  2,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x15},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 255 * 100, units.Percent}
		},
	},
	"P125": {
		Id:                  "P125",
		Name:                "Oil Switching Solenoid Valve (OSV) Current (Right)",
		Description:         "P125",
		CapabilityByteIndex: 42,
		CapabilityBitIndex:  1,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x16},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 32, units.Milliamps}
		},
	},
	"P126": {
		Id:                  "P126",
		Name:                "Oil Switching Solenoid Valve (OSV) Current (Left)",
		Description:         "P126",
		CapabilityByteIndex: 42,
		CapabilityBitIndex:  0,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x17},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 32, units.Milliamps}
		},
	},
	"P127": {
		Id:                  "P127",
		Name:                "VVL Lift Mode",
		Description:         "P127",
		CapabilityByteIndex: 43,
		CapabilityBitIndex:  1,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x1e},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.Raw}
		},
	},
	"P128": {
		Id:                  "P128",
		Name:                "H and LR/C Solenoid Valve Current",
		Description:         "P128",
		CapabilityByteIndex: 50,
		CapabilityBitIndex:  7,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x40},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 255, units.Amps}
		},
	},
	"P129": {
		Id:                  "P129",
		Name:                "D/C Solenoid Valve Current",
		Description:         "P129",
		CapabilityByteIndex: 50,
		CapabilityBitIndex:  6,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x41},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 255, units.Amps}
		},
	},
	"P130": {
		Id:                  "P130",
		Name:                "F/B Solenoid Valve Current",
		Description:         "P130",
		CapabilityByteIndex: 50,
		CapabilityBitIndex:  5,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x42},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 255, units.Amps}
		},
	},
	"P131": {
		Id:                  "P131",
		Name:                "I/C Solenoid Valve Current",
		Description:         "P131",
		CapabilityByteIndex: 50,
		CapabilityBitIndex:  4,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x43},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 255, units.Amps}
		},
	},
	"P132": {
		Id:                  "P132",
		Name:                "P/L Solenoid Valve Current",
		Description:         "P132",
		CapabilityByteIndex: 50,
		CapabilityBitIndex:  3,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x44},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 255, units.Amps}
		},
	},
	"P133": {
		Id:                  "P133",
		Name:                "L/U Solenoid Valve Current",
		Description:         "P133",
		CapabilityByteIndex: 50,
		CapabilityBitIndex:  2,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x45},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 255, units.Amps}
		},
	},
	"P134": {
		Id:                  "P134",
		Name:                "AWD Solenoid Valve Current",
		Description:         "P134",
		CapabilityByteIndex: 50,
		CapabilityBitIndex:  1,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x46},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 255, units.Amps}
		},
	},
	"P135": {
		Id:                  "P135",
		Name:                "Yaw Rate Sensor Voltage",
		Description:         "P135",
		CapabilityByteIndex: 50,
		CapabilityBitIndex:  0,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x47},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 51, units.Volts}
		},
	},
	"P136": {
		Id:                  "P136",
		Name:                "H and LR/C Solenoid Valve Pressure",
		Description:         "P136",
		CapabilityByteIndex: 51,
		CapabilityBitIndex:  7,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x48},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 10, units.KPA}
		},
	},
	"P137": {
		Id:                  "P137",
		Name:                "D/C Solenoid Valve Pressure",
		Description:         "P137",
		CapabilityByteIndex: 51,
		CapabilityBitIndex:  6,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x49},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 10, units.KPA}
		},
	},
	"P138": {
		Id:                  "P138",
		Name:                "F/B Solenoid Valve Pressure",
		Description:         "P138",
		CapabilityByteIndex: 51,
		CapabilityBitIndex:  5,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x4a},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 10, units.KPA}
		},
	},
	"P139": {
		Id:                  "P139",
		Name:                "I/C Solenoid Valve Pressure",
		Description:         "P139",
		CapabilityByteIndex: 51,
		CapabilityBitIndex:  4,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x4b},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 10, units.KPA}
		},
	},
	"P140": {
		Id:                  "P140",
		Name:                "P/L Solenoid Valve Pressure",
		Description:         "P140",
		CapabilityByteIndex: 51,
		CapabilityBitIndex:  3,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x4c},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 10, units.KPA}
		},
	},
	"P141": {
		Id:                  "P141",
		Name:                "L/U Solenoid Valve Pressure",
		Description:         "P141",
		CapabilityByteIndex: 51,
		CapabilityBitIndex:  2,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x4d},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 10, units.KPA}
		},
	},
	"P142": {
		Id:                  "P142",
		Name:                "AWD Solenoid Valve Pressure",
		Description:         "P142",
		CapabilityByteIndex: 51,
		CapabilityBitIndex:  1,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x4e},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 10, units.KPA}
		},
	},
	"P143": {
		Id:                  "P143",
		Name:                "Yaw Rate and  G Sensor Reference Voltage",
		Description:         "P143",
		CapabilityByteIndex: 51,
		CapabilityBitIndex:  0,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x4f},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 51, units.Volts}
		},
	},
	"P144": {
		Id:                  "P144",
		Name:                "Wheel Speed Front Right",
		Description:         "P144",
		CapabilityByteIndex: 52,
		CapabilityBitIndex:  7,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x3c},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.KMH}
		},
	},
	"P145": {
		Id:                  "P145",
		Name:                "Wheel Speed Front Left",
		Description:         "P145",
		CapabilityByteIndex: 52,
		CapabilityBitIndex:  6,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x3d},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.KMH}
		},
	},
	"P146": {
		Id:                  "P146",
		Name:                "Wheel Speed Rear Right",
		Description:         "P146",
		CapabilityByteIndex: 52,
		CapabilityBitIndex:  5,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x3e},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.KMH}
		},
	},
	"P147": {
		Id:                  "P147",
		Name:                "Wheel Speed Rear Left",
		Description:         "P147",
		CapabilityByteIndex: 52,
		CapabilityBitIndex:  4,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x3f},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.KMH}
		},
	},
	"P148": {
		Id:                  "P148",
		Name:                "Steering Angle Sensor",
		Description:         "P148-signed 16 bit value returned",
		CapabilityByteIndex: 52,
		CapabilityBitIndex:  3,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x5a},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.Degress}
		},
	},
	"P149": {
		Id:                  "P149",
		Name:                "Fwd/B Solenoid Valve Current",
		Description:         "P149",
		CapabilityByteIndex: 52,
		CapabilityBitIndex:  1,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x85},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 255, units.Amps}
		},
	},
	"P150": {
		Id:                  "P150",
		Name:                "Fwd/B Solenoid Valve Target Pressure",
		Description:         "P150",
		CapabilityByteIndex: 52,
		CapabilityBitIndex:  0,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x86},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 10, units.KPA}
		},
	},
	"P151": {
		Id:                  "P151",
		Name:                "Roughness Monitor Cylinder #5",
		Description:         "P151",
		CapabilityByteIndex: 55,
		CapabilityBitIndex:  3,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0xef},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.MisfireCount}
		},
	},
	"P152": {
		Id:                  "P152",
		Name:                "Roughness Monitor Cylinder #6",
		Description:         "P152",
		CapabilityByteIndex: 55,
		CapabilityBitIndex:  2,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0xf8},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.MisfireCount}
		},
	},
	"P153": {
		Id:                  "P153",
		Name:                "Learned Ignition Timing Correction",
		Description:         "P153-Value of only the whole learning value in the ignition timing learning value.",
		CapabilityByteIndex: 55,
		CapabilityBitIndex:  1,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0xf9},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 16, units.Degress}
		},
	},
	"P154": {
		Id:                  "P154",
		Name:                "Fuel Tank Pressure",
		Description:         "P154",
		CapabilityByteIndex: 59,
		CapabilityBitIndex:  7,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x9a},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) / 2, units.HPA}
		},
	},
	"P155": {
		Id:                  "P155",
		Name:                "Main Injection Period",
		Description:         "P155",
		CapabilityByteIndex: 60,
		CapabilityBitIndex:  7,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0xe1},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0])/5 - 15, units.DegreesCrankAngle}
		},
	},
	"P156": {
		Id:                  "P156",
		Name:                "Final Injection Amount",
		Description:         "P156",
		CapabilityByteIndex: 60,
		CapabilityBitIndex:  6,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0xe2},
			Length:  2,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(binary.BigEndian.Uint32(v)) / 256, units.MM3PerStroke}
		},
	},
	"P157": {
		Id:                  "P157",
		Name:                "Number of Times Injected",
		Description:         "P157",
		CapabilityByteIndex: 60,
		CapabilityBitIndex:  5,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0xe4},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.Count}
		},
	},
	"P158": {
		Id:                  "P158",
		Name:                "Target Intake Manifold Pressure",
		Description:         "P158",
		CapabilityByteIndex: 60,
		CapabilityBitIndex:  4,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0xe5},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.KPA}
		},
	},
	"P159": {
		Id:                  "P159",
		Name:                "Target Intake Air Amount",
		Description:         "P159",
		CapabilityByteIndex: 60,
		CapabilityBitIndex:  3,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0xe6},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 10, units.MGPerCylinder}
		},
	},
	"P160": {
		Id:                  "P160",
		Name:                "Air Mass",
		Description:         "P160",
		CapabilityByteIndex: 60,
		CapabilityBitIndex:  2,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0xe7},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 10, units.MGPerCylinder}
		},
	},
	"P161": {
		Id:                  "P161",
		Name:                "Exhaust Gas Recirculation (EGR) Target Valve Opening Angle",
		Description:         "P161",
		CapabilityByteIndex: 60,
		CapabilityBitIndex:  1,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0xe8},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) - 50, units.Degress}
		},
	},
	"P162": {
		Id:                  "P162",
		Name:                "Exhaust Gas Recirculation (EGR) Valve Opening Angle",
		Description:         "P162",
		CapabilityByteIndex: 60,
		CapabilityBitIndex:  0,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0xe9},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) - 50, units.Degress}
		},
	},
	"P163": {
		Id:                  "P163",
		Name:                "Exhaust Gas Recirculation (EGR) Duty",
		Description:         "P163",
		CapabilityByteIndex: 61,
		CapabilityBitIndex:  7,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0xea},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.Percent}
		},
	},
	"P164": {
		Id:                  "P164",
		Name:                "Common Rail Target Pressure",
		Description:         "P164",
		CapabilityByteIndex: 61,
		CapabilityBitIndex:  6,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0xeb},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.MPA}
		},
	},
	"P165": {
		Id:                  "P165",
		Name:                "Common Rail Pressure",
		Description:         "P165",
		CapabilityByteIndex: 61,
		CapabilityBitIndex:  5,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0xec},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.MPA}
		},
	},
	"P166": {
		Id:                  "P166",
		Name:                "Intake Air Temperature (combined)",
		Description:         "P166",
		CapabilityByteIndex: 61,
		CapabilityBitIndex:  4,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0xed},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) - 40, units.C}
		},
	},
	"P167": {
		Id:                  "P167",
		Name:                "Target Engine Speed",
		Description:         "P167",
		CapabilityByteIndex: 61,
		CapabilityBitIndex:  3,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0xee},
			Length:  2,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(binary.BigEndian.Uint32(v)) / 4, units.RPM}
		},
	},
	"P168": {
		Id:                  "P168",
		Name:                "Boost Pressure Feedback",
		Description:         "P168",
		CapabilityByteIndex: 61,
		CapabilityBitIndex:  2,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0xf0},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) - 128, units.Percent}
		},
	},
	"P169": {
		Id:                  "P169",
		Name:                "Electric Power Steering Current",
		Description:         "P169",
		CapabilityByteIndex: 61,
		CapabilityBitIndex:  1,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0xf5},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.Amps}
		},
	},
	"P170": {
		Id:                  "P170",
		Name:                "Fuel Pump Suction Control Valve Current (Target)",
		Description:         "P170-Target current value of suction control valve calculated by the ECM. Applies only to Diesel models.",
		CapabilityByteIndex: 61,
		CapabilityBitIndex:  0,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0xf6},
			Length:  2,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(binary.BigEndian.Uint32(v)), units.Milliamps}
		},
	},
	"P171": {
		Id:                  "P171",
		Name:                "Yaw Rate",
		Description:         "P171-signed 8 bit value returned",
		CapabilityByteIndex: 62,
		CapabilityBitIndex:  7,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0xf1},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 0.19118, units.DegreesPerSecond}
		},
	},
	"P172": {
		Id:                  "P172",
		Name:                "Lateral G",
		Description:         "P172-signed 8 bit value returned",
		CapabilityByteIndex: 62,
		CapabilityBitIndex:  6,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0xf2},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 1.0862, units.MetersPerSecondSquared}
		},
	},
	"P173": {
		Id:                  "P173",
		Name:                "Drivers Control Center Differential (DCCD) Torque Allocation",
		Description:         "P173",
		CapabilityByteIndex: 62,
		CapabilityBitIndex:  5,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0xf3},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.Raw}
		},
	},
	"P174": {
		Id:                  "P174",
		Name:                "Drivers Control Center Differential (DCCD) Mode",
		Description:         "P174",
		CapabilityByteIndex: 62,
		CapabilityBitIndex:  4,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0xf4},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.Raw}
		},
	},
	"P175": {
		Id:                  "P175",
		Name:                "Fuel Pump Suction Control Valve Current (Actual)",
		Description:         "P175-Actual current value of suction control valve. Input to the ECM. Applies only to Diesel models.",
		CapabilityByteIndex: 63,
		CapabilityBitIndex:  7,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0xf8},
			Length:  2,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(binary.BigEndian.Uint32(v)), units.Milliamps}
		},
	},
	"P176": {
		Id:                  "P176",
		Name:                "Mileage after Injector Learning",
		Description:         "P176",
		CapabilityByteIndex: 63,
		CapabilityBitIndex:  6,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0xfa},
			Length:  2,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(binary.BigEndian.Uint32(v)) * 5, units.Kilometers}
		},
	},
	"P177": {
		Id:                  "P177",
		Name:                "Mileage after Injector Replacement",
		Description:         "P177",
		CapabilityByteIndex: 63,
		CapabilityBitIndex:  5,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x4},
			Length:  2,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(binary.BigEndian.Uint32(v)) * 5, units.Kilometers}
		},
	},
	"P178": {
		Id:                  "P178",
		Name:                "Interior Heater",
		Description:         "P178",
		CapabilityByteIndex: 63,
		CapabilityBitIndex:  4,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x70},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.Steps}
		},
	},
	"P179": {
		Id:                  "P179",
		Name:                "Quantity Correction Cylinder #1",
		Description:         "P179",
		CapabilityByteIndex: 63,
		CapabilityBitIndex:  3,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x5d},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 100) * 10, units.US}
		},
	},
	"P180": {
		Id:                  "P180",
		Name:                "Quantity Correction Cylinder #2",
		Description:         "P180",
		CapabilityByteIndex: 63,
		CapabilityBitIndex:  2,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x5e},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 100) * 10, units.US}
		},
	},
	"P181": {
		Id:                  "P181",
		Name:                "Quantity Correction Cylinder #3",
		Description:         "P181",
		CapabilityByteIndex: 63,
		CapabilityBitIndex:  1,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x5f},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 100) * 10, units.US}
		},
	},
	"P182": {
		Id:                  "P182",
		Name:                "Quantity Correction Cylinder #4",
		Description:         "P182",
		CapabilityByteIndex: 63,
		CapabilityBitIndex:  0,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x60},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 100) * 10, units.US}
		},
	},
	"P183": {
		Id:                  "P183",
		Name:                "Battery Current",
		Description:         "P183",
		CapabilityByteIndex: 64,
		CapabilityBitIndex:  7,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x71},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) - 128, units.Amps}
		},
	},
	"P184": {
		Id:                  "P184",
		Name:                "Battery Temperature",
		Description:         "P184",
		CapabilityByteIndex: 64,
		CapabilityBitIndex:  6,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x73},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) - 40, units.C}
		},
	},
	"P185": {
		Id:                  "P185",
		Name:                "Alternator Control Mode",
		Description:         "P185-0=High,1=ExHigh,2=Low,3=Mid",
		CapabilityByteIndex: 64,
		CapabilityBitIndex:  5,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x72},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.Index}
		},
	},
	"P186": {
		Id:                  "P186",
		Name:                "Cumulative Ash Ratio",
		Description:         "P186",
		CapabilityByteIndex: 70,
		CapabilityBitIndex:  7,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x75},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.Percent}
		},
	},
	"P187": {
		Id:                  "P187",
		Name:                "Pressure Difference between Diesel Particulate Filter (DPF) Inlet and Outlet",
		Description:         "P187",
		CapabilityByteIndex: 70,
		CapabilityBitIndex:  6,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x76},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.KPA}
		},
	},
	"P188": {
		Id:                  "P188",
		Name:                "Exhaust Gas Temperature at Catalyst Inlet",
		Description:         "P188",
		CapabilityByteIndex: 70,
		CapabilityBitIndex:  5,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x77},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0])*5 - 40, units.C}
		},
	},
	"P189": {
		Id:                  "P189",
		Name:                "Exhaust Gas Temperature at Diesel Particulate Filter (DPF) Inlet",
		Description:         "P189",
		CapabilityByteIndex: 70,
		CapabilityBitIndex:  4,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x78},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0])*5 - 40, units.C}
		},
	},
	"P190": {
		Id:                  "P190",
		Name:                "Estimated Catalyst Temperature",
		Description:         "P190",
		CapabilityByteIndex: 70,
		CapabilityBitIndex:  3,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x79},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0])*5 - 40, units.C}
		},
	},
	"P191": {
		Id:                  "P191",
		Name:                "Estimated Temperature of the Diesel Particulate Filter (DPF)",
		Description:         "P191",
		CapabilityByteIndex: 70,
		CapabilityBitIndex:  2,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x7a},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0])*5 - 40, units.C}
		},
	},
	"P192": {
		Id:                  "P192",
		Name:                "Soot Accumulation Ratio",
		Description:         "P192",
		CapabilityByteIndex: 70,
		CapabilityBitIndex:  1,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x7b},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.Percent}
		},
	},
	"P193": {
		Id:                  "P193",
		Name:                "Oil Dilution Ratio",
		Description:         "P193",
		CapabilityByteIndex: 70,
		CapabilityBitIndex:  0,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x7c},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.Percent}
		},
	},
	"P194": {
		Id:                  "P194",
		Name:                "Front-Rear Wheel Rotation Ratio",
		Description:         "P194",
		CapabilityByteIndex: 71,
		CapabilityBitIndex:  7,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x93},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 128, units.Percent}
		},
	},
	"P195": {
		Id:                  "P195",
		Name:                "ABS/VDC Front Wheel Mean Wheel Speed",
		Description:         "P195",
		CapabilityByteIndex: 71,
		CapabilityBitIndex:  6,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x94},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 143 / 255, units.MPH}
		},
	},
	"P196": {
		Id:                  "P196",
		Name:                "ABS/VDC Rear Wheel Mean Wheel Speed",
		Description:         "P196",
		CapabilityByteIndex: 71,
		CapabilityBitIndex:  5,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x95},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 143 / 255, units.MPH}
		},
	},
	"P197": {
		Id:                  "P197",
		Name:                "Automatic Transmission Fluid (ATF) Deterioration Degree",
		Description:         "P197",
		CapabilityByteIndex: 71,
		CapabilityBitIndex:  4,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x96},
			Length:  2,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(binary.BigEndian.Uint32(v)) * 40 / 13107, units.Percent}
		},
	},
	"P198": {
		Id:                  "P198",
		Name:                "Accumulated Count of Overspeed Instances (Very High RPM)",
		Description:         "P198",
		CapabilityByteIndex: 72,
		CapabilityBitIndex:  7,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x98},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.Time}
		},
	},
	"P199": {
		Id:                  "P199",
		Name:                "Accumulated Count of Overspeed Instances (High RPM)",
		Description:         "P199",
		CapabilityByteIndex: 72,
		CapabilityBitIndex:  6,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x99},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.Time}
		},
	},
	"P204": {
		Id:                  "P204",
		Name:                "Actual Common Rail Pressure (Time Synchronized)",
		Description:         "P204",
		CapabilityByteIndex: 72,
		CapabilityBitIndex:  5,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x1f},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.MPA}
		},
	},
	"P205": {
		Id:                  "P205",
		Name:                "Estimated Distance to Oil Change",
		Description:         "P205",
		CapabilityByteIndex: 72,
		CapabilityBitIndex:  4,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x9a},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 62, units.Miles}
		},
	},
	"P206": {
		Id:                  "P206",
		Name:                "Running Distance since last Diesel Particulate Filter (DPF) Regeneration",
		Description:         "P206",
		CapabilityByteIndex: 72,
		CapabilityBitIndex:  3,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x9b},
			Length:  2,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(binary.BigEndian.Uint32(v)), units.Kilometers}
		},
	},
	"P207": {
		Id:                  "P207",
		Name:                "Diesel Particulate Filter (DPF) Regeneration Count",
		Description:         "P207",
		CapabilityByteIndex: 72,
		CapabilityBitIndex:  2,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x9d},
			Length:  2,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(binary.BigEndian.Uint32(v)), units.Times}
		},
	},
	"P208": {
		Id:                  "P208",
		Name:                "Micro-Quantity-Injection Final Learning Value 1-1",
		Description:         "P208-Injector learning value for idling for PL-CYL, where PL = common rail pressure level, CYL = cylinder number.",
		CapabilityByteIndex: 72,
		CapabilityBitIndex:  1,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x3d},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) * 5, units.US}
		},
	},
	"P209": {
		Id:                  "P209",
		Name:                "Micro-Quantity-Injection Final Learning Value 1-2",
		Description:         "P209-Injector learning value for idling for PL-CYL, where PL = common rail pressure level, CYL = cylinder number.",
		CapabilityByteIndex: 72,
		CapabilityBitIndex:  0,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x3e},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) * 5, units.US}
		},
	},
	"P210": {
		Id:                  "P210",
		Name:                "Micro-Quantity-Injection Final Learning Value 1-3",
		Description:         "P210-Injector learning value for idling for PL-CYL, where PL = common rail pressure level, CYL = cylinder number.",
		CapabilityByteIndex: 73,
		CapabilityBitIndex:  7,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x3f},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) * 5, units.US}
		},
	},
	"P211": {
		Id:                  "P211",
		Name:                "Micro-Quantity-Injection Final Learning Value 1-4",
		Description:         "P211-Injector learning value for idling for PL-CYL, where PL = common rail pressure level, CYL = cylinder number.",
		CapabilityByteIndex: 73,
		CapabilityBitIndex:  6,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x40},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) * 5, units.US}
		},
	},
	"P212": {
		Id:                  "P212",
		Name:                "Micro-Quantity-Injection Final Learning Value 2-1",
		Description:         "P212-Injector learning value for idling for PL-CYL, where PL = common rail pressure level, CYL = cylinder number.",
		CapabilityByteIndex: 73,
		CapabilityBitIndex:  5,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x41},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) * 5, units.US}
		},
	},
	"P213": {
		Id:                  "P213",
		Name:                "Micro-Quantity-Injection Final Learning Value 2-2",
		Description:         "P213-Injector learning value for idling for PL-CYL, where PL = common rail pressure level, CYL = cylinder number.",
		CapabilityByteIndex: 73,
		CapabilityBitIndex:  4,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x42},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) * 5, units.US}
		},
	},
	"P214": {
		Id:                  "P214",
		Name:                "Micro-Quantity-Injection Final Learning Value 2-3",
		Description:         "P214-Injector learning value for idling for PL-CYL, where PL = common rail pressure level, CYL = cylinder number.",
		CapabilityByteIndex: 73,
		CapabilityBitIndex:  3,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x43},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) * 5, units.US}
		},
	},
	"P215": {
		Id:                  "P215",
		Name:                "Micro-Quantity-Injection Final Learning Value 2-4",
		Description:         "P215-Injector learning value for idling for PL-CYL, where PL = common rail pressure level, CYL = cylinder number.",
		CapabilityByteIndex: 73,
		CapabilityBitIndex:  2,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x44},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) * 5, units.US}
		},
	},
	"P216": {
		Id:                  "P216",
		Name:                "Micro-Quantity-Injection Final Learning Value 3-1",
		Description:         "P216-Injector learning value for idling for PL-CYL, where PL = common rail pressure level, CYL = cylinder number.",
		CapabilityByteIndex: 73,
		CapabilityBitIndex:  1,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x45},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) * 5, units.US}
		},
	},
	"P217": {
		Id:                  "P217",
		Name:                "Micro-Quantity-Injection Final Learning Value 3-2",
		Description:         "P217-Injector learning value for idling for PL-CYL, where PL = common rail pressure level, CYL = cylinder number.",
		CapabilityByteIndex: 73,
		CapabilityBitIndex:  0,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x46},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) * 5, units.US}
		},
	},
	"P218": {
		Id:                  "P218",
		Name:                "Micro-Quantity-Injection Final Learning Value 3-3",
		Description:         "P218-Injector learning value for idling for PL-CYL, where PL = common rail pressure level, CYL = cylinder number.",
		CapabilityByteIndex: 74,
		CapabilityBitIndex:  7,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x47},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) * 5, units.US}
		},
	},
	"P219": {
		Id:                  "P219",
		Name:                "Micro-Quantity-Injection Final Learning Value 3-4",
		Description:         "P219-Injector learning value for idling for PL-CYL, where PL = common rail pressure level, CYL = cylinder number.",
		CapabilityByteIndex: 74,
		CapabilityBitIndex:  6,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x48},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) * 5, units.US}
		},
	},
	"P220": {
		Id:                  "P220",
		Name:                "Micro-Quantity-Injection Final Learning Value 4-1",
		Description:         "P220-Injector learning value for idling for PL-CYL, where PL = common rail pressure level, CYL = cylinder number.",
		CapabilityByteIndex: 74,
		CapabilityBitIndex:  5,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x49},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) * 5, units.US}
		},
	},
	"P221": {
		Id:                  "P221",
		Name:                "Micro-Quantity-Injection Final Learning Value 4-2",
		Description:         "P221-Injector learning value for idling for PL-CYL, where PL = common rail pressure level, CYL = cylinder number.",
		CapabilityByteIndex: 74,
		CapabilityBitIndex:  4,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x4a},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) * 5, units.US}
		},
	},
	"P222": {
		Id:                  "P222",
		Name:                "Micro-Quantity-Injection Final Learning Value 4-3",
		Description:         "P222-Injector learning value for idling for PL-CYL, where PL = common rail pressure level, CYL = cylinder number.",
		CapabilityByteIndex: 74,
		CapabilityBitIndex:  3,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x4b},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) * 5, units.US}
		},
	},
	"P223": {
		Id:                  "P223",
		Name:                "Micro-Quantity-Injection Final Learning Value 4-4",
		Description:         "P223-Injector learning value for idling for PL-CYL, where PL = common rail pressure level, CYL = cylinder number.",
		CapabilityByteIndex: 74,
		CapabilityBitIndex:  2,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x4c},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) * 5, units.US}
		},
	},
	"P224": {
		Id:                  "P224",
		Name:                "Micro-Quantity-Injection Final Learning Value 5-1",
		Description:         "P224-Injector learning value for idling for PL-CYL, where PL = common rail pressure level, CYL = cylinder number.",
		CapabilityByteIndex: 74,
		CapabilityBitIndex:  1,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x4d},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) * 5, units.US}
		},
	},
	"P225": {
		Id:                  "P225",
		Name:                "Micro-Quantity-Injection Final Learning Value 5-2",
		Description:         "P225-Injector learning value for idling for PL-CYL, where PL = common rail pressure level, CYL = cylinder number.",
		CapabilityByteIndex: 74,
		CapabilityBitIndex:  0,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x4e},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) * 5, units.US}
		},
	},
	"P226": {
		Id:                  "P226",
		Name:                "Micro-Quantity-Injection Final Learning Value 5-3",
		Description:         "P226-Injector learning value for idling for PL-CYL, where PL = common rail pressure level, CYL = cylinder number.",
		CapabilityByteIndex: 76,
		CapabilityBitIndex:  7,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x4f},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) * 5, units.US}
		},
	},
	"P227": {
		Id:                  "P227",
		Name:                "Micro-Quantity-Injection Final Learning Value 5-4",
		Description:         "P227-Injector learning value for idling for PL-CYL, where PL = common rail pressure level, CYL = cylinder number.",
		CapabilityByteIndex: 76,
		CapabilityBitIndex:  6,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x50},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) * 5, units.US}
		},
	},
	"P228": {
		Id:                  "P228",
		Name:                "Individual Pump Difference Learning Value",
		Description:         "P228",
		CapabilityByteIndex: 76,
		CapabilityBitIndex:  5,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x38},
			Length:  2,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(binary.BigEndian.Uint32(v)) - 1000, units.Milliamps}
		},
	},
	"P229": {
		Id:                  "P229",
		Name:                "Final Main Injection Period",
		Description:         "P229",
		CapabilityByteIndex: 76,
		CapabilityBitIndex:  4,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x57},
			Length:  2,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(binary.BigEndian.Uint32(v)), units.US}
		},
	},
	"P233": {
		Id:                  "P233",
		Name:                "Pre-Injection Final Period",
		Description:         "P233-Diesel parameter",
		CapabilityByteIndex: 60,
		CapabilityBitIndex:  6,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x55},
			Length:  2,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(binary.BigEndian.Uint32(v)), units.US}
		},
	},
	"P234": {
		Id:                  "P234",
		Name:                "Pre-Injection Amount",
		Description:         "P234-Volume",
		CapabilityByteIndex: 60,
		CapabilityBitIndex:  6,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x2f},
			Length:  2,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(binary.BigEndian.Uint32(v))/256 - 30, units.MM3PerStroke}
		},
	},
	"P235": {
		Id:                  "P235",
		Name:                "Pre-Injection Interval",
		Description:         "P235-Start of injection of pre-injection",
		CapabilityByteIndex: 60,
		CapabilityBitIndex:  6,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x31},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 50, units.DegreesCrankAngle}
		},
	},
	"P236": {
		Id:                  "P236",
		Name:                "Cumulative oil diesel entry",
		Description:         "P236-Cumulative amount of diesel fuel in the engine oil",
		CapabilityByteIndex: 60,
		CapabilityBitIndex:  6,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0xa2},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 5, units.Grams}
		},
	},
	"P238": {
		Id:                  "P238",
		Name:                "Final Initial Torque",
		Description:         "P238-Final initial torque including all limiters",
		CapabilityByteIndex: 60,
		CapabilityBitIndex:  6,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x2, 0x32},
			Length:  2,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(binary.BigEndian.Uint32(v)) - 50, units.Nm}
		},
	},
	"P239": {
		Id:                  "P239",
		Name:                "Global Timing User Adjustment Value",
		Description:         "P239-This is the fixed amount of timing removed globally - set by the user",
		CapabilityByteIndex: 8,
		CapabilityBitIndex:  0,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x6f},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{0 - float32(v[0]), units.Degress}
		},
	},
	"P240": {
		Id:                  "P240",
		Name:                "Engine Idle Speed User Adjustment (A/C off)",
		Description:         "P240-This is the fixed amount of idle speed adjustmnet while the A/C is off - set by the user",
		CapabilityByteIndex: 8,
		CapabilityBitIndex:  0,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x70},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0])*25 - 3200, units.RPM}
		},
	},
	"P241": {
		Id:                  "P241",
		Name:                "Engine Idle Speed User Adjustment (A/C on)",
		Description:         "P241-This is the fixed amount of idle speed adjustmnet while the A/C is on - set by the user",
		CapabilityByteIndex: 8,
		CapabilityBitIndex:  0,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x0, 0x71},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0])*25 - 3200, units.RPM}
		},
	},
	"P244": {
		Id:                  "P244",
		Name:                "Secondary Air Piping Pressure",
		Description:         "P244",
		CapabilityByteIndex: 41,
		CapabilityBitIndex:  7,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x8},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), units.KPA}
		},
	},
	"P245": {
		Id:                  "P245",
		Name:                "Secondary Air Flow",
		Description:         "P245",
		CapabilityByteIndex: 41,
		CapabilityBitIndex:  6,
		Address: &ParameterAddress{
			Address: [3]byte{0x0, 0x1, 0x82},
			Length:  2,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(binary.BigEndian.Uint32(v)) / 100, units.GS}
		},
	},
}

var DerivedParameters = map[string]DerivedParameter{
	"P200": {
		Id:                  "P200",
		Name:                "Engine Load (Calculated)",
		Description:         "P200-Engine load as calculated from MAF and RPM.",
		DependsOnParameters: []string{"P8", "P12"},
		Value: func(params map[string]ParameterValue) (*ParameterValue, error) {
			return &ParameterValue{((params["P12"].Value) * 60) / (params["P8"].Value), units.GramsPerRev}, nil
		},
	},
	"P201": {
		Id:                  "P201",
		Name:                "Injector Duty Cycle",
		Description:         "P201-IDC as calculated from RPM and injector PW.",
		DependsOnParameters: []string{"P8", "P21"},
		Value: func(params map[string]ParameterValue) (*ParameterValue, error) {
			return &ParameterValue{((params["P8"].Value) * (params["P21"].SafeConvertTo(units.US).Value)) / 1200, units.Percent}, nil
		},
	},
	"P202": {
		Id:                  "P202",
		Name:                "Manifold Relative Pressure (Corrected)",
		Description:         "P202-Difference between Manifold Absolute Pressure and Atmospheric Pressure.",
		DependsOnParameters: []string{"P7", "P24"},
		Value: func(params map[string]ParameterValue) (*ParameterValue, error) {
			return &ParameterValue{(params["P7"].SafeConvertTo(units.KPA).Value) - (params["P24"].SafeConvertTo(units.KPA).Value), units.PSI}, nil
		},
	},
	"P203": {
		Id:                  "P203",
		Name:                "Fuel Consumption (Est.)",
		Description:         "P203-Estimated fuel consumption based on MAF, AFR and vehicle speed.",
		DependsOnParameters: []string{"P9", "P12", "P58"},
		Value: func(params map[string]ParameterValue) (*ParameterValue, error) {
			return &ParameterValue{((params["P9"].SafeConvertTo(units.KMH).Value) / 3600) / (((params["P12"].Value) / (params["P58"].SafeConvertTo(units.Lambda).Value)) / 2880), units.MPGUS}, nil
		},
	},
	"P230": {
		Id:                  "P230",
		Name:                "Final Injection Amount (Fuel Temperature Corrected)",
		Description:         "P230",
		DependsOnParameters: []string{"P156", "P31"},
		Value: func(params map[string]ParameterValue) (*ParameterValue, error) {
			return &ParameterValue{(params["P156"].Value) * (835 - (0.7 * ((params["P31"].SafeConvertTo(units.C).Value) - 15))) / 1000, units.MGPerCylinder}, nil
		},
	},
	"P231": {
		Id:                  "P231",
		Name:                "Angle of Main Injection",
		Description:         "P231",
		DependsOnParameters: []string{"P229", "P8"},
		Value: func(params map[string]ParameterValue) (*ParameterValue, error) {
			return &ParameterValue{(params["P229"].SafeConvertTo(units.US).Value) / (2777.77 / ((params["P8"].Value) / 60)), units.DegreesCrankAngle}, nil
		},
	},
	"P232": {
		Id:                  "P232",
		Name:                "Lambda (Smoke Behaviour)",
		Description:         "P232",
		DependsOnParameters: []string{"P160", "P230"},
		Value: func(params map[string]ParameterValue) (*ParameterValue, error) {
			return &ParameterValue{((params["P160"].Value) / (params["P230"].Value)) / 14.7, units.Lambda}, nil
		},
	},
	"P237": {
		Id:                  "P237",
		Name:                "Air mass/charge pressure coefficient (TD)",
		Description:         "P237-Coefficient for determining the turbocharger efficiency",
		DependsOnParameters: []string{"P160", "P7"},
		Value: func(params map[string]ParameterValue) (*ParameterValue, error) {
			return &ParameterValue{(params["P160"].Value) / (params["P7"].SafeConvertTo(units.KPA).Value), units.Coefficient}, nil
		},
	},
	"P242": {
		Id:                  "P242",
		Name:                "Volumetric Efficiency 2.0L (Calculated)",
		Description:         "P242-VE calculated from IGL, MMA, MAF, IAT, absolute manifold pressure, assuming engine displacement of 122.04 CID (EJ207)",
		DependsOnParameters: []string{"P200", "P11", "P7"},
		Value: func(params map[string]ParameterValue) (*ParameterValue, error) {
			return &ParameterValue{((params["P200"].Value) * 2 * 8.314472 * ((params["P11"].SafeConvertTo(units.C).Value) + 273.15)) / ((params["P7"].SafeConvertTo(units.KPA).Value) * 2.0 * 28.97) * 100, units.Percent}, nil
		},
	},
	"P243": {
		Id:                  "P243",
		Name:                "Volumetric Efficiency 2.5L (Calculated)",
		Description:         "P243-VE calculated from IGL, MMA, MAF, IAT, absolute manifold pressure, assuming engine displacement of 149.9 CID (EJ257)",
		DependsOnParameters: []string{"P200", "P11", "P7"},
		Value: func(params map[string]ParameterValue) (*ParameterValue, error) {
			return &ParameterValue{((params["P200"].Value) * 2 * 8.314472 * ((params["P11"].SafeConvertTo(units.C).Value) + 273.15)) / ((params["P7"].SafeConvertTo(units.KPA).Value) * 2.5 * 28.97) * 100, units.Percent}, nil
		},
	},
}
