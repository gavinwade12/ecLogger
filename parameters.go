package ssm2

import (
	"encoding/binary"

	"github.com/pkg/errors"
)

// Parameter represents a parameter that can be read from the ECU.
type Parameter struct {
	Id          string
	Name        string
	Description string

	// CapabilityByteIndex points to the capability byte containing the parameter's flag.
	CapabilityByteIndex uint
	// CapabilityBitIndex is the index of the bit flag within the byte containing the parameter's flag.
	CapabilityBitIndex uint

	// Address is present when the parameter value is read from RAM instead of calculated
	Address *ParameterAddress

	// DependsOnParameters is present when the parameter's value is derives from other parameters' values
	// instead of read from RAM.
	DependsOnParameters []string

	Value func(v []byte) ParameterValue
}

// ParameterAddress describes the address(es) containing the value for the parameter
// with an optional bit for switch parameters.
type ParameterAddress struct {
	Address [3]byte
	Length  int   // used when the value takes more than 1 address e.g. a 32-bit value on a 16-bit ECU
	Bit     *uint // used for switches
}

// Unit provides common values for units used to describe a parameter's value.
type Unit string

// The valid units.
const (
	// Velocity
	UnitMPH Unit = "mph"
	UnitKMH Unit = "km/h"

	// Rotational Speed
	UnitRPM Unit = "rpm"

	// Timing
	UnitDegress = "degress"

	// Temperature
	UnitF Unit = "F"
	UnitC Unit = "C"

	// Pressure
	UnitPSI  Unit = "psi"
	UnitBAR  Unit = "bar"
	UnitKPA  Unit = "kPa"
	UnitHPA  Unit = "hPa"
	UnitInHG Unit = "inHg"
	UnitMmHG Unit = "mmHg"

	// Airflow
	UnitGS Unit = "g/s"

	// Electricity
	UnitVolts Unit = "V"

	// Time
	UnitMS Unit = "ms"
	UnitUS Unit = "µs"

	// Percentage
	UnitPercent Unit = "%"
)

// Parameter value stores a parameter's value with its current unit.
type ParameterValue struct {
	Value float32
	Unit  Unit
}

// ErrorInvalidConversion is returned when an invalid unit conversion attempt is made.
var ErrorInvalidConversion = errors.New("units are invalid for conversion")

