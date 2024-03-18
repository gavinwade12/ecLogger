package main

import (
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"github.com/gavinwade12/ecLogger/protocols/ssm2"
)

const (
	configDirectoryName             = "ssm2"
	configFileName                  = ".ssm2"
	defaultLogFileNameFormat string = "ssm2_log_{{romId}}_{{timestamp}}.csv"
)

type Config struct {
	SelectedPort        string
	LogDirectory        *string
	LogFileNameFormat   *string
	LoggedParams        map[string]*loggedParam
	UseFakeConnection   bool
	AutoConnect         bool
	DefaultToLoggingTab bool
}

type App struct {
	config *Config
	// used for locking the logged params map on the config
	loggedParamsMu sync.RWMutex

	fyneApp fyne.App

	tabItems      *container.AppTabs
	ConnectionTab *ConnectionTab
	ParametersTab *ParametersTab
	LoggingTab    *LoggingTab
	SettingsTab   *SettingsTab

	connection ssm2.Connection
	ecu        *ssm2.ECU
}

func (a *App) removeFromLoggedParams(key string) {
	a.loggedParamsMu.Lock()
	defer a.loggedParamsMu.Unlock()
	delete(a.config.LoggedParams, key)
}

func (a *App) updateLoggedParam(key string, update func(*loggedParam)) {
	a.loggedParamsMu.Lock()
	defer a.loggedParamsMu.Unlock()
	update(a.config.LoggedParams[key])
}

func (a *App) updateOrAddToLoggedParams(key string, update func(*loggedParam), add *loggedParam) {
	a.loggedParamsMu.Lock()
	defer a.loggedParamsMu.Unlock()

	if a.config.LoggedParams[key] != nil {
		update(a.config.LoggedParams[key])
	} else {
		a.config.LoggedParams[key] = add
	}
}

func (a *App) getLoggedParam(key string) *loggedParam {
	a.loggedParamsMu.RLock()
	defer a.loggedParamsMu.RUnlock()
	return a.config.LoggedParams[key]
}

func (a *App) readOnlyLoggedParams() map[string]*loggedParam {
	a.loggedParamsMu.RLock()
	defer a.loggedParamsMu.RUnlock()

	m := make(map[string]*loggedParam)
	for k, v := range a.config.LoggedParams {
		m[k] = v
	}
	return m
}

func (a *App) getCurrentLoggedParamLists() ([]ssm2.Parameter, []ssm2.DerivedParameter) {
	params := []ssm2.Parameter{}
	derivedParams := []ssm2.DerivedParameter{}
	loggedParams := a.readOnlyLoggedParams()
	for id, p := range loggedParams {
		if p.Derived {
			derivedParams = append(derivedParams, ssm2.DerivedParameters[id])
		} else {
			params = append(params, ssm2.Parameters[id])
		}
	}

	return params, derivedParams
}

func (a *App) onNewConnection(conn ssm2.Connection, ecu *ssm2.ECU) {
	a.connection = conn
	a.ecu = ecu

	a.ParametersTab.setAvailableParameters(ecu)
	a.LoggingTab.updateLiveLogParameters()
	a.tabItems.EnableIndex(2) // enable the Logging tab
}

func (a *App) onDisconnect() {
	if a.LoggingTab.cancelLogging != nil {
		a.LoggingTab.cancelLogging()
		a.LoggingTab.cancelLogging = nil
	}

	a.connection.Close()
	a.connection = nil
	a.ecu = nil
	a.ConnectionTab.onDisconnect()

	a.ParametersTab.setAvailableParameters(nil)
	a.LoggingTab.updateLiveLogParameters()
	a.tabItems.DisableIndex(2) // disable the Logging tab
}
