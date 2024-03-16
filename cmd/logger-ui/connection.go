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
	"github.com/gavinwade12/ecLogger/protocols/ssm2"
	"github.com/pkg/errors"
	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
)

var (
	serialPortSelect *widget.Select
	connectBtn       *widget.Button
)

func connectionContainer() fyne.CanvasObject {
	serialPortSelect = widget.NewSelect([]string{}, func(s string) {
		config.SelectedPort = s
	})
	go querySerialPorts()

	form := widget.NewForm(
		widget.NewFormItem("Port", serialPortSelect),
	)

	connectionState.Set("Disconnected")
	connectBtn = widget.NewButton("Connect", nil)
	cancelBtn := widget.NewButton("Cancel", nil)
	disconnectBtn := widget.NewButton("Disconnect", nil)
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
			} else {
				disconnectBtn.Show()
			}
			cancel()
		}()
	}
	disconnectBtn.OnTapped = func() {
		if loggingTab.stopLogging != nil {
			loggingTab.stopLogging()
			loggingTab.stopLogging = nil
		}
		conn.Close()
		conn = nil
		disconnectBtn.Hide()
		go querySerialPorts()
		setAvailableParameters(nil)
		loggingTab.updateLiveLogParameters()
		tabItems.DisableIndex(2) // disable the Logging tab
		connectBtn.Enable()
		connectionState.Set("Disconnected")
	}
	cancelBtn.Hide()
	disconnectBtn.Hide()

	c := container.New(layout.NewVBoxLayout(),
		form,
		container.New(layout.NewHBoxLayout(),
			widget.NewLabel("Status: "),
			widget.NewLabelWithData(connectionState)),
		connectBtn,
		cancelBtn,
		disconnectBtn)

	return c
}

var (
	defaultOpenFunc = func() error {
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
	fakeOpenFunc = func() error {
		conn = ssm2.NewFakeConnection(time.Millisecond * 50)
		return nil
	}
	openSSM2Connection = defaultOpenFunc
)

func initSSM2Connection(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			conn.Close()
			return ctx.Err()
		default:
			var err error
			ecu, err = conn.InitECU(ctx)
			if err != nil {
				logger.Debug(err.Error())
				continue
			}
			setAvailableParameters(ecu)
			loggingTab.updateLiveLogParameters()
			tabItems.EnableIndex(2) // enable the Logging tab
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

var (
	connectionState = binding.NewString()
	logger          = ssm2.DefaultLogger(os.Stdout)
	conn            ssm2.Connection
	ecu             *ssm2.ECU
)