// ConvertTo converts a parameter value from its current unit to the given unit.
// ErrorInvalidConversion is returned if the unit conversion is invalid.
func (v ParameterValue) ConvertTo(u Unit) (*ParameterValue, error) {
	conversions := UnitConversions[v.Unit]
	if conversions == nil {
		return nil, ErrorInvalidConversion
	}

	convert := conversions[u]
	if convert == nil {
		return nil, ErrorInvalidConversion
	}

	return &ParameterValue{Value: convert(v.Value), Unit: u}, nil
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
			Address: [3]byte{0x00, 0x00, 0x07},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 100 / 255, UnitPercent}
		},
	},
	"P2": {
		Id:                  "P2",
		Name:                "Coolant Temperature",
		Description:         "P2",
		CapabilityByteIndex: 8,
		CapabilityBitIndex:  6,
		Address: &ParameterAddress{
			Address: [3]byte{0x00, 0x00, 0x08},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) - 40, UnitC}
		},
	},
	"P3": {
		Id:                  "P3",
		Name:                "A/F Correction #1",
		Description:         "P3",
		CapabilityByteIndex: 8,
		CapabilityBitIndex:  5,
		Address: &ParameterAddress{
			Address: [3]byte{0x00, 0x00, 0x09},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) * 100 / 128, UnitPercent}
		},
	},
	"P4": {
		Id:                  "P4",
		Name:                "A/F Learning #1",
		Description:         "P4",
		CapabilityByteIndex: 8,
		CapabilityBitIndex:  4,
		Address: &ParameterAddress{
			Address: [3]byte{0x00, 0x00, 0x0A},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) * 100 / 128, UnitPercent}
		},
	},
	"P5": {
		Id:                  "P5",
		Name:                "A/F Correction #2",
		Description:         "P3",
		CapabilityByteIndex: 8,
		CapabilityBitIndex:  3,
		Address: &ParameterAddress{
			Address: [3]byte{0x00, 0x00, 0x0B},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) * 100 / 128, UnitPercent}
		},
	},
	"P6": {
		Id:                  "P6",
		Name:                "A/F Learning #2",
		Description:         "P4",
		CapabilityByteIndex: 8,
		CapabilityBitIndex:  2,
		Address: &ParameterAddress{
			Address: [3]byte{0x00, 0x00, 0x0C},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) * 100 / 128, UnitPercent}
		},
	},
	"P7": {
		Id:                  "P7",
		Name:                "Manifold Absolute Pressure",
		Description:         "P7",
		CapabilityByteIndex: 8,
		CapabilityBitIndex:  1,
		Address: &ParameterAddress{
			Address: [3]byte{0x00, 0x00, 0x0D},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), UnitKPA}
		},
	},
	"P8": {
		Id:                  "P8",
		Name:                "Engine Speed",
		Description:         "P8",
		CapabilityByteIndex: 8,
		CapabilityBitIndex:  0,
		Address: &ParameterAddress{
			Address: [3]byte{0x00, 0x00, 0x0E},
			Length:  2,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(binary.BigEndian.Uint32(v)) / 4, UnitRPM}
		},
	},
	"P9": {
		Id:                  "P9",
		Name:                "Vehicle Speed",
		Description:         "P9",
		CapabilityByteIndex: 9,
		CapabilityBitIndex:  7,
		Address: &ParameterAddress{
			Address: [3]byte{0x00, 0x00, 0x10},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), UnitKMH}
		},
	},
	"P10": {
		Id:                  "P10",
		Name:                "Ignition Total Timing",
		Description:         "P10",
		CapabilityByteIndex: 9,
		CapabilityBitIndex:  6,
		Address: &ParameterAddress{
			Address: [3]byte{0x00, 0x00, 0x11},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) / 2, UnitDegress}
		},
	},
	"P11": {
		Id:                  "P11",
		Name:                "Intake Air Temperature",
		Description:         "P11",
		CapabilityByteIndex: 9,
		CapabilityBitIndex:  5,
		Address: &ParameterAddress{
			Address: [3]byte{0x00, 0x00, 0x12},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) - 40, UnitC}
		},
	},
	"P12": {
		Id:                  "P12",
		Name:                "Mass Airflow",
		Description:         "P12",
		CapabilityByteIndex: 9,
		CapabilityBitIndex:  4,
		Address: &ParameterAddress{
			Address: [3]byte{0x00, 0x00, 0x13},
			Length:  2,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(binary.BigEndian.Uint32(v)) / 100, UnitGS}
		},
	},
	"P13": {
		Id:                  "P13",
		Name:                "Throttle Opening Angle",
		Description:         "P13",
		CapabilityByteIndex: 9,
		CapabilityBitIndex:  3,
		Address: &ParameterAddress{
			Address: [3]byte{0x00, 0x00, 0x15},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 100 / 255, UnitPercent}
		},
	},
	"P14": {
		Id:                  "P14",
		Name:                "Front 02 Sensor #1",
		Description:         "P14",
		CapabilityByteIndex: 9,
		CapabilityBitIndex:  2,
		Address: &ParameterAddress{
			Address: [3]byte{0x00, 0x00, 0x16},
			Length:  2,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(binary.BigEndian.Uint32(v)) / 200, UnitVolts}
		},
	},
	"P15": {
		Id:                  "P15",
		Name:                "Rear 02 Sensor",
		Description:         "P15",
		CapabilityByteIndex: 9,
		CapabilityBitIndex:  1,
		Address: &ParameterAddress{
			Address: [3]byte{0x00, 0x00, 0x18},
			Length:  2,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(binary.BigEndian.Uint32(v)) / 200, UnitVolts}
		},
	},
	"P16": {
		Id:                  "P16",
		Name:                "Front 02 Sensor #2",
		Description:         "P16",
		CapabilityByteIndex: 9,
		CapabilityBitIndex:  0,
		Address: &ParameterAddress{
			Address: [3]byte{0x00, 0x00, 0x1A},
			Length:  2,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(binary.BigEndian.Uint32(v)) / 200, UnitVolts}
		},
	},
	"P17": {
		Id:                  "P17",
		Name:                "Battery Voltage",
		Description:         "P17",
		CapabilityByteIndex: 10,
		CapabilityBitIndex:  7,
		Address: &ParameterAddress{
			Address: [3]byte{0x00, 0x00, 0x1C},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 8 / 100, UnitVolts}
		},
	},
	"P18": {
		Id:                  "P18",
		Name:                "Mass Airflow Sensor Voltage",
		Description:         "P18",
		CapabilityByteIndex: 10,
		CapabilityBitIndex:  6,
		Address: &ParameterAddress{
			Address: [3]byte{0x00, 0x00, 0x1D},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 50, UnitVolts}
		},
	},
	"P19": {
		Id:                  "P19",
		Name:                "Throttle Sensor Voltage",
		Description:         "P19",
		CapabilityByteIndex: 10,
		CapabilityBitIndex:  5,
		Address: &ParameterAddress{
			Address: [3]byte{0x00, 0x00, 0x1E},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 50, UnitVolts}
		},
	},
	"P20": {
		Id:                  "P20",
		Name:                "Differential Pressure Sensor Voltage",
		Description:         "P20",
		CapabilityByteIndex: 10,
		CapabilityBitIndex:  4,
		Address: &ParameterAddress{
			Address: [3]byte{0x00, 0x00, 0x1F},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 50, UnitVolts}
		},
	},
	"P21": {
		Id:                  "P21",
		Name:                "Fuel Injector #1 Pulse Width",
		Description:         "P21",
		CapabilityByteIndex: 10,
		CapabilityBitIndex:  3,
		Address: &ParameterAddress{
			Address: [3]byte{0x00, 0x00, 0x20},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 256, UnitVolts}
		},
	},
	"P22": {
		Id:                  "P22",
		Name:                "Fuel Injector #2 Pulse Width",
		Description:         "P22",
		CapabilityByteIndex: 10,
		CapabilityBitIndex:  2,
		Address: &ParameterAddress{
			Address: [3]byte{0x00, 0x00, 0x21},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) * 256, UnitVolts}
		},
	},
	"P23": {
		Id:                  "P23",
		Name:                "Knock Correction Advance",
		Description:         "P23",
		CapabilityByteIndex: 10,
		CapabilityBitIndex:  1,
		Address: &ParameterAddress{
			Address: [3]byte{0x00, 0x00, 0x22},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) / 2, UnitDegress}
		},
	},
	"P24": {
		Id:                  "P24",
		Name:                "Atmospheric Pressure",
		Description:         "P24",
		CapabilityByteIndex: 10,
		CapabilityBitIndex:  0,
		Address: &ParameterAddress{
			Address: [3]byte{0x00, 0x00, 0x23},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]), UnitKPA}
		},
	},
	"P25": {
		Id:                  "P25",
		Name:                "Manifold Relative Pressure",
		Description:         "P25",
		CapabilityByteIndex: 11,
		CapabilityBitIndex:  7,
		Address: &ParameterAddress{
			Address: [3]byte{0x00, 0x00, 0x24},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) - 128, UnitKPA}
		},
	},
	"P26": {
		Id:                  "P26",
		Name:                "Pressure Differential Sensor",
		Description:         "P26",
		CapabilityByteIndex: 11,
		CapabilityBitIndex:  6,
		Address: &ParameterAddress{
			Address: [3]byte{0x00, 0x00, 0x25},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) - 128, UnitKPA}
		},
	},
	"P27": {
		Id:                  "P27",
		Name:                "Fuel Tank Pressure",
		Description:         "P27",
		CapabilityByteIndex: 11,
		CapabilityBitIndex:  5,
		Address: &ParameterAddress{
			Address: [3]byte{0x00, 0x00, 0x26},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{(float32(v[0]) - 128) / 40, UnitKPA}
		},
	},
	"P28": {
		Id:                  "P28",
		Name:                "CO Adjustment",
		Description:         "P28",
		CapabilityByteIndex: 11,
		CapabilityBitIndex:  4,
		Address: &ParameterAddress{
			Address: [3]byte{0x00, 0x00, 0x27},
			Length:  1,
		},
		Value: func(v []byte) ParameterValue {
			return ParameterValue{float32(v[0]) / 50, UnitVolts}
		},
	},
}

