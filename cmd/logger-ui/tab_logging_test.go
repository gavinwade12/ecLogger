package main_test

import (
	"bufio"
	"os"
	"path/filepath"
	"slices"
	"testing"

	ui "github.com/gavinwade12/ecLogger/cmd/logger-ui"
)

func TestEnableLoggingWithoutParams(t *testing.T) {
	ta := newTestApp(t)
	ta.connect()

	ta.do(ta.LoggingTab.EnableLogging)
	ta.assertLiveReadsStopped()

	ta.setLiveLog("Engine Speed", true)
	ta.waitForLiveReads()

	ta.setLiveLog("Engine Speed", false)
	ta.assertLiveReadsStopped()
	ta.do(ta.LoggingTab.EnableLogging)
	ta.assertLiveReadsStopped()
}

func TestFileLoggingOnlyIncludesFileParams(t *testing.T) {
	ta := newTestApp(t)
	ta.connect()
	logging := ui.TabLogging

	ta.setLiveLog("Engine Speed", true)
	if ta.buttonShown(logging, "Log To File") {
		t.Fatal("expected no file logging controls when nothing logs to file")
	}

	ta.setFileLog("Vehicle Speed", true)
	unit := ta.paramRow("Vehicle Speed").unit
	ta.do(func() { unit.SetSelected("mph") })
	if !ta.buttonShown(logging, "Log To File") {
		t.Fatal("expected the Log To File button once a parameter logs to file")
	}

	ta.tap(logging, "Log To File")
	if ta.buttonShown(logging, "Log To File") || !ta.buttonShown(logging, "Stop") {
		t.Error("expected Stop in place of Log To File while logging to file")
	}
	ta.do(func() {
		if !hasTextWithPrefix(ta.content(logging), "Logging to ") {
			t.Errorf("Logging tab shows %q, want the file being logged to", texts(ta.content(logging)))
		}
	})

	ta.tap(logging, "Stop")
	if !ta.buttonShown(logging, "Log To File") || ta.buttonShown(logging, "Stop") {
		t.Error("expected Log To File in place of Stop after stopping")
	}
	ta.do(func() {
		if hasTextWithPrefix(ta.content(logging), "Logging to ") {
			t.Error("expected no file status after stopping")
		}
	})

	// only the file parameters are in the file, in their configured units
	files, err := filepath.Glob(filepath.Join(ta.cfg.LogDirectory, "*"))
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

func TestFileLoggingShowsWhyItCouldntStart(t *testing.T) {
	ta := newTestApp(t)
	ta.connect()
	logging := ui.TabLogging

	// a log directory inside a file can't be created
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	ta.do(func() { ta.cfg.LogDirectory = filepath.Join(file, "logs") })

	ta.setFileLog("Vehicle Speed", true)
	ta.show(logging)
	ta.tap(logging, "Log To File")
	if ta.buttonShown(logging, "Stop") {
		t.Error("expected file logging not to start")
	}
	ta.do(func() {
		if !hasTextWithPrefix(ta.content(logging), "Couldn't log to file: creating the log directory") {
			t.Errorf("Logging tab shows %q, want why file logging couldn't start", texts(ta.content(logging)))
		}
	})

	ta.do(func() { ta.cfg.LogDirectory = t.TempDir() })
	ta.tap(logging, "Log To File")
	ta.do(func() {
		if !hasTextWithPrefix(ta.content(logging), "Logging to ") {
			t.Errorf("Logging tab shows %q after retrying, want the file being logged to", texts(ta.content(logging)))
		}
	})
	ta.tap(logging, "Stop")
}

func TestLiveLogExplainsWhenNothingIsLiveLogged(t *testing.T) {
	ta := newTestApp(t)
	ta.connect()
	const hint = "No parameters are live logged"

	ta.show(ui.TabLogging)
	ta.waitForText(ui.TabLogging, hint, true)

	ta.setLiveLog("Engine Speed", true)
	ta.show(ui.TabLogging)
	ta.waitForText(ui.TabLogging, hint, false)

	ta.setLiveLog("Engine Speed", false)
	ta.show(ui.TabLogging)
	ta.waitForText(ui.TabLogging, hint, true)

	ta.tap(ui.TabLogging, "Choose Parameters")
	ta.do(func() {
		if got := ta.tabs().SelectedIndex(); got != int(ui.TabParameters) {
			t.Errorf("selected tab = %d, want the Parameters tab", got)
		}
	})
	_ = slices.Contains[[]string]
}
