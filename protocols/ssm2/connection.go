package ssm2

import (
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"time"

	"github.com/pkg/errors"
)

// Connection provides high-level methods for communicating with an ECU
// via the SSM2 protocol.
type Connection interface {
	InitECU(ctx context.Context) (*ECU, error)
	SendReadAddressesRequest(ctx context.Context, addresses [][3]byte, continous bool) (Packet, error)
	NextPacket(ctx context.Context) (Packet, error)
	Close() error
}

type connection struct {
	serialPort io.ReadWriteCloser
	logger     Logger
}

const (
	// ConnectionBaudRate is the baud rate (bits/s) used for the serial connection.
	ConnectionBaudRate int = 4800
	// ConnectionDataBits is the data bit setting (bits/word) used for the serial connection.
	ConnectionDataBits int = 8
	// ConnectionReadTimeout is the amount of time per read spent before a timeout occurs.
	// The timeout is per read, but it may take several reads to consume an entire packet.
	ConnectionReadTimeout time.Duration = time.Millisecond * 1500
	// ConnectionTotalReadTimeout is the amount of time spent to read an entire buffer before
	// a timeout occurs. This applies to the full-length read and not individual reads.
	ConnectionTotalReadTimeout time.Duration = time.Millisecond * 5000
)

// ErrReadTimeout is returned when reading a packet times out.
var ErrReadTimeout = errors.New("the read operation timed out")

// NewConnection returns a new Connection.
func NewConnection(serialPort io.ReadWriteCloser, l Logger) Connection {
	if l == nil {
		l = NopLogger
	}
	return &connection{
		serialPort: serialPort,
		logger:     l,
	}
}

// InitECU sends an init Command to the ECU and parses the response.
func (c *connection) InitECU(ctx context.Context) (*ECU, error) {
	p := newPacket(DeviceDiagnosticTool, DeviceEngine, CommandInitRequest, nil)
	rp, err := c.sendPacket(ctx, p)
	if err != nil {
		return nil, errors.Wrap(err, "sending packet")
	}

	if rp[PacketIndexCommand] != byte(CommandInitResponse) {
		return nil, ErrInvalidResponseCommand
	}

	return parseECUFromInitResponse(rp), nil
}

// ReadAddressses sends a read addresses request to the ECU. The results should be fetched via NextPacket().
// When continous is true, NextPacket() will continue to return results for the given addresses until the ECU
// is interrupted.
func (c *connection) SendReadAddressesRequest(ctx context.Context, addresses [][3]byte, continous bool) (Packet, error) {
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

	p := newPacket(DeviceDiagnosticTool, DeviceEngine, CommandReadAddressesRequest, data)
	rp, err := c.sendPacket(ctx, p)
	if err != nil {
		return nil, errors.Wrap(err, "sending packet")
	}

	if rp[PacketIndexCommand] != byte(CommandReadAddressesResponse) {
		return nil, ErrInvalidResponseCommand
	}

	return rp, nil
}

func (c *connection) sendPacket(ctx context.Context, p Packet) (Packet, error) {
	logBytes(c.logger, p, "sending packet: ")

	wb, err := c.serialPort.Write(p)
	if err != nil {
		return nil, errors.Wrap(err, "writing packet bytes")
	}
	if wb != len(p) {
		return nil, errors.Wrapf(err, "only wrote %d bytes (packet had %d bytes)", wb, len(p))
	}

	sentCommand := p[PacketIndexCommand]
	currentCommand := sentCommand
	for currentCommand == sentCommand { // make sure we aren't reading back the packet we just sent
		p, err = c.NextPacket(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "reading response packet")
		}
		currentCommand = p[PacketIndexCommand]
		if currentCommand == sentCommand {
			c.logger.Debug("read back same command")
		}
	}

	return p, nil
}

