package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestLoadConfig_ReadsTheGivenFile(t *testing.T) {
	t.Chdir(t.TempDir())
	file := filepath.Join(t.TempDir(), "ssm2.yaml")
	if err := os.WriteFile(file, []byte("port: /dev/ttyUSB1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	v := viper.New()
	if err := loadConfig(v, file, ""); err != nil {
		t.Fatal(err)
	}

	if got := v.GetString("port"); got != "/dev/ttyUSB1" {
		t.Errorf("port = %q, want /dev/ttyUSB1", got)
	}
}

func TestLoadConfig_CreatesAMissingGivenFile(t *testing.T) {
	wd := t.TempDir()
	t.Chdir(wd)
	file := filepath.Join(t.TempDir(), "ssm2.yaml")

	if err := loadConfig(viper.New(), file, ""); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(file); err != nil {
		t.Errorf("config file wasn't created: %v", err)
	}
	if entries, _ := os.ReadDir(wd); len(entries) != 0 {
		t.Errorf("working directory has %d entries, want 0", len(entries))
	}
}

func TestLoadConfig_DefaultsToHome(t *testing.T) {
	home := t.TempDir()

	if err := loadConfig(viper.New(), "", home); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(home, ".ssm2.yaml")); err != nil {
		t.Errorf("config file wasn't created: %v", err)
	}
}
