package logging

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/gavinwade12/ecLogger/units"
)

// A Log is a log file read back: the time of each row, and each column's
// values. It is column-oriented so each parameter can be charted on its own.
type Log struct {
	Times   []time.Time
	Columns []Column
}

// A Column is one logged parameter. Values has one entry per row of the log,
// in the same order as Log.Times.
type Column struct {
	Name   string
	Unit   units.Unit
	Values []float64 // NaN where the cell was empty
}

// ReadCSV reads a log written by CSVWriter. It is the inverse of CSVWriter:
// timestamps are parsed as local time, and empty cells become NaN.
//
// The writer flushes every row, so a file read while it is still being logged
// can end in a partial line. A malformed final line is dropped; a malformed
// line anywhere else is an error.
func ReadCSV(r io.Reader) (*Log, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1 // checked below so the error can name the line
	cr.ReuseRecord = true

	header, err := cr.Read()
	if errors.Is(err, io.EOF) {
		return nil, errors.New("reading header: file is empty")
	}
	if err != nil {
		return nil, fmt.Errorf("reading header: %w", err)
	}
	if header[0] != "Timestamp" {
		return nil, fmt.Errorf("reading header: first column is %q, want \"Timestamp\"", header[0])
	}

	l := &Log{Columns: make([]Column, len(header)-1)}
	for i, cell := range header[1:] {
		l.Columns[i].Name, l.Columns[i].Unit = parseHeaderCell(cell)
	}

	// A bad row is only fatal if another row follows it, so hold on to the
	// error until we know whether it was the last one.
	var pending error
	for {
		record, err := cr.Read()
		if errors.Is(err, io.EOF) {
			return l, nil // any pending error was for the final line
		}
		if pending != nil {
			return nil, pending
		}
		if err != nil {
			pending = fmt.Errorf("reading row: %w", err)
			continue
		}

		line, _ := cr.FieldPos(0)
		t, values, err := parseRow(record, len(l.Columns))
		if err != nil {
			pending = fmt.Errorf("line %d: %w", line, err)
			continue
		}
		l.Times = append(l.Times, t)
		for i, v := range values {
			l.Columns[i].Values = append(l.Columns[i].Values, v)
		}
	}
}

// parseRow parses a record into its time and its n values.
func parseRow(record []string, n int) (time.Time, []float64, error) {
	if len(record) != n+1 {
		return time.Time{}, nil, fmt.Errorf("got %d fields, want %d", len(record), n+1)
	}
	t, err := time.ParseInLocation(csvTimestampFormat, record[0], time.Local)
	if err != nil {
		return time.Time{}, nil, fmt.Errorf("parsing timestamp: %w", err)
	}
	values := make([]float64, n)
	for i, cell := range record[1:] {
		if cell == "" {
			values[i] = math.NaN()
			continue
		}
		if values[i], err = strconv.ParseFloat(cell, 64); err != nil {
			return time.Time{}, nil, fmt.Errorf("parsing column %d: %w", i+1, err)
		}
	}
	return t, values, nil
}

// parseHeaderCell splits a "name (unit)" header cell. Names can contain
// brackets themselves, so the unit is the last bracketed group. A cell with no
// bracketed suffix is all name.
func parseHeaderCell(cell string) (string, units.Unit) {
	if !strings.HasSuffix(cell, ")") {
		return cell, ""
	}
	// walk back to the bracket matching the final one, in case a unit has
	// brackets of its own
	depth := 0
	for i := len(cell) - 1; i >= 0; i-- {
		switch cell[i] {
		case ')':
			depth++
		case '(':
			depth--
		}
		if depth == 0 {
			if i == 0 || cell[i-1] != ' ' {
				break
			}
			return cell[:i-1], units.Unit(cell[i+1 : len(cell)-1])
		}
	}
	return cell, ""
}
