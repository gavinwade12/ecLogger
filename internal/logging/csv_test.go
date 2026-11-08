package logging_test

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/gavinwade12/ecLogger/internal/logging"
	"github.com/gavinwade12/ecLogger/protocols/ssm2"
	"github.com/gavinwade12/ecLogger/units"
)

func TestFileName(t *testing.T) {
	now := time.Date(2026, 10, 18, 13, 4, 5, 0, time.UTC)
	romID := []byte{0x4a, 0x10, 0x52, 0x50, 0x07}

	tests := map[string]string{
		"ssm2_log_{{romId}}_{{timestamp}}.csv": "ssm2_log_4a10525007_20261018_130405.csv",
		"{{romID}}.csv":                        "4a10525007.csv",
		"plain.csv":                            "plain.csv",
	}
	for format, want := range tests {
		if got := logging.FileName(format, romID, now); got != want {
			t.Errorf("FileName(%q) = %q, want %q", format, got, want)
		}
	}
}

func TestCSVWriter(t *testing.T) {
	params := []ssm2.Parameter{ssm2.Parameters["P8"], ssm2.Parameters["P9"]}
	derived := []ssm2.DerivedParameter{ssm2.DerivedParameters["P200"]}
	unitFor := func(id string, defaultUnit units.Unit) units.Unit {
		if id == "P9" {
			return units.KMH
		}
		return defaultUnit
	}

	var buf bytes.Buffer
	w, err := logging.NewCSVWriter(&buf, params, derived, unitFor)
	if err != nil {
		t.Fatal(err)
	}

	ts := time.Date(2026, 10, 18, 13, 4, 5, 500000000, time.UTC)
	err = w.Write(ts, map[string]ssm2.ParameterValue{
		"P8":   {Value: 3000, Unit: units.RPM},
		"P9":   {Value: 88.5, Unit: units.KMH},
		"P200": {Value: 1.23456, Unit: units.GramsPerRev},
	})
	if err != nil {
		t.Fatal(err)
	}
	// a missing value leaves its column empty
	if err = w.Write(ts, map[string]ssm2.ParameterValue{"P8": {Value: 1}}); err != nil {
		t.Fatal(err)
	}

	want := "Timestamp,Engine Speed (rpm),Vehicle Speed (km/h),Engine Load (Calculated) (g/rev)\n" +
		"2026-10-18 13:04:05.5,3000.0000,88.5000,1.2346\n" +
		"2026-10-18 13:04:05.5,1.0000,,\n"
	if got := buf.String(); got != want {
		t.Errorf("unexpected CSV.\nwant:\n%s\ngot:\n%s", want, got)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

func TestCSVWriterReportsWriteErrors(t *testing.T) {
	_, err := logging.NewCSVWriter(failingWriter{}, []ssm2.Parameter{ssm2.Parameters["P8"]}, nil,
		func(_ string, u units.Unit) units.Unit { return u })
	if err == nil {
		t.Fatal("expected an error")
	}
}
