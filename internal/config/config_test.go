package config_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/gavinwade12/ecLogger/internal/config"
	"github.com/gavinwade12/ecLogger/internal/logging"
	"github.com/gavinwade12/ecLogger/units"
)

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	dir := t.TempDir()

	c, err := config.Load(filepath.Join(dir, ".ssm2"))
	if err != nil {
		t.Fatal(err)
	}
	if c.LogDirectory != filepath.Join(dir, "logs") {
		t.Errorf("LogDirectory = %q, want %q", c.LogDirectory, filepath.Join(dir, "logs"))
	}
	if c.LogFileNameFormat != config.DefaultLogFileNameFormat {
		t.Errorf("LogFileNameFormat = %q, want the default", c.LogFileNameFormat)
	}
	if c.LoggedParams == nil {
		t.Error("LoggedParams is nil")
	}
	if c.LiveGraphMinutes != 5 || c.LiveGraphWindow() != 5*time.Minute {
		t.Errorf("LiveGraphMinutes = %d, want 5", c.LiveGraphMinutes)
	}
}

func TestLoadFixesInvalidLiveGraphMinutes(t *testing.T) {
	for _, minutes := range []string{"0", "-3"} {
		path := filepath.Join(t.TempDir(), ".ssm2")
		if err := os.WriteFile(path, []byte(`{"LiveGraphMinutes":`+minutes+`}`), 0o644); err != nil {
			t.Fatal(err)
		}
		c, err := config.Load(path)
		if err != nil {
			t.Fatal(err)
		}
		if c.LiveGraphMinutes != config.DefaultLiveGraphMinutes {
			t.Errorf("LiveGraphMinutes %s loaded as %d, want the default", minutes, c.LiveGraphMinutes)
		}
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	// the parent directory doesn't exist yet
	path := filepath.Join(t.TempDir(), "ssm2", ".ssm2")
	want := &config.Config{
		SelectedPort:      "/dev/ttyUSB0",
		LogDirectory:      "/tmp/logs",
		LogFileNameFormat: "{{timestamp}}.csv",
		LoggedParams: map[string]logging.LoggedParam{
			"P9": {LiveLog: true, Unit: units.KMH},
		},
		AutoConnect:      true,
		LiveGraphMinutes: 15,
	}

	if err := config.Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Load() = %+v, want %+v", got, want)
	}
}

func TestLoadIsCompatibleWithOldConfigs(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".ssm2")
	old := `{"SelectedPort":"COM3","LogDirectory":null,"LogFileNameFormat":"x.csv",` +
		`"LoggedParams":{"P8":{"LogToFile":true,"LiveLog":false,"Derived":false,"Unit":"rpm"}},` +
		`"UseFakeConnection":false,"AutoConnect":false,"DefaultToLoggingTab":true}`
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}

	c, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.SelectedPort != "COM3" || c.LogFileNameFormat != "x.csv" || !c.DefaultToLoggingTab {
		t.Errorf("unexpected config: %+v", c)
	}
	if c.LiveGraphMinutes != config.DefaultLiveGraphMinutes {
		t.Errorf("an old config's LiveGraphMinutes = %d, want the default", c.LiveGraphMinutes)
	}
	if c.LogDirectory == "" {
		t.Error("a null LogDirectory should fall back to the default")
	}
	if lp := c.LoggedParams["P8"]; !lp.LogToFile || lp.Unit != units.RPM {
		t.Errorf("LoggedParams[P8] = %+v", lp)
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".ssm2")
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := config.Load(path); err == nil {
		t.Fatal("expected an error")
	}
}
