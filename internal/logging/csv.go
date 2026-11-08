package logging

import (
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gavinwade12/ecLogger/protocols/ssm2"
	"github.com/gavinwade12/ecLogger/units"
)

const (
	csvTimestampFormat  = "2006-01-02 15:04:05.999999999"
	fileTimestampFormat = "20060102_150405"
)

// FileName fills in the variables in a log file name format. The supported
// variables are {{romId}} (the hex-encoded ROM ID) and {{timestamp}}.
// {{romID}} is accepted as an alias for {{romId}}.
func FileName(format string, romID []byte, now time.Time) string {
	id := hex.EncodeToString(romID)
	return strings.NewReplacer(
		"{{romId}}", id,
		"{{romID}}", id,
		"{{timestamp}}", now.Format(fileTimestampFormat),
	).Replace(format)
}

// CSVWriter writes parameter values to CSV, one row per call to Write.
type CSVWriter struct {
	w   *csv.Writer
	ids []string
}

// NewCSVWriter writes a header row for the given parameters to w and returns a
// CSVWriter for writing their values. Columns are in the order given, read
// parameters first. unitFor returns the unit to show in each column header.
func NewCSVWriter(w io.Writer, params []ssm2.Parameter, derived []ssm2.DerivedParameter,
	unitFor func(id string, defaultUnit units.Unit) units.Unit) (*CSVWriter, error) {
	c := &CSVWriter{w: csv.NewWriter(w)}

	header := []string{"Timestamp"}
	addColumn := func(id, name string, defaultUnit units.Unit) {
		c.ids = append(c.ids, id)
		header = append(header, fmt.Sprintf("%s (%s)", name, unitFor(id, defaultUnit)))
	}
	for _, p := range params {
		addColumn(p.Id, p.Name, p.DefaultUnit)
	}
	for _, p := range derived {
		addColumn(p.Id, p.Name, p.DefaultUnit)
	}

	if err := c.writeRow(header); err != nil {
		return nil, fmt.Errorf("writing header: %w", err)
	}
	return c, nil
}

// Write writes a row with the given values. Missing values are left empty.
func (c *CSVWriter) Write(t time.Time, values map[string]ssm2.ParameterValue) error {
	row := make([]string, 0, len(c.ids)+1)
	row = append(row, t.Format(csvTimestampFormat))
	for _, id := range c.ids {
		v, ok := values[id]
		if !ok {
			row = append(row, "")
			continue
		}
		row = append(row, strconv.FormatFloat(float64(v.Value), 'f', 4, 32))
	}
	return c.writeRow(row)
}

// writeRow writes and flushes a row so the file is usable while logging.
func (c *CSVWriter) writeRow(row []string) error {
	if err := c.w.Write(row); err != nil {
		return err
	}
	c.w.Flush()
	return c.w.Error()
}
