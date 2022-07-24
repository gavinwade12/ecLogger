package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/gavinwade12/ssm2/protocols/ssm2"
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

	loggingState.Set("Disconnected")
	connectBtn := widget.NewButton("Connect", nil)
	cancelBtn := widget.NewButton("Cancel", nil)
	connectBtn.OnTapped = func() {
		// disable this button and the select, show the cancel button, and stop
		// quering for serial port changes
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
			loggingState.Set("Disconnected")
		}

		// try connecting in another goroutine to prevent the UI from locking up
		go func() {
			err := openSSM2Connection(ctx)
			cancelBtn.Hide()
			serialPortSelect.Enable()
			if err != nil {
				go querySerialPorts()
				connectBtn.Enable()
				loggingState.Set("Disconnected")
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
			widget.NewLabelWithData(loggingState)),
		connectBtn,
		cancelBtn)

	return c
}

func openSSM2Connection(ctx context.Context) error {
	if logger == nil {
		logger = ssm2.DefaultLogger(os.Stdout)
	}

	loggingState.Set("Connecting...")

	sp, err := openSerialPort(config.SelectedPort)
	if err != nil {
		return err
	}

	conn = ssm2.NewConnection(sp, logger)
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
			fmt.Println(resp.SSM_ID)
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
