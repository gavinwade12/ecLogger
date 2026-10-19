// Package serialport opens SSM2 connections on serial ports and lists the
// serial ports on the host.
package serialport

import (
	"fmt"

	"github.com/gavinwade12/ecLogger/protocols/ssm2"
	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
)

// Open opens the named serial port with the SSM2 settings and returns a
// connection on it. The port is closed if it can't be set up.
func Open(name string, l ssm2.Logger) (ssm2.Connection, error) {
	if l == nil {
		l = ssm2.NopLogger
	}

	l.Debugf("opening serial port %s", name)
	sp, err := serial.Open(name, &serial.Mode{
		BaudRate: ssm2.ConnectionBaudRate,
		DataBits: ssm2.ConnectionDataBits,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	})
	if err != nil {
		return nil, fmt.Errorf("opening serial port '%s': %w", name, err)
	}

	if err = setUp(sp); err != nil {
		sp.Close()
		return nil, err
	}
	return ssm2.NewConnection(sp, l), nil
}

// setUp sets the read timeout and discards any stale input, which would
// otherwise be read as the start of the first response.
func setUp(sp serial.Port) error {
	if err := sp.SetReadTimeout(ssm2.ConnectionReadTimeout); err != nil {
		return fmt.Errorf("setting serial port read timeout: %w", err)
	}
	if err := sp.ResetInputBuffer(); err != nil {
		return fmt.Errorf("resetting input buffer: %w", err)
	}
	return nil
}

// Port describes a serial port on the host.
type Port struct {
	Name      string
	Product   string
	IsUSB     bool
	VendorID  string
	ProductID string
}

// List returns the serial ports on the host.
func List() ([]Port, error) {
	list, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return nil, err
	}

	ports := make([]Port, len(list))
	for i, p := range list {
		ports[i] = Port{
			Name:      p.Name,
			Product:   p.Product,
			IsUSB:     p.IsUSB,
			VendorID:  p.VID,
			ProductID: p.PID,
		}
	}
	return ports, nil
}
