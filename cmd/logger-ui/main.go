package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"github.com/gavinwade12/ssm2/units"
	"github.com/pkg/errors"
)

var (
	configFileName           = ".ssm2"
	defaultLogFileNameFormat = "ssm2_log_{{romId}}_{{timestamp}}.csv"
	config                   struct {
		SelectedPort      string
		LogFileNameFormat *string
		LoggedParams      map[string]*loggedParam
		UseFakeConnection bool
	}
	loggedParamsMu sync.RWMutex
)

type loggedParam struct {
	LogToFile bool
	LiveLog   bool
	Derived   bool
	Unit      units.Unit
}

func main() {
	if err := loadConfig(); err != nil {
		log.Fatal(err)
	}

	a := app.New()
	w := a.NewWindow("Logger")
	w.Resize(fyne.NewSize(800, 400))

	tabs := container.NewAppTabs(
		container.NewTabItem("Connection", connectionContainer()),
		container.NewTabItem("Parameters", parametersContainer()),
		container.NewTabItem("Logging", loggingContainer()),
		container.NewTabItem("Settings", settingsContainer()),
	)

	tabs.SetTabLocation(container.TabLocationLeading)

	w.SetContent(tabs)
	w.ShowAndRun()

	if err := saveConfig(); err != nil {
		log.Fatal(err)
	}
}

func loadConfig() error {
	dir, err := os.UserHomeDir()
	if err != nil {
		return errors.Wrap(err, "finding user home directory")
	}

	f, err := os.Open(filepath.Join(dir, configFileName))
	if err != nil {
		if !os.IsNotExist(err) {
			return errors.Wrap(err, "opening config file")
		}

		config.LogFileNameFormat = &defaultLogFileNameFormat
		config.LoggedParams = make(map[string]*loggedParam)
		return nil
	}
	defer f.Close()

	err = json.NewDecoder(f).Decode(&config)
	if err != nil {
		return errors.Wrap(err, "decoding config from file")
	}
	if config.LoggedParams == nil {
		config.LoggedParams = make(map[string]*loggedParam)
	}
	return nil
}

func saveConfig() error {
	dir, err := os.UserHomeDir()
	if err != nil {
		return errors.Wrap(err, "finding user home directory")
	}

	f, err := os.OpenFile(filepath.Join(dir, configFileName), os.O_CREATE|os.O_TRUNC|os.O_RDWR, os.ModePerm)
	if err != nil {
		return errors.Wrap(err, "opening config file")
	}
	defer f.Close()

	err = json.NewEncoder(f).Encode(config)
	if err != nil {
		return errors.Wrap(err, "encoding config to file")
	}
	return nil
}
