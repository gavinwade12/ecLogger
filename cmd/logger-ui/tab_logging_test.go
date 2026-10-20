package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/gavinwade12/ecLogger/internal/logging"
	"github.com/gavinwade12/ecLogger/protocols/ssm2"
	"github.com/gavinwade12/ecLogger/units"
)

func TestEnableLoggingWithoutParams(t *testing.T) {
	h := newTestApp(t)
	h.connect()

	h.do(func() {
		h.LoggingTab.EnableLogging()
		if h.LoggingTab.runner.Running() {
			t.Fatal("started a session with no logged parameters")
		}

		h.loggedParams.SetLiveLog("P8", false, units.RPM, true)
		h.LoggingTab.EnableLogging()
		if !h.LoggingTab.runner.Running() {
			t.Fatal("expected a session for the logged parameter")
		}

		h.loggedParams.SetLiveLog("P8", false, units.RPM, false)
		h.LoggingTab.EnableLogging()
		if h.LoggingTab.runner.Running() {
			t.Fatal("expected the session to stop once nothing is logged")
		}
	})
}

func TestFileLoggingOnlyIncludesFileParams(t *testing.T) {
	h := newTestApp(t)
	h.connect()

	h.do(func() {
		h.loggedParams.SetLiveLog("P8", false, units.RPM, true)
		h.LoggingTab.onLoggedParametersChanged()
		if h.LoggingTab.header.Visible() {
			t.Fatal("expected no file logging controls when nothing logs to file")
		}

		h.loggedParams.SetLogToFile("P9", false, units.MPH, true)
		h.LoggingTab.onLoggedParametersChanged()
		if !h.LoggingTab.header.Visible() || !h.LoggingTab.startBtn.Visible() {
			t.Fatal("expected the Log To File button once a parameter logs to file")
		}

		h.LoggingTab.updateLiveLogParameters()
		test.Tap(h.LoggingTab.startBtn)
		if h.LoggingTab.startBtn.Visible() || !h.LoggingTab.stopBtn.Visible() {
			t.Error("expected Stop in place of Log To File while logging to file")
		}
		if got := h.LoggingTab.fileStatus.Text; !strings.HasPrefix(got, "Logging to ") {
			t.Errorf("file status = %q, want the file being logged to", got)
		}

		test.Tap(h.LoggingTab.stopBtn)
		if !h.LoggingTab.startBtn.Visible() || h.LoggingTab.stopBtn.Visible() {
			t.Error("expected Log To File in place of Stop after stopping")
		}
		if got := h.LoggingTab.fileStatus.Text; got != "" {
			t.Errorf("file status = %q after stopping, want none", got)
		}
	})

	files, err := filepath.Glob(filepath.Join(h.config.LogDirectory, "*"))
	if err != nil || len(files) != 1 {
		t.Fatalf("log files = %v, %v; want one", files, err)
	}
	f, err := os.Open(files[0])
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	header, _ := bufio.NewReader(f).ReadString('\n')
	if want := "Timestamp,Vehicle Speed (mph)\n"; header != want {
		t.Errorf("header = %q, want %q", header, want)
	}
}

func TestLiveLogModelShowsPlaceholderUntilAValue(t *testing.T) {
	m := newLiveLogModel("P8", "Engine Speed")
	check := func(wantValue, wantUnit string) {
		t.Helper()
		for name, b := range map[string]interface{ Get() (string, error) }{
			"current": m.CurrentValueBinding, "min": m.MinValueBinding, "max": m.MaxValueBinding,
		} {
			if got, _ := b.Get(); got != wantValue {
				t.Errorf("%s = %q, want %q", name, got, wantValue)
			}
		}
		if got, _ := m.UnitBinding.Get(); got != wantUnit {
			t.Errorf("unit = %q, want %q", got, wantUnit)
		}
	}

	check(noValue, "")
	m.Update(ssm2.ParameterValue{Value: 800, Unit: units.RPM}, logging.Stats{Min: 800, Max: 800, Count: 1})
	check("800.00", string(units.RPM))
}

func TestFileLoggingShowsWhyItCouldntStart(t *testing.T) {
	h := newTestApp(t)
	h.connect()

	// a log directory inside a file can't be created
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	h.config.LogDirectory = filepath.Join(file, "logs")

	h.do(func() {
		h.loggedParams.SetLogToFile("P9", false, units.MPH, true)
		h.LoggingTab.onLoggedParametersChanged()
		test.Tap(h.LoggingTab.startBtn)

		if h.LoggingTab.logFile != nil || h.LoggingTab.stopBtn.Visible() {
			t.Error("expected file logging not to start")
		}
		if got := h.LoggingTab.fileStatus.Text; !strings.HasPrefix(got, "Couldn't log to file: creating the log directory") {
			t.Errorf("file status = %q, want why file logging couldn't start", got)
		}

		h.config.LogDirectory = t.TempDir()
		test.Tap(h.LoggingTab.startBtn)
		if got := h.LoggingTab.fileStatus.Text; !strings.HasPrefix(got, "Logging to ") {
			t.Errorf("file status = %q after retrying, want the file being logged to", got)
		}
		test.Tap(h.LoggingTab.stopBtn)
	})
}

func TestLiveLogExplainsWhenNothingIsLiveLogged(t *testing.T) {
	h := newTestApp(t)
	h.connect()

	h.do(func() {
		if !h.LoggingTab.empty.Visible() {
			t.Fatal("expected a hint when nothing is live logged")
		}

		h.loggedParams.SetLiveLog("P8", false, units.RPM, true)
		h.LoggingTab.updateLiveLogParameters()
		if h.LoggingTab.empty.Visible() {
			t.Error("expected no hint once a parameter is live logged")
		}

		h.loggedParams.SetLiveLog("P8", false, units.RPM, false)
		h.LoggingTab.updateLiveLogParameters()
		if !h.LoggingTab.empty.Visible() {
			t.Fatal("expected the hint once nothing is live logged again")
		}

		test.Tap(h.LoggingTab.chooseParamsBtn)
		if got := h.tabItems.SelectedIndex(); got != int(TabParameters) {
			t.Errorf("selected tab = %d, want the Parameters tab", got)
		}
	})
}
