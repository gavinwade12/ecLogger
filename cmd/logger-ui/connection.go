package main

import (
	"context"
	"os"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/gavinwade12/ssm2/protocols/ssm2"
	"github.com/pkg/errors"
	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
)

var serialPortSelect *widget.Select

func connectionContainer() fyne.CanvasObject {
	serialPortSelect = widget.NewSelect([]string{}, func(s string) {
		config.SelectedPort = s
	})
	go querySerialPorts()

	form := widget.NewForm(
		widget.NewFormItem("Port", serialPortSelect),
	)

	connectionState.Set("Disconnected")
	connectBtn := widget.NewButton("Connect", nil)
	cancelBtn := widget.NewButton("Cancel", nil)
	connectBtn.OnTapped = func() {
		// disable this button and the select, show the cancel button, and stop
		// querying for serial port changes
		connectBtn.Disable()
		serialPortSelect.Disable()
		cancelBtn.Show()
		stopSerialPortQuery <- struct{}{}

		// set up the cancel button
		ctx, cancel := context.WithCancel(context.Background())
		cancelBtn.OnTapped = func() {
			cancel()
			cancelBtn.Hide()
			serialPortSelect.Enable()
			go querySerialPorts()
			connectBtn.Enable()
			connectionState.Set("Disconnected")
		}

		// try connecting in another goroutine to prevent the UI from locking up
		go func() {
			connectionState.Set("Connecting")
			err := openSSM2Connection()
			if err != nil {
				cancelBtn.Hide()
				serialPortSelect.Enable()
				go querySerialPorts()
				connectBtn.Enable()
				connectionState.Set("Disconnected")
				logger.Debug(err.Error())
				return
			}

			connectionState.Set("Initializing")
			err = initSSM2Connection(ctx)
			cancelBtn.Hide()
			serialPortSelect.Enable()
			if err != nil {
				go querySerialPorts()
				connectBtn.Enable()
				connectionState.Set("Disconnected")
				logger.Debug(err.Error())
			}
			cancel()
		}()
	}
	cancelBtn.Hide()

	c := container.New(layout.NewVBoxLayout(),
		form,
		container.New(layout.NewHBoxLayout(),
			widget.NewLabel("Status: "),
			widget.NewLabelWithData(connectionState)),
		connectBtn,
		cancelBtn)

	return c
}

var defaultOpenFunc = func() error {
	if logger == nil {
		logger = ssm2.DefaultLogger(os.Stdout)
	}

	connectionState.Set("Connecting...")

	if config.SelectedPort == "" {
		return errors.New("a port is required")
	}

	logger.Debugf("opening serial port %s", config.SelectedPort)
	sp, err := serial.Open(config.SelectedPort, &serial.Mode{
		BaudRate: ssm2.ConnectionBaudRate,
		DataBits: ssm2.ConnectionDataBits,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	})
	if err != nil {
		return errors.Wrapf(err, "opening serial port '%s'", config.SelectedPort)
	}

	if err = sp.SetReadTimeout(ssm2.ConnectionReadTimeout); err != nil {
		return errors.Wrap(err, "setting serial port read timeout")
	}
	if err != nil {
		return err
	}

	conn = ssm2.NewConnection(sp, logger)
	return nil
}

var openSSM2Connection = defaultOpenFunc

func initSSM2Connection(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			conn.Close()
			return ctx.Err()
		default:
			resp, err := conn.InitECU(ctx)
			if err != nil {
				logger.Debug(err.Error())
				continue
			}
			setAvailableParameters(resp)
			connectionState.Set("Connected")
			return nil
		}
	}
}

var stopSerialPortQuery chan struct{}

func querySerialPorts() {
	stopSerialPortQuery = make(chan struct{})

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

		serialPortSelect.Options = ports
		serialPortSelect.SetSelected(config.SelectedPort)
		serialPortSelect.Refresh()
	}

	query()

	for {
		select {
		case <-stopSerialPortQuery:
			return
		case <-time.NewTicker(time.Second * 30).C:
			query()
		}
	}
}

var connectionState = binding.NewString()

var logger ssm2.Logger
var conn ssm2.Connection
