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
		}

		// try connecting in another goroutine to prevent the UI from locking up
		go connectionTab.openAndInitConnection(ctx, cancelBtn, disconnectBtn)
	}
	disconnectBtn.OnTapped = app.onDisconnect
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

func (t *ConnectionTab) openAndInitConnection(ctx context.Context, cancelBtn, disconnectBtn *widget.Button) {
	var (
		cleanup = func() {
			cancelBtn.Hide()
			t.serialPortSelect.Enable()
		}
		onError = func(err error) {
			cleanup()
			t.onDisconnect()
			logger.Debug(err.Error())
		}
	)

	t.connectionState.Set("Connecting")
	conn, err := openSSM2Connection(t.app)
	if err != nil {
		onError(err)
		return
	}

	t.connectionState.Set("Initializing")
	err = t.initSSM2Connection(ctx, conn)
	cleanup()
	if err != nil {
		onError(err)
	} else {
		disconnectBtn.Show()
	}
}

func (t *ConnectionTab) initSSM2Connection(ctx context.Context, conn ssm2.Connection) error {
	// keep trying to init the connection until we successfully connect
	// or the context is cancelled
	for {
		select {
		case <-ctx.Done():
			conn.Close()
			return ctx.Err()
		default:
			ecu, err := conn.InitECU(ctx)
			if err != nil {
				logger.Debug(err.Error())
				continue
			}

			t.app.onNewConnection(conn, ecu)
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

	// query now
	query()

	// and query every 30 seconds until we receive a signal to stop
	for {
		select {
		case <-t.stopSerialPortQuery:
			return
		case <-time.NewTicker(time.Second * 30).C:
			query()
		}
	}
}

func (t *ConnectionTab) onDisconnect() {
	t.serialPortSelect.Enable()
	go t.querySerialPorts()
	t.connectBtn.Enable()
	t.connectionState.Set("Disconnected")
}

var (
	defaultOpenFunc = func(app *App) (ssm2.Connection, error) {
		app.ConnectionTab.connectionState.Set("Connecting...")

		if app.config.SelectedPort == "" {
			return nil, errors.New("a port is required")
		}

		logger.Debugf("opening serial port %s", app.config.SelectedPort)
		sp, err := serial.Open(app.config.SelectedPort, &serial.Mode{
			BaudRate: ssm2.ConnectionBaudRate,
			DataBits: ssm2.ConnectionDataBits,
			Parity:   serial.NoParity,
			StopBits: serial.OneStopBit,
		})
		if err != nil {
			return nil, errors.Wrapf(err, "opening serial port '%s'", app.config.SelectedPort)
		}

		if err = sp.SetReadTimeout(ssm2.ConnectionReadTimeout); err != nil {
			return nil, errors.Wrap(err, "setting serial port read timeout")
		}
		if err != nil {
			return nil, err
		}

		return ssm2.NewConnection(sp, logger), nil
	}
	fakeOpenFunc = func(app *App) (ssm2.Connection, error) {
		return ssm2.NewFakeConnection(time.Millisecond * 50), nil
	}
	openSSM2Connection = defaultOpenFunc
)
