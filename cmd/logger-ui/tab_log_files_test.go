package main_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	ui "github.com/gavinwade12/ecLogger/cmd/logger-ui"
)

// addLogFile writes a file of size bytes to the log directory, modified at
// age before now.
func (ta *testApp) addLogFile(name string, size int, age time.Duration) string {
	ta.t.Helper()
	path := filepath.Join(ta.cfg.LogDirectory, name)
	if err := os.WriteFile(path, []byte(strings.Repeat("x", size)), 0o644); err != nil {
		ta.t.Fatal(err)
	}
	mod := time.Now().Add(-age)
	if err := os.Chtimes(path, mod, mod); err != nil {
		ta.t.Fatal(err)
	}
	return path
}

// logFileButtons returns the buttons with the given text, in row order.
func (ta *testApp) logFileButtons(text string) (bs []*widget.Button) {
	ta.do(func() {
		bs = findAll(ta.content(ui.TabLogFiles), func(b *widget.Button) bool { return b.Text == text })
	})
	return bs
}

// logFileNames returns the file names the tab lists, in order.
func (ta *testApp) logFileNames() (names []string) {
	ta.do(func() {
		for _, s := range texts(ta.content(ui.TabLogFiles)) {
			if strings.HasSuffix(s, ".csv") {
				names = append(names, s)
			}
		}
	})
	return names
}

// tapDialog taps a button of the dialog that's shown.
func (ta *testApp) tapDialog(text string) {
	ta.t.Helper()
	ta.do(func() {
		top := ta.window.Canvas().Overlays().Top()
		if top == nil {
			ta.t.Fatal("no dialog is shown")
		}
		b := button(top, text)
		if b == nil {
			ta.t.Fatalf("the dialog has no %q button", text)
		}
		test.Tap(b)
	})
}

func TestLogFilesTabEnabledWhileDisconnected(t *testing.T) {
	ta := newTestApp(t)
	if !ta.tabEnabled(ui.TabLogFiles) {
		t.Fatal("the Log Files tab is disabled while disconnected")
	}
}

func TestLogFilesTabListsNewestFirst(t *testing.T) {
	ta := newTestApp(t)
	ta.addLogFile("old.csv", 10, 3*time.Hour)
	ta.addLogFile("new.csv", 1536, time.Hour)
	ta.addLogFile("mid.csv", 2*1024*1024, 2*time.Hour)
	ta.addLogFile("notes.txt", 5, time.Minute)
	if err := os.Mkdir(filepath.Join(ta.cfg.LogDirectory, "dir.csv"), 0o755); err != nil {
		t.Fatal(err)
	}
	ta.show(ui.TabLogFiles)

	if got, want := ta.logFileNames(), []string{"new.csv", "mid.csv", "old.csv"}; !slices.Equal(got, want) {
		t.Fatalf("files = %v, want %v", got, want)
	}
	shown := ta.texts(ui.TabLogFiles)
	for _, size := range []string{"1.5 KB", "2.0 MB", "10 B"} {
		if !slices.Contains(shown, size) {
			t.Errorf("size %q not shown: %v", size, shown)
		}
	}
}

func TestLogFilesTabRefreshesOnSelect(t *testing.T) {
	ta := newTestApp(t)
	ta.addLogFile("a.csv", 1, time.Hour)
	ta.show(ui.TabLogFiles)
	ta.show(ui.TabConnection)
	ta.addLogFile("b.csv", 1, time.Minute)
	ta.show(ui.TabLogFiles)
	if got, want := ta.logFileNames(), []string{"b.csv", "a.csv"}; !slices.Equal(got, want) {
		t.Fatalf("files = %v, want %v", got, want)
	}
}

func TestLogFilesTabGraphOpensFile(t *testing.T) {
	ta := newTestApp(t)
	src := writeLog(t, "run.csv", graphColumns, 5)
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ta.cfg.LogDirectory, "run.csv"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	ta.show(ui.TabLogFiles)
	ta.tap(ui.TabLogFiles, "Graph")

	ta.waitFor("the log to load", func() bool {
		return !hasTextWithPrefix(ta.content(ui.TabGraphing), "Loading")
	})
	var selected *ui.TabType
	ta.do(func() {
		for i, item := range ta.tabs().Items {
			if item == ta.tabs().Selected() {
				tab := ui.TabType(i)
				selected = &tab
			}
		}
	})
	if selected == nil || *selected != ui.TabGraphing {
		t.Errorf("selected tab = %v, want the Graphing tab", selected)
	}
	if !slices.Contains(ta.texts(ui.TabGraphing), "run.csv") {
		t.Errorf("file not open in the Graphing tab: %v", ta.texts(ui.TabGraphing))
	}
}

