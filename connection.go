package ssm2

import (
	"context"
	"fmt"
	"io"
	"math"
	"time"

	"github.com/pkg/errors"
	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
)

// SerialPort describes a serial port on the host.
type SerialPort struct {
	PortName    string
	Description string
	IsUSB       bool
}

// AvailablePorts returns all available serial ports on the current host.
func AvailablePorts() ([]SerialPort, error) {
	list, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return nil, err
	}

	ports := make([]SerialPort, len(list))
	for i, p := range list {
		ports[i] = SerialPort{
			PortName:    p.Name,
			Description: p.Product,
			IsUSB:       p.IsUSB,
		}
	}

	return ports, nil
}

// Connection provides high-level methods for communicating with
// an ECU.
type Connection struct {
	portName   string
	serialPort io.ReadWriteCloser
}

const (
	// ConnectionBaudRate is the baud rate (bits/s) used for the serial connection.
	ConnectionBaudRate int = 4800
	// ConnectionDataBits is the data bit setting (bits/word) used for the serial connection.
	ConnectionDataBits int = 8
	// ConnectionReadTimeout is the amount of time per read spent before a timeout occurs.
	// The timeout is per read, but it may take several reads to consume an entire packet.
	ConnectionReadTimeout time.Duration = time.Minute * 500
	// ConnectionTotalReadTimeout is the amount of time spent to read an entire buffer before
	// a timeout occurs. This applies to the full-length read and not individual reads.
	ConnectionTotalReadTimeout time.Duration = time.Minute * 1500
)

// NewConnection returns a new Connection with the serial port already initialized.
func NewConnection(portName string) (*Connection, error) {
	c := &Connection{portName: portName}
	err := c.Initialize()
	if err != nil {
		return nil, errors.Wrap(err, "initializing serial port")
	}

	return c, nil
}

// Initialize opens the serial port and configures it. If a conncetion already exists on the port,
// it will be closed before it's re-opened and configured.
func (c *Connection) Initialize() error {
	if c.serialPort != nil {
		if err := c.serialPort.Close(); err != nil {
			return errors.Wrap(err, "closing existing serial port during initialization")
		}
		c.serialPort = nil
	}

	sp, err := serial.Open(c.portName, &serial.Mode{
		BaudRate: ConnectionBaudRate,
		DataBits: ConnectionDataBits,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	})
	if err != nil {
		return errors.Wrapf(err, "opening serial port '%s'", c.portName)
	}

	// this only applies once at least one byte is read, so our
	// manual timeout applied during reading will take affect most/all the time
	sp.SetReadTimeout(ConnectionReadTimeout)

	c.serialPort = sp

	return nil
}

func (c *Connection) SerialPort() io.ReadWriter {
	return c.serialPort
}

// Close releases any resouces held by the connection e.g. the connection over the serial port.
func (c *Connection) Close() error {
	if c.serialPort == nil {
		return nil
	}

	err := c.serialPort.Close()
	if err != nil {
		return errors.Wrap(err, "closing serial port")
	}

	c.serialPort = nil

	return nil
}

const (
	deviceNone                   byte = 0
	deviceEngine                 byte = 0x10
	deviceTransmission           byte = 0x18
	deviceDiagnosticTool         byte = 0xf0
	deviceFastModeDiagnosticTool byte = 0xf2

	commandNone                  byte = 0
	commandReadBlockRequest      byte = 0xa0
	commandReadBlockResponse     byte = 0xe0
	commandReadAddressesRequest  byte = 0xa8
	commandReadAddressesResponse byte = 0xe8
	commandWriteBlockRequest     byte = 0xb0
	commandWriteBlockResponse    byte = 0xf0
	commandWriteAddressRequest   byte = 0xb8
	commandWriteAddressResponse  byte = 0xf8
	commandInitRequest           byte = 0xbf
	commandInitResponse          byte = 0xff
)

var devices = []byte{
	deviceNone, deviceEngine, deviceTransmission, deviceDiagnosticTool,
	deviceFastModeDiagnosticTool,
}

var commands = []byte{
	commandNone, commandReadBlockRequest, commandReadBlockResponse,
	commandReadAddressesRequest, commandReadAddressesResponse,
	commandWriteBlockRequest, commandWriteBlockResponse,
	commandWriteAddressRequest, commandWriteAddressResponse,
	commandInitRequest, commandInitResponse,
}

// SendInitCommand sends an init command to the ECU and returns its response. This is useful for
// getting information about the ECU and which parameters it supports.
func (c *Connection) SendInitCommand(ctx context.Context) (InitResponse, error) {
	p := newPacket(deviceDiagnosticTool, deviceEngine, commandInitRequest, nil)
	rp, err := c.sendPacket(ctx, p)
	if err != nil {
		return nil, errors.Wrap(err, "sending packet")
	}

	if rp[PacketIndexCommand] != byte(commandInitResponse) {
		return nil, errors.New(fmt.Sprintf("unexpected response code %x (expected %x)",
			rp[PacketIndexCommand], commandInitResponse))
	}

	return InitResponse(rp), nil
}

