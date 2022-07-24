package main

import (
	"io"

	"fyne.io/fyne/v2/data/binding"
	"github.com/gavinwade12/ssm2/protocols/ssm2"
	"github.com/pkg/errors"
	"go.bug.st/serial"
)

var loggingState = binding.NewString()

var openSerialPort = func(p string) (io.ReadWriteCloser, error) {
	if p == "" {
		return nil, errors.New("a port is required")
	}

	logger.Debugf("opening serial port %s", p)
	sp, err := serial.Open(p, &serial.Mode{
		BaudRate: ssm2.ConnectionBaudRate,
		DataBits: ssm2.ConnectionDataBits,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "opening serial port '%s'", p)
	}

	if err = sp.SetReadTimeout(ssm2.ConnectionReadTimeout); err != nil {
		return nil, errors.Wrap(err, "setting serial port read timeout")
	}

	return sp, nil
}

var logger ssm2.Logger
var conn *ssm2.Connection