func TestLogFilesTabDelete(t *testing.T) {
	ta := newTestApp(t)
	ta.do(func() { ta.SetWindow(ta.window) })
	keep := ta.addLogFile("keep.csv", 1, time.Hour)
	gone := ta.addLogFile("gone.csv", 1, time.Minute)
	ta.show(ui.TabLogFiles)

	// cancelling keeps the file
	ta.tap(ui.TabLogFiles, "Delete")
	ta.do(func() {
		if !hasTextWithPrefix(ta.window.Canvas().Overlays().Top(), "Delete gone.csv?") {
			t.Errorf("no confirmation naming the file")
		}
	})
	ta.tapDialog("Cancel")
	if _, err := os.Stat(gone); err != nil {
		t.Fatalf("file removed after cancelling: %v", err)
	}

	ta.tap(ui.TabLogFiles, "Delete")
	ta.tapDialog("Delete")
	if _, err := os.Stat(gone); !os.IsNotExist(err) {
		t.Errorf("file still exists after confirming: %v", err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Errorf("the other file was removed: %v", err)
	}
	if got, want := ta.logFileNames(), []string{"keep.csv"}; !slices.Equal(got, want) {
		t.Errorf("files = %v, want %v", got, want)
	}
}

func TestLogFilesTabDeleteShowsWhyItFailed(t *testing.T) {
	ta := newTestApp(t)
	ta.do(func() { ta.SetWindow(ta.window) })
	path := ta.addLogFile("a.csv", 1, time.Hour)
	ta.show(ui.TabLogFiles)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	// the file is gone behind the app's back, so removing it fails
	ta.tap(ui.TabLogFiles, "Delete")
	ta.tapDialog("Delete")
	ta.do(func() {
		if !hasTextWithPrefix(ta.content(ui.TabLogFiles), "Couldn't delete a.csv") {
			t.Errorf("no error shown: %v", texts(ta.content(ui.TabLogFiles)))
		}
	})
}

func TestLogFilesTabMarksFileBeingLogged(t *testing.T) {
	ta := newTestApp(t)
	ta.do(func() { ta.cfg.LogFileNameFormat = "live.csv" })
	ta.connect()
	ta.setFileLog("Vehicle Speed", true)
	ta.addLogFile("other.csv", 1, time.Hour)
	ta.tap(ui.TabLogging, "Log To File")

	ta.show(ui.TabLogFiles)
	if !slices.Contains(ta.texts(ui.TabLogFiles), "Logging") {
		t.Errorf("the file being logged isn't marked: %v", ta.texts(ui.TabLogFiles))
	}
	if got, want := ta.logFileNames(), []string{"live.csv", "other.csv"}; !slices.Equal(got, want) {
		t.Fatalf("files = %v, want %v", got, want)
	}
	deletes := ta.logFileButtons("Delete")
	ta.do(func() {
		// the log file is the newest
		if !deletes[0].Disabled() {
			t.Error("Delete is enabled for the file being logged")
		}
		if deletes[1].Disabled() {
			t.Error("Delete is disabled for another file")
		}
	})

	ta.show(ui.TabLogging)
	ta.tap(ui.TabLogging, "Stop")
	ta.waitFor("the mark to go", func() bool {
		return !slices.Contains(texts(ta.content(ui.TabLogFiles)), "Logging")
	})
	for _, b := range ta.logFileButtons("Delete") {
		ta.do(func() {
			if b.Disabled() {
				t.Error("Delete is disabled after logging stopped")
			}
		})
	}
}

func TestLogFilesTabEmptyHints(t *testing.T) {
	ta := newTestApp(t)
	ta.show(ui.TabLogFiles)
	if !ta.hasTextWithPrefix(ui.TabLogFiles, "No log files in") {
		t.Errorf("no empty hint: %v", ta.texts(ui.TabLogFiles))
	}
	ta.addLogFile("notes.txt", 1, 0)
	ta.show(ui.TabConnection)
	ta.do(func() { ta.cfg.LogDirectory = filepath.Join(ta.cfg.LogDirectory, "missing") })
	ta.show(ui.TabLogFiles)
	if !ta.hasTextWithPrefix(ui.TabLogFiles, "No log files in") {
		t.Errorf("no hint for a missing directory: %v", ta.texts(ui.TabLogFiles))
	}
	if !strings.Contains(strings.Join(ta.texts(ui.TabLogFiles), " "), "Log To File") {
		t.Errorf("no explanation of where logs come from: %v", ta.texts(ui.TabLogFiles))
	}
}

func TestLogFilesTabShowsWhyTheDirectoryCouldntBeRead(t *testing.T) {
	ta := newTestApp(t)
	// a file where the directory should be
	file := ta.addLogFile("file", 1, 0)
	ta.do(func() { ta.cfg.LogDirectory = file })
	ta.show(ui.TabLogFiles)
	if !ta.hasTextWithPrefix(ui.TabLogFiles, "Couldn't read") {
		t.Errorf("no error shown: %v", ta.texts(ui.TabLogFiles))
	}
}

func TestLogFilesTabFitsSmallWindow(t *testing.T) {
	ta := newTestApp(t)
	for i := range 15 {
		name := "log-" + strings.Repeat("a", i) + ".csv"
		if i == 3 {
			name = strings.Repeat("a-very-long-log-file-name-", 6) + ".csv"
		}
		ta.addLogFile(name, 1000*(i+1), time.Duration(i)*time.Hour)
	}
	ta.show(ui.TabLogFiles)
	ta.resize(800, 480)
	ta.do(func() {
		min := ta.content(ui.TabLogFiles).MinSize()
		if min.Width > 800 || min.Height > 480 {
			t.Errorf("min size %v doesn't fit 800x480", min)
		}
	})
}
