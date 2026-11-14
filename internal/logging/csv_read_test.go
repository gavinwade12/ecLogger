package logging_test

import (
	"bytes"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/gavinwade12/ecLogger/internal/logging"
	"github.com/gavinwade12/ecLogger/protocols/ssm2"
	"github.com/gavinwade12/ecLogger/units"
)

func TestReadCSVRoundTrip(t *testing.T) {
	params := []ssm2.Parameter{ssm2.Parameters["P8"], ssm2.Parameters["P9"]}
	derived := []ssm2.DerivedParameter{ssm2.DerivedParameters["P200"]} // name has brackets
	unitFor := func(id string, defaultUnit units.Unit) units.Unit { return defaultUnit }

	var buf bytes.Buffer
	w, err := logging.NewCSVWriter(&buf, params, derived, unitFor)
	if err != nil {
		t.Fatal(err)
	}
	t1 := time.Date(2026, 10, 18, 13, 4, 5, 500000000, time.Local)
	t2 := t1.Add(123456 * time.Microsecond)
	rows := []struct {
		t      time.Time
		values map[string]ssm2.ParameterValue
	}{
		{t1, map[string]ssm2.ParameterValue{
			"P8": {Value: 3000}, "P9": {Value: 88.5}, "P200": {Value: 1.23456}}},
		{t2, map[string]ssm2.ParameterValue{"P8": {Value: -1}}},
	}
	for _, r := range rows {
		if err := w.Write(r.t, r.values); err != nil {
			t.Fatal(err)
		}
	}

	l, err := logging.ReadCSV(&buf)
	if err != nil {
		t.Fatal(err)
	}

	wantNames := []string{params[0].Name, params[1].Name, derived[0].Name}
	wantUnits := []units.Unit{params[0].DefaultUnit, params[1].DefaultUnit, derived[0].DefaultUnit}
	if len(l.Columns) != 3 {
		t.Fatalf("got %d columns, want 3", len(l.Columns))
	}
	for i, c := range l.Columns {
		if c.Name != wantNames[i] || c.Unit != wantUnits[i] {
			t.Errorf("column %d = %q (%s), want %q (%s)", i, c.Name, c.Unit, wantNames[i], wantUnits[i])
		}
	}

	if len(l.Times) != 2 || !l.Times[0].Equal(t1) || !l.Times[1].Equal(t2) {
		t.Errorf("times = %v, want [%v %v]", l.Times, t1, t2)
	}

	nan := math.NaN()
	want := [][]float64{{3000, -1}, {88.5, nan}, {1.2346, nan}}
	for i, c := range l.Columns {
		if len(c.Values) != len(want[i]) {
			t.Fatalf("column %d has %d values, want %d", i, len(c.Values), len(want[i]))
		}
		for j, v := range c.Values {
			if w := want[i][j]; v != w && !(math.IsNaN(v) && math.IsNaN(w)) {
				t.Errorf("column %d row %d = %v, want %v", i, j, v, w)
			}
		}
	}
}

func TestReadCSVHeaders(t *testing.T) {
	tests := []struct {
		cell string
		name string
		unit units.Unit
	}{
		{"Engine Speed (rpm)", "Engine Speed", "rpm"},
		{"Engine Load (Relative) (%)", "Engine Load (Relative)", "%"},
		{"Oil Switching Solenoid Valve (OSV) Duty (Right) (%)", "Oil Switching Solenoid Valve (OSV) Duty (Right)", "%"},
		{"Throttle", "Throttle", ""},
		{"Engine Load (Relative)", "Engine Load", "Relative"}, // indistinguishable from a unit
		{"Odd(unit)", "Odd(unit)", ""},
		{"Empty ()", "Empty", ""},
	}
	for _, tt := range tests {
		l, err := logging.ReadCSV(strings.NewReader("Timestamp," + tt.cell + "\n"))
		if err != nil {
			t.Errorf("%q: %v", tt.cell, err)
			continue
		}
		if len(l.Columns) != 1 || l.Columns[0].Name != tt.name || l.Columns[0].Unit != tt.unit {
			t.Errorf("%q = %+v, want %q (%s)", tt.cell, l.Columns, tt.name, tt.unit)
		}
	}
}

func TestReadCSVHeaderOnly(t *testing.T) {
	l, err := logging.ReadCSV(strings.NewReader("Timestamp,A (x),B (y)\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Times) != 0 || len(l.Columns) != 2 || len(l.Columns[0].Values) != 0 {
		t.Errorf("unexpected log %+v", l)
	}
}

func TestReadCSVDropsTruncatedFinalLine(t *testing.T) {
	const good = "2026-10-18 13:04:05.5,1.0000,2.0000\n"
	for name, tail := range map[string]string{
		"short":         "2026-10-18 13:04:06,3.00",
		"cut timestamp": "2026-10-18 13:0",
		"bad number":    "2026-10-18 13:04:06,3.0000,2.x",
		"open quote":    "2026-10-18 13:04:06,\"3",
	} {
		l, err := logging.ReadCSV(strings.NewReader("Timestamp,A (x),B (y)\n" + good + tail))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if len(l.Times) != 1 || len(l.Columns[1].Values) != 1 {
			t.Errorf("%s: got %d rows, want 1", name, len(l.Times))
		}
	}
}

func TestReadCSVMalformedMiddleLine(t *testing.T) {
	const good = "2026-10-18 13:04:05.5,1.0000,2.0000\n"
	for name, bad := range map[string]string{
		"short":     "2026-10-18 13:04:06,3.0000\n",
		"timestamp": "yesterday,3.0000,4.0000\n",
		"number":    "2026-10-18 13:04:06,x,4.0000\n",
	} {
		_, err := logging.ReadCSV(strings.NewReader("Timestamp,A (x),B (y)\n" + good + bad + good))
		if err == nil || !strings.Contains(err.Error(), "line 3") {
			t.Errorf("%s: err = %v, want one naming line 3", name, err)
		}
	}
}

func TestReadCSVBadHeaders(t *testing.T) {
	for name, in := range map[string]string{
		"empty":         "",
		"not timestamp": "Time,A (x)\n1,2\n",
	} {
		if _, err := logging.ReadCSV(strings.NewReader(in)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}