// UnitConversions provides conversion functions for the package-defined Units.
var UnitConversions = map[Unit]map[Unit]func(v float32) float32{
	UnitMPH: {
		UnitKMH: func(v float32) float32 {
			return v * 1.60934
		},
	},
	UnitKMH: {
		UnitMPH: func(v float32) float32 {
			return v * 0.621371
		},
	},
	UnitF: {
		UnitC: func(v float32) float32 {
			return (v - 32) / 9 * 5
		},
	},
	UnitC: {
		UnitF: func(v float32) float32 {
			return (v / 5 * 9) + 32
		},
	},
	UnitKPA: {
		UnitPSI: func(v float32) float32 {
			return v * 37 / 255
		},
		UnitBAR: func(v float32) float32 {
			return v / 100
		},
		UnitHPA: func(v float32) float32 {
			return v * 10
		},
		UnitInHG: func(v float32) float32 {
			return v * 0.2953
		},
		UnitMmHG: func(v float32) float32 {
			return v * 7.5
		},
	},
	UnitPSI: {
		UnitKPA: func(v float32) float32 {
			return v * 255 / 37
		},
		UnitBAR: func(v float32) float32 {
			return v * 0.0689475729
		},
		UnitHPA: func(v float32) float32 {
			return v * 2550 / 37
		},
		UnitInHG: func(v float32) float32 {
			return v * 2.03602
		},
		UnitMmHG: func(v float32) float32 {
			return v * 51.7149
		},
	},
	UnitBAR: {
		UnitPSI: func(v float32) float32 {
			return v * 14.5038
		},
		UnitKPA: func(v float32) float32 {
			return v * 100
		},
		UnitHPA: func(v float32) float32 {
			return v * 1000
		},
		UnitInHG: func(v float32) float32 {
			return v * 29.53
		},
		UnitMmHG: func(v float32) float32 {
			return v * 750.062
		},
	},
	UnitHPA: {
		UnitPSI: func(v float32) float32 {
			return v * 0.0145038
		},
		UnitBAR: func(v float32) float32 {
			return v / 1000
		},
		UnitKPA: func(v float32) float32 {
			return v / 10
		},
		UnitInHG: func(v float32) float32 {
			return v * 0.029529983071445
		},
		UnitMmHG: func(v float32) float32 {
			return v * 0.75006157584566
		},
	},
	UnitInHG: {
		UnitPSI: func(v float32) float32 {
			return v * 0.491154
		},
		UnitBAR: func(v float32) float32 {
			return v * 0.0338639
		},
		UnitKPA: func(v float32) float32 {
			return v * 3.3863886666667
		},
		UnitHPA: func(v float32) float32 {
			return v * 33.863886666667
		},
		UnitMmHG: func(v float32) float32 {
			return v * 25.4
		},
	},
	UnitMmHG: {
		UnitPSI: func(v float32) float32 {
			return v * 0.0193368
		},
		UnitBAR: func(v float32) float32 {
			return v * 0.00133322
		},
		UnitKPA: func(v float32) float32 {
			return v * 0.13332239
		},
		UnitHPA: func(v float32) float32 {
			return v * 1.3332239
		},
		UnitInHG: func(v float32) float32 {
			return v * 0.0393701
		},
	},
}
