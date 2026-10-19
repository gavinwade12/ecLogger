package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gavinwade12/ecLogger/protocols/ssm2"
	"github.com/gavinwade12/ecLogger/units"
)

func testECU(t *testing.T) *ssm2.ECU {
	t.Helper()
	ecu, err := ssm2.NewFakeConnection(time.Millisecond).InitECU(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return ecu
}

func TestSelectParams(t *testing.T) {
	ecu := testECU(t)
	ecu.SupportedParameters = []ssm2.Parameter{ssm2.Parameters["P8"], ssm2.Parameters["P9"], ssm2.Parameters["P12"]}
	ecu.SupportedDerivedParameters = ssm2.AvailableDerivedParameters(ecu.SupportedParameters)

	params, derived, unitFor, err := selectParams([]loggedParameter{
		{Id: "P8"},
		{Id: "P9", Unit: units.MPH},
		{Id: "P7", Unit: units.KPA},    // unsupported
		{Id: "P9999", Unit: units.KPA}, // unknown
		{Id: "P200", Derived: true},
	}, ecu)
	if err != nil {
		t.Fatal(err)
	}
	if len(params) != 2 || params[0].Id != "P8" || params[1].Id != "P9" {
		t.Errorf("params = %v, want [P8 P9]", params)
	}
	if len(derived) != 1 || derived[0].Id != "P200" {
		t.Errorf("derived = %v, want [P200]", derived)
	}
	want := map[string]units.Unit{"P8": units.RPM, "P9": units.MPH, "P200": units.GramsPerRev}
	if len(unitFor) != len(want) {
		t.Errorf("unitFor = %v, want %v", unitFor, want)
	}
	for id, u := range want {
		if unitFor[id] != u {
			t.Errorf("unitFor[%s] = %s, want %s", id, unitFor[id], u)
		}
	}

	if _, _, _, err := selectParams([]loggedParameter{{Id: "P9", Unit: units.PSI}}, ecu); err == nil {
		t.Error("expected an error for an unconvertible unit")
	}
	if _, _, _, err := selectParams([]loggedParameter{{Id: "P7"}}, ecu); err == nil {
		t.Error("expected an error when nothing is supported")
	}
}

// syncBuffer is a bytes.Buffer that's safe to write while it's read.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestLogToCSV(t *testing.T) {
	params, derived, unitFor, err := selectParams([]loggedParameter{
		{Id: "P8"},              // 2 bytes
		{Id: "P9", Unit: "mph"}, // converted
		{Id: "P12"},             // 2 bytes
		{Id: "P200", Derived: true},
	}, testECU(t))
	if err != nil {
		t.Fatal(err)
	}

	var out syncBuffer
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	err = logToCSV(ctx, ssm2.NewFakeConnection(time.Millisecond), &out, params, derived, unitFor, nil)
	if err != nil {
		t.Fatalf("logToCSV() = %v, want nil once canceled", err)
	}

	rows, err := csv.NewReader(bytes.NewBufferString(out.String())).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 3 {
		t.Fatalf("got %d rows, want a header and at least 2 rows", len(rows))
	}
	wantHeader := []string{"Timestamp", "Engine Speed (rpm)", "Vehicle Speed (mph)", "Mass Airflow (g/s)",
		"Engine Load (Calculated) (g/rev)"}
	for i, h := range wantHeader {
		if rows[0][i] != h {
			t.Errorf("header[%d] = %q, want %q", i, rows[0][i], h)
		}
	}
	for _, row := range rows[1:] {
		if len(row) != len(wantHeader) {
			t.Fatalf("row %v has %d columns, want %d", row, len(row), len(wantHeader))
		}
		for _, v := range row[1:] {
			if _, err := strconv.ParseFloat(v, 32); err != nil {
				t.Errorf("value %q in row %v isn't a number", v, row)
			}
		}
	}
}