// ReadAddressses sends a read addresses request to the ECU. The results should be fetched via NextPacket().
// When continous is true, NextPacket() will continue to return results for the given addresses until the ECU
// is interrupted.
func (c *Connection) SendReadAddressesRequest(ctx context.Context, addresses [][3]byte, continous bool) (Packet, error) {
	data := make([]byte, 1+len(addresses)*3)
	if continous {
		data[0] = 0x01
	} else {
		data[0] = 0x00
	}

	for i, set := range addresses {
		for j, a := range set {
			data[(i*3)+j+1] = a
		}
	}

	p := newPacket(deviceDiagnosticTool, deviceEngine, commandReadAddressesRequest, data)
	rp, err := c.sendPacket(ctx, p)
	if err != nil {
		return nil, errors.Wrap(err, "sending packet")
	}

	if rp[PacketIndexCommand] != byte(commandReadAddressesResponse) {
		return nil, errors.New(fmt.Sprintf("unexpected response code %x (expected %x)",
			rp[PacketIndexCommand], commandReadAddressesResponse))
	}

	return rp, nil
}

func (c *Connection) sendPacket(ctx context.Context, p Packet) (Packet, error) {
	fmt.Print("sending packet: ")
	for _, b := range p {
		fmt.Printf("0x%x ", b)
	}
	fmt.Println()

	wb, err := c.serialPort.Write(p)
	if err != nil {
		return nil, errors.Wrap(err, "writing packet bytes")
	}

	if wb != len(p) {
		return nil, errors.Wrapf(err, "only wrote %d bytes (packet had %d bytes)", wb, len(p))
	}

	// wait for the written bytes to transfer
	waitForNBytesToTransfer(len(p))

	sentCommand := p[PacketIndexCommand]
	currentCommand := sentCommand
	for currentCommand == sentCommand { // make sure we aren't reading back the packet we just sent
		resp, err := c.NextPacket(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "reading second response packet")
		}
		currentCommand = resp[PacketIndexCommand]
		if currentCommand == sentCommand {
			fmt.Println("read back same command")
			fmt.Printf("packets are equal: %v\n", string(resp) == string(p))
		}
	}

	return p, nil
}

// NextPacket reads the next packet from the ECU.
func (c *Connection) NextPacket(ctx context.Context) (Packet, error) {
	fmt.Println("reading next packet")

	header := make([]byte, PacketHeaderSize)
	fmt.Printf("reading %d header bytes\n", PacketHeaderSize)
	err := c.readInFull(ctx, header)
	if err != nil {
		return nil, errors.Wrap(err, "reading packet header")
	}
	if err = validateHeader(header); err != nil {
		return nil, errors.Wrap(err, "invalid packet header")
	}

	payload := make([]byte, int(header[PacketIndexPayloadSize]))
	fmt.Printf("reading %d payload bytes\n", len(payload))
	err = c.readInFull(ctx, payload)
	if err != nil {
		return nil, errors.Wrap(err, "reading packet payload")
	}

	return Packet(append(header, payload...)), nil
}

type readResult struct {
	count int
	err   error
}

var ErrorReadTimeout = errors.New("the read operation timed out")

func (c *Connection) readInFull(ctx context.Context, b []byte) error {
	// start a goroutine to read the buffer in full
	result := make(chan readResult, 1)
	go func(ctx context.Context, result chan<- readResult, b []byte) {
		readCount := 0
		for readCount < len(b) {
			waitForNBytesToTransfer(len(b) - readCount)
			fmt.Println("starting read")

			// start a goroutine to call read on the port
			result := make(chan readResult, 1)
			go func(r chan<- readResult, b []byte) {
				count, err := c.serialPort.Read(b)
				r <- readResult{count, err}
			}(result, b[readCount:])

			select {
			case <-ctx.Done():
				result <- readResult{readCount, ctx.Err()}
				return
			case <-time.NewTimer(ConnectionReadTimeout).C:
				result <- readResult{readCount, ErrorReadTimeout}
				return
			case r := <-result:
				if r.count > 0 {
					fmt.Print("read: ")
					for _, bb := range b[readCount : readCount+r.count] {
						fmt.Printf("0x%x ", bb)
					}
					fmt.Println()
				}
				readCount += r.count

				if r.err != nil {
					result <- readResult{readCount, r.err}
					return
				}
			}
		}
	}(ctx, result, b)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.NewTimer(ConnectionTotalReadTimeout).C:
		return ErrorReadTimeout
	case r := <-result:
		return r.err
	}
}

func waitForNBytesToTransfer(n int) {
	ms := microsecondsOnTheWire(n)
	fmt.Printf("waiting %s for %d bytes\n", ms, n)
	<-time.NewTimer(ms).C
}

// baud rate = bits per second
// baud (bits/s) * 1s/1,000,000 µs = baud rate in µs
// word = start bit (1) + data bits (8) + stop bits (1) = 10 bits
// since we use 8 data bits, and 8 bits = 1 byte, we are transmitting 1 word per byte of data
// so, creating an equation...
// baud rate in µs = 4800bits/1,000,000µs = 10bits*byteCount/xµs = wordSize*wordCount/x
// x = (10*byteCount*1,000,000)/4800 µs = (byteCount*10,000,000)/4800 µs
func microsecondsOnTheWire(byteCount int) time.Duration {
	return time.Duration(int(math.Round(
		float64(byteCount*10000000)/float64(ConnectionBaudRate),
	))) * time.Microsecond
}
