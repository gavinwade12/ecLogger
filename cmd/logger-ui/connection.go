package main

import (
	"context"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/gavinwade12/ecLogger/protocols/ssm2"
	"github.com/pkg/errors"
	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
)

type ConnectionTab struct {
	app *App

	serialPortSelect    *widget.Select
	stopSerialPortQuery chan struct{}

	connectBtn      *widget.Button
	connectionState binding.String

	container *fyne.Container

	conn ssm2.Connection
	ecu  *ssm2.ECU
}

func NewConnectionTab(app *App) *ConnectionTab {
	connectionTab := &ConnectionTab{
		app: app,
		serialPortSelect: widget.NewSelect([]string{}, func(s string) {
			app.config.SelectedPort = s
		}),
		connectBtn:      widget.NewButton("Connect", nil),
		connectionState: binding.NewString(),
	}
	go connectionTab.querySerialPorts()

	form := widget.NewForm(
		widget.NewFormItem("Port", connectionTab.serialPortSelect),
	)

	connectionTab.connectionState.Set("Disconnected")
	cancelBtn := widget.NewButton("Cancel", nil)
	disconnectBtn := widget.NewButton("Disconnect", nil)
	connectionTab.connectBtn.OnTapped = func() {
		// disable this button and the select, show the cancel button, and stop
		// querying for serial port changes
		connectionTab.connectBtn.Disable()
		connectionTab.serialPortSelect.Disable()
		cancelBtn.Show()
		connectionTab.stopSerialPortQuery <- struct{}{}

		// set up the cancel button
		ctx, cancel := context.WithCancel(context.Background())
		cancelBtn.OnTapped = func() {
			cancel()
			cancelBtn.Hide()
			connectionTab.serialPortSelect.Enable()
			go connectionTab.querySerialPorts()
			connectionTab.connectBtn.Enable()
			connectionTab.connectionState.Set("Disconnected")
		}

		// try connecting in another goroutine to prevent the UI from locking up
		go func() {
			connectionTab.connectionState.Set("Connecting")
			err := openSSM2Connection(app)
			if err != nil {
				cancelBtn.Hide()
				connectionTab.serialPortSelect.Enable()
				go connectionTab.querySerialPorts()
				connectionTab.connectBtn.Enable()
				connectionTab.connectionState.Set("Disconnected")
				logger.Debug(err.Error())
				return
			}

			connectionTab.connectionState.Set("Initializing")
			err = connectionTab.initSSM2Connection(ctx)
			cancelBtn.Hide()
			connectionTab.serialPortSelect.Enable()
			if err != nil {
				go connectionTab.querySerialPorts()
				connectionTab.connectBtn.Enable()
				connectionTab.connectionState.Set("Disconnected")
				logger.Debug(err.Error())
			} else {
				disconnectBtn.Show()
			}
			cancel()
		}()
	}
	disconnectBtn.OnTapped = func() {
		if app.LoggingTab.stopLogging != nil {
			app.LoggingTab.stopLogging()
			app.LoggingTab.stopLogging = nil
		}
		connectionTab.conn.Close()
		connectionTab.conn = nil
		disconnectBtn.Hide()
		go connectionTab.querySerialPorts()
		app.ParametersTab.setAvailableParameters(nil)
		app.LoggingTab.updateLiveLogParameters()
		app.tabItems.DisableIndex(2) // disable the Logging tab
		connectionTab.connectBtn.Enable()
		connectionTab.connectionState.Set("Disconnected")
	}
	cancelBtn.Hide()
	disconnectBtn.Hide()

	connectionTab.container = container.New(layout.NewVBoxLayout(),
		form,
		container.New(layout.NewHBoxLayout(),
			widget.NewLabel("Status: "),
			widget.NewLabelWithData(connectionTab.connectionState)),
		connectionTab.connectBtn,
		cancelBtn,
		disconnectBtn)

	return connectionTab
}

func (t *ConnectionTab) Container() fyne.CanvasObject {
	return t.container
}

var (
	defaultOpenFunc = func(app *App) error {
		app.ConnectionTab.connectionState.Set("Connecting...")

		if app.config.SelectedPort == "" {
			return errors.New("a port is required")
		}

		logger.Debugf("opening serial port %s", app.config.SelectedPort)
		sp, err := serial.Open(app.config.SelectedPort, &serial.Mode{
			BaudRate: ssm2.ConnectionBaudRate,
			DataBits: ssm2.ConnectionDataBits,
			Parity:   serial.NoParity,
			StopBits: serial.OneStopBit,
		})
		if err != nil {
			return errors.Wrapf(err, "opening serial port '%s'", app.config.SelectedPort)
		}

		if err = sp.SetReadTimeout(ssm2.ConnectionReadTimeout); err != nil {
			return errors.Wrap(err, "setting serial port read timeout")
		}
		if err != nil {
			return err
		}

		app.ConnectionTab.conn = ssm2.NewConnection(sp, logger)
		return nil
	}
	fakeOpenFunc = func(app *App) error {
		app.ConnectionTab.conn = ssm2.NewFakeConnection(time.Millisecond * 50)
		return nil
	}
	openSSM2Connection = defaultOpenFunc
)

func (t *ConnectionTab) initSSM2Connection(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			t.conn.Close()
			return ctx.Err()
		default:
			var err error
			t.ecu, err = t.conn.InitECU(ctx)
			if err != nil {
				logger.Debug(err.Error())
				continue
			}
			t.app.ParametersTab.setAvailableParameters(t.ecu)
			t.app.LoggingTab.updateLiveLogParameters()
			t.app.tabItems.EnableIndex(2) // enable the Logging tab
			t.connectionState.Set("Connected")
			return nil
		}
	}
}

func (t *ConnectionTab) querySerialPorts() {
	t.stopSerialPortQuery = make(chan struct{})

	query := func() {
		pl, err := enumerator.GetDetailedPortsList()
		if err != nil {
			logger.Debug(err.Error())
			return
		}

		ports := make([]string, len(pl))
		for i, p := range pl {
			ports[i] = p.Name
		}

		t.serialPortSelect.Options = ports
		t.serialPortSelect.SetSelected(t.app.config.SelectedPort)
		t.serialPortSelect.Refresh()
	}

	query()

	for {
		select {
		case <-t.stopSerialPortQuery:
			return
		case <-time.NewTicker(time.Second * 30).C:
			query()
		}
	}
}
