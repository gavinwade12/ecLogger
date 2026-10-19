package ssm2

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"sync"
	"time"
)

// Connection provides high-level methods for communicating with an ECU
// via the SSM2 protocol.
type Connection interface {
	InitECU(ctx context.Context) (*ECU, error)
	SendReadAddressesRequest(ctx context.Context, addresses [][3]byte, continous bool) (Packet, error)
	NextPacket(ctx context.Context) (Packet, error)
	Close() error
}

// loggerFor returns the logger used by conn, or NopLogger if it doesn't expose one.
func loggerFor(conn Connection) Logger {
	if c, ok := conn.(interface{ logger() Logger }); ok {
		return c.logger()
	}
	return NopLogger
}

type connection struct {
	serialPort io.ReadWriteCloser
	log        Logger

	// reads receives what the background reader reads from the serial port.
	// It's nil when no reader is running. unread holds received bytes that
	// haven't been consumed yet.
	reads  chan readResult
	unread []byte

	closed    chan struct{}
	closeOnce sync.Once
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
		log:        l,
		closed:     make(chan struct{}),
	}
}

// InitECU sends an init Command to the ECU and parses the response.
func (c *connection) InitECU(ctx context.Context) (*ECU, error) {
	p := NewPacket(DeviceDiagnosticTool, DeviceEngine, CommandInitRequest, nil)
	rp, err := c.sendPacket(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("sending packet: %w", err)
	}

	if rp[PacketIndexCommand] != byte(CommandInitResponse) {
		return nil, ErrInvalidResponseCommand
	}

	return parseECUFromInitResponse(rp)
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

	p := NewPacket(DeviceDiagnosticTool, DeviceEngine, CommandReadAddressesRequest, data)
	rp, err := c.sendPacket(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("sending packet: %w", err)
	}

	if rp[PacketIndexCommand] != byte(CommandReadAddressesResponse) {
		return nil, ErrInvalidResponseCommand
	}

	return rp, nil
}

func (c *connection) sendPacket(ctx context.Context, p Packet) (Packet, error) {
	logBytes(c.log, p, "sending packet: ")

	wb, err := c.serialPort.Write(p)
	if err != nil {
		return nil, fmt.Errorf("writing packet bytes: %w", err)
	}
	if wb != len(p) {
		return nil, fmt.Errorf("only wrote %d bytes (packet had %d bytes)", wb, len(p))
	}

	sentCommand := p[PacketIndexCommand]
	currentCommand := sentCommand
	for currentCommand == sentCommand { // make sure we aren't reading back the packet we just sent
		p, err = c.NextPacket(ctx)
		if err != nil {
			return nil, fmt.Errorf("reading response packet: %w", err)
		}
		currentCommand = p[PacketIndexCommand]
		if currentCommand == sentCommand {
			c.log.Debug("read back same command")
		}
	}

	return p, nil
}

// NextPacket reads the next packet from the ECU.
func (c *connection) NextPacket(ctx context.Context) (Packet, error) {
	return c.nextPacket(ctx, 1)
}

func (c *connection) nextPacket(ctx context.Context, attempt int) (Packet, error) {
	if attempt > 5 {
		return nil, fmt.Errorf("no valid packet could be read")
	}

	c.log.Debugf("reading next packet (attempt %d)\n", attempt)

	header := make([]byte, PacketHeaderSize)
	c.log.Debugf("reading %d header bytes\n", PacketHeaderSize)
	err := c.readInFull(ctx, header)
	if err != nil {
		return nil, fmt.Errorf("reading packet header: %w", err)
	}
	if err = validateHeader(header); err != nil {
		if header[0] == PacketMagicByte {
			return nil, fmt.Errorf("invalid packet header: %w", err)
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
			return c.nextPacket(ctx, attempt+1)
		}

		// read the remaining header bytes and re-validate
		rem := make([]byte, *mbi)
		err := c.readInFull(ctx, rem)
		if err != nil {
			return nil, fmt.Errorf("reading packet header: %w", err)
		}
		header = append(header[*mbi:], rem...)
		if err = validateHeader(header); err != nil {
			return nil, fmt.Errorf("invalid packet header: %w", err)
		}
	}

	payload := make([]byte, int(header[PacketIndexPayloadSize]))
	c.log.Debugf("reading %d payload bytes\n", len(payload))
	err = c.readInFull(ctx, payload)
	if err != nil {
		return nil, fmt.Errorf("reading packet payload: %w", err)
	}

	packet := Packet(append(header, payload...))
	checksum := packet[len(packet)-1]
	calculatedChecksum := CalculateChecksum(packet)
	if checksum != calculatedChecksum {
		c.log.Debugf("invalid checksum. want: %x. got: %x.\n", calculatedChecksum, checksum)
		return nil, ErrInvalidChecksumByte
	}
	return packet, nil
}

type readResult struct {
	data []byte
	err  error
}

// readInFull fills b with bytes from the serial port. The port is read by a
// background goroutine, so a read that's canceled or times out doesn't leave
// a goroutine behind to swallow the bytes of the next packet: anything that
// arrives later is kept for the next read. Bytes already copied into b are
// dropped; the next packet read resyncs on the magic byte.
func (c *connection) readInFull(ctx context.Context, b []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	n := copy(b, c.unread)
	c.unread = c.unread[n:]
	if n == len(b) {
		return nil
	}

	if c.reads == nil {
		c.reads = make(chan readResult, 16)
		go c.readSerialPort(c.reads)
	}

	timeout := time.NewTimer(ConnectionTotalReadTimeout)
	defer timeout.Stop()

	for n < len(b) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout.C:
			return ErrReadTimeout
		case r := <-c.reads:
			if r.err != nil {
				c.reads = nil // the reader stops after an error
				return r.err
			}
			logBytes(c.log, r.data, "read: ")
			copied := copy(b[n:], r.data)
			n += copied
			c.unread = append(c.unread, r.data[copied:]...)
		}
	}
	return nil
}

// readSerialPort reads from the serial port and sends the results on reads
// until a read fails or the connection is closed.
func (c *connection) readSerialPort(reads chan<- readResult) {
	buf := make([]byte, 256)
	for {
		count, err := c.serialPort.Read(buf)
		if count > 0 {
			select {
			case reads <- readResult{data: append([]byte(nil), buf[:count]...)}:
			case <-c.closed:
				return
			}
		}
		if err != nil {
			select {
			case reads <- readResult{err: err}:
			case <-c.closed:
			}
			return
		}
	}
}

func (c *connection) logger() Logger {
	return c.log
}

func (c *connection) Close() error {
	c.log.Debug("closing connection")
	c.closeOnce.Do(func() { close(c.closed) })

	if c.serialPort != nil {
		return c.serialPort.Close()
	}

	return nil
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
