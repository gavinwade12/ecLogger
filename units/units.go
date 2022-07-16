package units

import "errors"

// Unit provides common values for units used to describe a parameter's value.
type Unit string

// The valid units.
const (
	// Velocity
	MPH Unit = "mph"
	KMH Unit = "km/h"

	// Distance
	Miles      Unit = "miles"
	Kilometers Unit = "km"

	// Rotational Speed
	RPM Unit = "rpm"

	// Timing
	Degress Unit = "degrees"

	// Temperature
	F Unit = "F"
	C Unit = "C"

	// Pressure
	PSI  Unit = "psi"
	BAR  Unit = "bar"
	KPA  Unit = "kPa"
	HPA  Unit = "hPa"
	MPA  Unit = "MPa"
	InHG Unit = "inHg"
	MmHG Unit = "mmHg"

	// Airflow
	GS Unit = "g/s"

	// Fueling
	AFR               Unit = "AFR"
	Lambda            Unit = "Lambda"
	DegreesCrankAngle Unit = "°CA"
	MM3PerStroke      Unit = "mm³/st"
	MGPerCylinder     Unit = "mg/cyl"

	// Fuel Efficiency
	MPGUS    Unit = "mpg (US)"
	MPGUK    Unit = "mpg (UK)"
	KMPerL   Unit = "km/l"
	LPer100K Unit = "l/100k"

	// Electricity
	Volts     Unit = "V"
	Amps      Unit = "A"
	Milliamps Unit = "mA"
	Ohms      Unit = "ohm"

	// Time
	Time Unit = "Time"
	MS   Unit = "ms"
	US   Unit = "µs"

	// Misc
	Percent                Unit = "%"
	Steps                  Unit = "steps"
	Gear                   Unit = "gear"
	Count                  Unit = "count"
	MisfireCount           Unit = "misfire count"
	Multiplier             Unit = "multiplier"
	Index                  Unit = "index"
	Raw                    Unit = "raw ecu value"
	DegreesPerSecond       Unit = "degrees/s"
	MetersPerSecondSquared Unit = "m/s²"
	GramsPerRev            Unit = "g/rev"
	Times                  Unit = "Times"
	Grams                  Unit = "g"
	Coefficient            Unit = "coefficient"
	Nm                     Unit = "Nm"
)

// ErrorInvalidConversion is returned when an invalid unit conversion attempt is made.
var ErrorInvalidConversion = errors.New("units are invalid for conversion")

func Convert(value float32, from, to Unit) (float32, error) {
	cvs := UnitConversions[from]
	if cvs == nil {
		return 0, ErrorInvalidConversion
	}

	cv := cvs[to]
	if cv == nil {
		return 0, ErrorInvalidConversion
	}

	return cv(value), nil
}

// UnitConversions provides conversion functions for the package-defined Units.
var UnitConversions = map[Unit]map[Unit]func(v float32) float32{
	MPH: {
		KMH: func(v float32) float32 {
			return v * 1.60934
		},
	},
	KMH: {
		MPH: func(v float32) float32 {
			return v * 0.621371
		},
	},
	F: {
		C: func(v float32) float32 {
			return (v - 32) / 9 * 5
		},
	},
	C: {
		F: func(v float32) float32 {
			return (v / 5 * 9) + 32
		},
	},
	KPA: {
		PSI: func(v float32) float32 {
			return v * 37 / 255
		},
		BAR: func(v float32) float32 {
			return v / 100
		},
		HPA: func(v float32) float32 {
			return v * 10
		},
		InHG: func(v float32) float32 {
			return v * 0.2953
		},
		MmHG: func(v float32) float32 {
			return v * 7.5
		},
	},
	PSI: {
		KPA: func(v float32) float32 {
			return v * 255 / 37
		},
		BAR: func(v float32) float32 {
			return v * 0.0689475729
		},
		HPA: func(v float32) float32 {
			return v * 2550 / 37
		},
		InHG: func(v float32) float32 {
			return v * 2.03602
		},
		MmHG: func(v float32) float32 {
			return v * 51.7149
		},
	},
	BAR: {
		PSI: func(v float32) float32 {
			return v * 14.5038
		},
		KPA: func(v float32) float32 {
			return v * 100
		},
		HPA: func(v float32) float32 {
			return v * 1000
		},
		InHG: func(v float32) float32 {
			return v * 29.53
		},
		MmHG: func(v float32) float32 {
			return v * 750.062
		},
	},
	HPA: {
		PSI: func(v float32) float32 {
			return v * 0.0145038
		},
		BAR: func(v float32) float32 {
			return v / 1000
		},
		KPA: func(v float32) float32 {
			return v / 10
		},
		InHG: func(v float32) float32 {
			return v * 0.029529983071445
		},
		MmHG: func(v float32) float32 {
			return v * 0.75006157584566
		},
	},
	InHG: {
		PSI: func(v float32) float32 {
			return v * 0.491154
		},
		BAR: func(v float32) float32 {
			return v * 0.0338639
		},
		KPA: func(v float32) float32 {
			return v * 3.3863886666667
		},
		HPA: func(v float32) float32 {
			return v * 33.863886666667
		},
		MmHG: func(v float32) float32 {
			return v * 25.4
		},
	},
	MmHG: {
		PSI: func(v float32) float32 {
			return v * 0.0193368
		},
		BAR: func(v float32) float32 {
			return v * 0.00133322
		},
		KPA: func(v float32) float32 {
			return v * 0.13332239
		},
		HPA: func(v float32) float32 {
			return v * 1.3332239
		},
		InHG: func(v float32) float32 {
			return v * 0.0393701
		},
	},
}
