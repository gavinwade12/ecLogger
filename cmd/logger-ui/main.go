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
	"github.com/gavinwade12/ecLogger/units"
	"github.com/pkg/errors"
)

var (
	configDirectoryName      = "ssm2"
	configFileName           = ".ssm2"
	defaultLogFileNameFormat = "ssm2_log_{{romId}}_{{timestamp}}.csv"
	config                   struct {
		SelectedPort        string
		LogDirectory        *string
		LogFileNameFormat   *string
		LoggedParams        map[string]*loggedParam
		UseFakeConnection   bool
		AutoConnect         bool
		DefaultToLoggingTab bool
	}
	loggedParamsMu sync.RWMutex
)

type loggedParam struct {
	LogToFile bool
	LiveLog   bool
	Derived   bool
	Unit      units.Unit
}

var tabItems *container.AppTabs

func main() {
	if err := loadConfig(); err != nil {
		log.Fatal(err)
	}

	a := app.New()
	w := a.NewWindow("Logger")
	w.Resize(fyne.NewSize(800, 400))

	loggingTab = NewLoggingTab()

	tabItems = container.NewAppTabs(
		container.NewTabItem("Connection", connectionContainer()),
		container.NewTabItem("Parameters", parametersContainer()),
		container.NewTabItem("Logging", loggingTab.Container()),
		container.NewTabItem("Settings", settingsContainer()),
	)
	tabItems.DisableIndex(1)
	tabItems.DisableIndex(2)
	if config.AutoConnect {
		go connectBtn.Tapped(&fyne.PointEvent{})
	}
	if config.DefaultToLoggingTab {
		tabItems.SelectIndex(2)
	}

	tabItems.SetTabLocation(container.TabLocationLeading)

	w.SetContent(tabItems)
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
	ssm2Dir := filepath.Join(dir, configDirectoryName)

	f, err := os.Open(filepath.Join(ssm2Dir, configFileName))
	if err != nil {
		if !os.IsNotExist(err) {
			return errors.Wrap(err, "opening config file")
		}

		logDirectory := filepath.Join(ssm2Dir, "logs")
		config.LogDirectory = &logDirectory
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
	if config.UseFakeConnection {
		openSSM2Connection = fakeOpenFunc
	}
	if config.LogDirectory == nil {
		logDirectory := filepath.Join(ssm2Dir, "logs")
		config.LogDirectory = &logDirectory
	}
	if config.LogFileNameFormat == nil {
		config.LogFileNameFormat = &defaultLogFileNameFormat
	}
	return nil
}

func saveConfig() error {
	dir, err := os.UserHomeDir()
	if err != nil {
		return errors.Wrap(err, "finding user home directory")
	}

	configPath := filepath.Join(dir, configDirectoryName, configFileName)
	f, err := os.OpenFile(configPath, os.O_CREATE|os.O_TRUNC|os.O_RDWR, os.ModePerm)
	if err != nil {
		if !os.IsNotExist(err) {
			return errors.Wrap(err, "opening config file")
		}

		if err = os.Mkdir(filepath.Join(dir, configDirectoryName), os.ModePerm); err != nil {
			return errors.Wrap(err, "creating config directory")
		}
		f, err = os.OpenFile(configPath, os.O_CREATE|os.O_TRUNC|os.O_RDWR, os.ModePerm)
		if err != nil {
			return errors.Wrap(err, "opening config file")
		}
	}
	defer f.Close()

	err = json.NewEncoder(f).Encode(config)
	if err != nil {
		return errors.Wrap(err, "encoding config to file")
	}
	return nil
}