// NextPacket reads the next packet from the ECU.
func (c *connection) NextPacket(ctx context.Context) (Packet, error) {
	c.logger.Debug("reading next packet")

	header := make([]byte, PacketHeaderSize)
	c.logger.Debugf("reading %d header bytes\n", PacketHeaderSize)
	err := c.readInFull(ctx, header)
	if err != nil {
		return nil, errors.Wrap(err, "reading packet header")
	}
	if err = validateHeader(header); err != nil {
		if header[0] == PacketMagicByte {
			return nil, errors.Wrap(err, "invalid packet header")
		}

		// let's try to find the next magic byte - maybe we're getting data from a previous packet
		var mbi *int
		for i, b := range header {
			if b == PacketMagicByte {
				mbi = &i
				break
			}
		}
		// the magic byte isn't within this buffer, so restart
		if mbi == nil {
			return c.NextPacket(ctx)
		}

		// read the remaining header bytes and re-validate
		rem := make([]byte, *mbi)
		err := c.readInFull(ctx, rem)
		if err != nil {
			return nil, errors.Wrap(err, "reading packet header")
		}
		header = append(header[*mbi:], rem...)
		if err = validateHeader(header); err != nil {
			return nil, errors.Wrap(err, "invalid packet header")
		}
	}

	payload := make([]byte, int(header[PacketIndexPayloadSize]))
	c.logger.Debugf("reading %d payload bytes\n", len(payload))
	err = c.readInFull(ctx, payload)
	if err != nil {
		return nil, errors.Wrap(err, "reading packet payload")
	}

	packet := Packet(append(header, payload...))
	checksum := packet[len(packet)-1]
	calculatedChecksum := CalculateChecksum(packet)
	if checksum != calculatedChecksum {
		c.logger.Debugf("invalid checksum. want: %x. got: %x.\n", calculatedChecksum, checksum)
		return nil, ErrInvalidChecksumByte
	}
	return packet, nil
}

type readResult struct {
	count int
	err   error
}

func (c *connection) readInFull(ctx context.Context, b []byte) error {
	// start a goroutine to read the buffer in full
	result := make(chan readResult, 1)
	go func(ctx context.Context, result chan<- readResult, b []byte) {
		readCount := 0
		for readCount < len(b) {
			if err := c.waitForNBytesToTransfer(ctx, len(b)-readCount); err != nil {
				result <- readResult{readCount, err}
			}

			c.logger.Debug("starting read")
			count, err := c.serialPort.Read(b[readCount:])

			if count > 0 {
				logBytes(c.logger, b[readCount:readCount+count], "read: ")
			}
			readCount += count

			if err != nil {
				result <- readResult{readCount, err}
				return
			}
		}
		result <- readResult{readCount, nil}
	}(ctx, result, b)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.NewTimer(ConnectionTotalReadTimeout).C:
		return ErrReadTimeout
	case r := <-result:
		return r.err
	}
}

func (c *connection) Close() error {
	c.logger.Debug("closing connection")

	if c.serialPort != nil {
		return c.serialPort.Close()
	}

	return nil
}

func (c *connection) waitForNBytesToTransfer(ctx context.Context, n int) error {
	ms := microsecondsOnTheWire(n)
	c.logger.Debugf("waiting %s for %d bytes\n", ms, n)
	select {
	case <-time.NewTimer(ms).C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
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

type Logger interface {
	Debug(message string)
	Debugf(message string, args ...interface{})
}

type nopLogger struct{}

func (l nopLogger) Debug(message string) {}

func (l nopLogger) Debugf(message string, args ...interface{}) {}

var NopLogger Logger = nopLogger{}

type defaultLogger struct {
	l *log.Logger
}

func (l *defaultLogger) Debug(message string) {
	l.l.Println(message)
}

func (l *defaultLogger) Debugf(message string, args ...interface{}) {
	l.l.Printf(message, args...)
}

var DefaultLogger = func(out io.Writer) Logger {
	return &defaultLogger{log.New(out, "SSM2 ", log.LstdFlags)}
}

func logBytes(l Logger, b []byte, prefix string) {
	s := prefix
	for _, bb := range b {
		s += fmt.Sprintf("0x%x ", bb)
	}
	l.Debug(s)
}
