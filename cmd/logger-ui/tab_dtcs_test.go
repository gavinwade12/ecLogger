package main

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"github.com/gavinwade12/ecLogger/protocols/ssm2"
	"github.com/gavinwade12/ecLogger/units"
)

// dtcConnection answers read addresses requests as an ECU with the given set
// and stored DTCs, or with err. Reads block until release is closed when it's
// non-nil.
type dtcConnection struct {
	ssm2.Connection
	set, stored []ssm2.DTC
	err         error
	release     chan struct{}
}

func (c *dtcConnection) SendReadAddressesRequest(ctx context.Context, addresses [][3]byte, _ bool) (ssm2.Packet, error) {
	if c.release != nil {
		<-c.release
	}
	if c.err != nil {
		return nil, c.err
	}

	data := make([]byte, len(addresses))
	for i, addr := range addresses {
		for _, dtc := range c.set {
			if dtc.TempAddress.Address == addr {
				data[i] |= 1 << dtc.TempAddress.Bit
			}
		}
		for _, dtc := range c.stored {
			if dtc.Address.Address == addr {
				data[i] |= 1 << dtc.Address.Bit
			}
		}
	}
	return ssm2.NewPacket(ssm2.DeviceEngine, ssm2.DeviceDiagnosticTool, ssm2.CommandReadAddressesResponse, data), nil
}

// stubDTCs makes the connected ECU report the given DTCs.
func stubDTCs(h *testHarness, conn *dtcConnection) {
	h.do(func() {
		conn.Connection = h.connection
		h.connection = conn
	})
}

func (h *testHarness) refreshDTCs() {
	h.t.Helper()
	h.do(func() { test.Tap(h.DTCsTab.refreshBtn) })
	h.waitFor("the DTC refresh", func() bool { return !h.DTCsTab.refreshBtn.Disabled() })
}

func TestDTCRefreshShowsDTCs(t *testing.T) {
	h := newTestApp(t)
	h.connect()
	stubDTCs(h, &dtcConnection{set: []ssm2.DTC{ssm2.DTCs["D9"]}, stored: []ssm2.DTC{ssm2.DTCs["D1"], ssm2.DTCs["D10"]}})

	h.refreshDTCs()
	h.do(func() {
		// set DTCs come first, then stored, each sorted by name
		var got [][]string
		for _, o := range h.DTCsTab.list.Objects {
			var row []string
			for _, cell := range o.(*columnsRow).cells.Objects {
				row = append(row, cell.(*wrappedText).text)
			}
			got = append(got, row)
		}
		want := [][]string{
			{"P0102", "MAF SENSOR LOW INPUT", labelSet},
			{"P0103", "MAF SENSOR HIGH INPUT", labelStored},
			{"P0335", "CRANKSHAFT POS. SENSOR A MALFUNCTION", labelStored},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("DTC rows = %q, want %q", got, want)
		}
		if h.DTCsTab.status.Visible() {
			t.Errorf("unexpected status %q", h.DTCsTab.status.Text())
		}
	})
}

func TestDTCRefreshStatus(t *testing.T) {
	tests := map[string]struct {
		err       error
		want      string
		wantColor fyne.ThemeColorName
	}{
		"none":  {nil, "No DTCs found", colorNameSecondaryText},
		"error": {errors.New("boom"), "Couldn't read the DTCs: reading set DTCs: sending read addresses request: boom", theme.ColorNameError},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			h := newTestApp(t)
			h.connect()
			stubDTCs(h, &dtcConnection{err: tt.err})

			h.refreshDTCs()
			h.do(func() {
				if !h.DTCsTab.status.Visible() || h.DTCsTab.status.Text() != tt.want {
					t.Errorf("status = %q (visible %v), want %q",
						h.DTCsTab.status.Text(), h.DTCsTab.status.Visible(), tt.want)
				}
				if got := h.DTCsTab.status.segment.Style.ColorName; got != tt.wantColor {
					t.Errorf("status color = %q, want %q", got, tt.wantColor)
				}
			})
		})
	}
}

func TestDTCRefreshDoesNotBlockTheUI(t *testing.T) {
	h := newTestApp(t)
	h.connect()
	release := make(chan struct{})
	stubDTCs(h, &dtcConnection{release: release})

	// with the reads blocked, the tap must still return
	h.do(func() { test.Tap(h.DTCsTab.refreshBtn) })
	h.do(func() {
		if !h.DTCsTab.refreshBtn.Disabled() || h.DTCsTab.status.Text() != "Reading DTCs..." {
			t.Error("expected Refresh disabled while reading")
		}
	})
	close(release)
	h.waitFor("the DTC refresh", func() bool { return !h.DTCsTab.refreshBtn.Disabled() })
}

func TestDTCRefreshKeepsFileLogging(t *testing.T) {
	h := newTestApp(t)
	h.connect()
	h.do(func() {
		h.loggedParams.SetLogToFile("P8", false, units.RPM, true)
		h.LoggingTab.updateLiveLogParameters()
		h.LoggingTab.startFileLogging()
	})
	release := make(chan struct{})
	stubDTCs(h, &dtcConnection{release: release})

	h.do(func() {
		test.Tap(h.DTCsTab.refreshBtn)
		if h.LoggingTab.runner.Running() {
			t.Error("expected the logging session to be paused while reading")
		}
	})
	close(release)
	h.waitFor("the DTC refresh", func() bool { return !h.DTCsTab.refreshBtn.Disabled() })

	h.do(func() {
		if h.LoggingTab.logFile == nil {
			t.Error("refreshing DTCs stopped file logging")
		}
		if !h.LoggingTab.runner.Running() {
			t.Error("expected the logging session to resume")
		}
	})
}

func TestParameterChangesDuringDTCRefresh(t *testing.T) {
	h := newTestApp(t)
	h.connect()
	release := make(chan struct{})
	stubDTCs(h, &dtcConnection{release: release})

	h.do(func() {
		test.Tap(h.DTCsTab.refreshBtn)
		// live logging a parameter mustn't start a session mid-read
		h.loggedParams.SetLiveLog("P8", false, units.RPM, true)
		h.LoggingTab.updateLiveLogParameters()
		if h.LoggingTab.runner.Running() {
			t.Error("a logging session started while reading DTCs")
		}
	})
	close(release)
	h.waitFor("the logging session to start", func() bool { return h.LoggingTab.runner.Running() })
}

func TestSplitDTCName(t *testing.T) {
	for name, want := range map[string][2]string{
		"P0102 MAF SENSOR LOW INPUT": {"P0102", "MAF SENSOR LOW INPUT"},
		"MAF SENSOR LOW INPUT":       {"", "MAF SENSOR LOW INPUT"},
		"P0102":                      {"", "P0102"},
	} {
		if code, description := splitDTCName(name); code != want[0] || description != want[1] {
			t.Errorf("splitDTCName(%q) = %q, %q, want %q, %q", name, code, description, want[0], want[1])
		}
	}
}
