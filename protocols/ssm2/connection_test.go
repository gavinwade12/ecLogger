package ssm2_test

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/gavinwade12/ecLogger/protocols/ssm2"
)

// testSerialPort reads from out and writes to in. Like a real port with a
// read timeout, reading with nothing to read returns no bytes and no error.
// The connection reads on a background goroutine, so access is guarded by mu.
type testSerialPort struct {
	mu     sync.Mutex
	out    *bytes.Buffer
	in     *bytes.Buffer
	closed bool
}

func (p *testSerialPort) Read(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.out.Len() == 0 {
		time.Sleep(time.Millisecond)
		return 0, nil
	}
	return p.out.Read(b)
}

// setOut replaces the bytes to be read.
func (p *testSerialPort) setOut(b []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.out = bytes.NewBuffer(b)
}

// written returns the bytes written to the port, and clears them.
func (p *testSerialPort) written() []byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	b := p.in.Bytes()
	p.in = &bytes.Buffer{}
	return b
}

func (p *testSerialPort) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.in.Write(b)
}

func (p *testSerialPort) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	return nil
}

func newTestSerialPort() *testSerialPort {
	return &testSerialPort{
		out: &bytes.Buffer{},
		in:  &bytes.Buffer{},
	}
}

func calculateChecksum(b []byte) byte {
	sum := 0
	for _, p := range b {
		sum += int(p)
	}
	return byte(sum)
}

// minimalInitResponse returns an init response containing only
// the SSM ID and ROM ID, with no capability bytes.
func minimalInitResponse() []byte {
	ids := make([]byte, 8)
	resp := []byte{
		ssm2.PacketMagicByte, ssm2.DeviceDiagnosticTool, ssm2.DeviceEngine,
		byte(len(ids) + 1), ssm2.CommandInitResponse,
	}
	resp = append(resp, ids...)
	return append(resp, calculateChecksum(resp))
}

var initRequestPacket = []byte{
	ssm2.PacketMagicByte, ssm2.DeviceEngine, ssm2.DeviceDiagnosticTool,
	0x01, ssm2.CommandInitRequest, 0x40,
}

func TestSendingPacket(t *testing.T) {
	t.Run("ChecksResponseMagicByte", func(t *testing.T) {
		port := newTestSerialPort()

		resp := []byte{
			ssm2.PacketMagicByte + 1, ssm2.DeviceDiagnosticTool, ssm2.DeviceEngine,
			0x01, ssm2.CommandInitResponse,
		}
		resp = append(resp, calculateChecksum(resp))
		port.out = bytes.NewBuffer(resp)

		conn := ssm2.NewConnection(port, nil)

		// no magic byte is found, so it waits for more bytes until the deadline
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		_, err := conn.InitECU(ctx)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ChecksResponseDest", func(t *testing.T) {
		port := newTestSerialPort()

		resp := []byte{
			ssm2.PacketMagicByte, ssm2.CommandInitRequest, ssm2.DeviceEngine,
			0x01, ssm2.CommandInitResponse,
		}
		resp = append(resp, calculateChecksum(resp))
		port.out = bytes.NewBuffer(resp)

		conn := ssm2.NewConnection(port, nil)

		_, err := conn.InitECU(context.Background())
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ChecksResponseSource", func(t *testing.T) {
		port := newTestSerialPort()

		resp := []byte{
			ssm2.PacketMagicByte, ssm2.DeviceDiagnosticTool, ssm2.CommandInitRequest,
			0x01, ssm2.CommandInitResponse,
		}
		resp = append(resp, calculateChecksum(resp))
		port.out = bytes.NewBuffer(resp)

		conn := ssm2.NewConnection(port, nil)

		_, err := conn.InitECU(context.Background())
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ChecksResponseCommand", func(t *testing.T) {
		port := newTestSerialPort()

		resp := []byte{
			ssm2.PacketMagicByte, ssm2.DeviceDiagnosticTool, ssm2.DeviceEngine,
			0x01, ssm2.DeviceEngine,
		}
		resp = append(resp, calculateChecksum(resp))
		port.out = bytes.NewBuffer(resp)

		conn := ssm2.NewConnection(port, nil)

		_, err := conn.InitECU(context.Background())
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ChecksResponseChecksum", func(t *testing.T) {
		port := newTestSerialPort()

		resp := []byte{
			ssm2.PacketMagicByte, ssm2.DeviceDiagnosticTool, ssm2.DeviceEngine,
			0x01, ssm2.CommandInitResponse,
		}
		resp = append(resp, calculateChecksum(resp)+1)
		port.out = bytes.NewBuffer(resp)

		conn := ssm2.NewConnection(port, nil)

		_, err := conn.InitECU(context.Background())
		if !errors.Is(err, ssm2.ErrInvalidChecksumByte) {
			t.Fatalf("want ErrInvalidChecksumByte (%v). got: %v.", ssm2.ErrInvalidChecksumByte, err)
		}
	})

	t.Run("IgnoresAnEchoedRequest", func(t *testing.T) {
		port := newTestSerialPort()

		resp := append(initRequestPacket, minimalInitResponse()...)
		port.out = bytes.NewBuffer(resp)

		conn := ssm2.NewConnection(port, nil)

		_, err := conn.InitECU(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		port.mu.Lock()
		defer port.mu.Unlock()
		if port.out.Len() != 0 {
			t.Fatal("expected the buffer to have been read in full")
		}
	})
}

func TestInitECU(t *testing.T) {
	t.Run("ValidRequest", func(t *testing.T) {
		port := newTestSerialPort()
		port.out = bytes.NewBuffer(minimalInitResponse())

		conn := ssm2.NewConnection(port, nil)

		_, err := conn.InitECU(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		got := port.written()
		if !bytes.Equal(initRequestPacket, got) {
			t.Fatalf("unexpected init request. want: 0x%x. got: 0x%x.", initRequestPacket, got)
		}
	})

	t.Run("ChecksResponseCommand", func(t *testing.T) {
		port := newTestSerialPort()

		resp := []byte{
			ssm2.PacketMagicByte, ssm2.DeviceDiagnosticTool, ssm2.DeviceEngine,
			0x01, ssm2.CommandReadAddressesResponse,
		}
		resp = append(resp, calculateChecksum(resp))
		port.out = bytes.NewBuffer(resp)

		conn := ssm2.NewConnection(port, nil)

		_, err := conn.InitECU(context.Background())
		if !errors.Is(err, ssm2.ErrInvalidResponseCommand) {
			t.Fatalf("want ErrInvalidResponseCommand (%v). got: %v.", ssm2.ErrInvalidResponseCommand, err)
		}
	})

	t.Run("RejectsShortResponse", func(t *testing.T) {
		port := newTestSerialPort()
		resp := []byte{
			ssm2.PacketMagicByte, ssm2.DeviceDiagnosticTool, ssm2.DeviceEngine,
			0x01, ssm2.CommandInitResponse,
		}
		port.out = bytes.NewBuffer(append(resp, calculateChecksum(resp)))
		conn := ssm2.NewConnection(port, nil)

		_, err := conn.InitECU(context.Background())
		if !errors.Is(err, ssm2.ErrInvalidResponseLength) {
			t.Fatalf("want ErrInvalidResponseLength. got: %v.", err)
		}
	})

	t.Run("ValidResponse", func(t *testing.T) {
		port := newTestSerialPort()

		ssmID := []byte{0x02, 0x03, 0x04}
		romID := []byte{0x10, 0x40, 0xA1, 0x32, 0xB1}
		resp := []byte{
			ssm2.PacketMagicByte, ssm2.DeviceDiagnosticTool, ssm2.DeviceEngine,
			byte(len(ssmID) + len(romID) + 3), ssm2.CommandInitResponse,
		}
		resp = append(resp, ssmID...)
		resp = append(resp, romID...)
		resp = append(resp, 0b00000001) // enable P8, P239, P240, P241
		resp = append(resp, 0b00010000) // enable P12
		resp = append(resp, calculateChecksum(resp))
		port.out = bytes.NewBuffer(resp)

		conn := ssm2.NewConnection(port, nil)

		ecu, err := conn.InitECU(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		if ecu == nil {
			t.Fatal("ecu is nil")
		}

		if !bytes.Equal(ecu.SSM_ID, ssmID) {
			t.Fatalf("invalid SSM_ID. want: %x. got: %x.", ssmID, ecu.SSM_ID)
		}
		if !bytes.Equal(ecu.ROM_ID, romID) {
			t.Fatalf("invalid ROM_ID. want: %x. got: %x.", romID, ecu.ROM_ID)
		}

		if len(ecu.SupportedParameters) != 5 {
			t.Fatalf("expected 5 supported params (P8, P12, P239, P240, P241). got: %d (%v).", len(ecu.SupportedParameters), ecu.SupportedParameters)
		}

		wants := []string{"P8", "P12", "P239", "P240", "P241"}
		for _, want := range wants {
			supported := false
			for _, p := range ecu.SupportedParameters {
				if p.Id == want {
					supported = true
					break
				}
			}
			if !supported {
				t.Fatalf("expected %s param to be supported", want)
			}
		}

		if len(ecu.SupportedDerivedParameters) != 1 {
			t.Fatalf("expected 1 supported derived param. got: %d.", len(ecu.SupportedDerivedParameters))
		}
		if ecu.SupportedDerivedParameters[0].Id != "P200" {
			t.Fatal("expected P200 derived param to be supported")
		}
	})
}

func TestSendReadAddressesRequest(t *testing.T) {
	addresses := [][3]byte{
		{0x00, 0x00, 0x01},
		{0x00, 0x00, 0x0A},
	}

	t.Run("ValidRequest", func(t *testing.T) {
		port := newTestSerialPort()

		resp := []byte{
			ssm2.PacketMagicByte, ssm2.DeviceDiagnosticTool, ssm2.DeviceEngine,
			0x01, ssm2.CommandReadAddressesResponse,
		}
		resp = append(resp, calculateChecksum(resp))
		port.out = bytes.NewBuffer(resp)

		conn := ssm2.NewConnection(port, nil)

		_, err := conn.SendReadAddressesRequest(context.Background(), addresses, false)
		if err != nil {
			t.Fatal(err)
		}

		got := port.written()
		want := []byte{
			ssm2.PacketMagicByte, ssm2.DeviceEngine, ssm2.DeviceDiagnosticTool,
			byte((len(addresses) * 3) + 2), ssm2.CommandReadAddressesRequest, 0x00,
		}
		for _, a := range addresses {
			want = append(want, a[:]...)
		}
		want = append(want, calculateChecksum(want))

		if !bytes.Equal(want, got) {
			t.Fatalf("unexpected read addresses request. want: 0x%x. got: 0x%x.", want, got)
		}

		port.setOut(resp)
		_, err = conn.SendReadAddressesRequest(context.Background(), addresses, true)
		if err != nil {
			t.Fatal(err)
		}

		got = port.written()
		want = []byte{
			ssm2.PacketMagicByte, ssm2.DeviceEngine, ssm2.DeviceDiagnosticTool,
			byte((len(addresses) * 3) + 2), ssm2.CommandReadAddressesRequest, 0x01,
		}
		for _, a := range addresses {
			want = append(want, a[:]...)
		}
		want = append(want, calculateChecksum(want))

		if !bytes.Equal(want, got) {
			t.Fatalf("unexpected read addresses request. want: 0x%x. got: 0x%x.", want, got)
		}
	})

	t.Run("ChecksResponseCommand", func(t *testing.T) {
		port := newTestSerialPort()

		resp := []byte{
			ssm2.PacketMagicByte, ssm2.DeviceDiagnosticTool, ssm2.DeviceEngine,
			0x01, ssm2.CommandWriteBlockResponse,
		}
		resp = append(resp, calculateChecksum(resp))
		port.out = bytes.NewBuffer(resp)

		conn := ssm2.NewConnection(port, nil)

		_, err := conn.SendReadAddressesRequest(context.Background(), addresses, false)
		if !errors.Is(err, ssm2.ErrInvalidResponseCommand) {
			t.Fatalf("want ErrInvalidResponseCommand (%v). got: %v.", ssm2.ErrInvalidResponseCommand, err)
		}
	})

	t.Run("ValidResponse", func(t *testing.T) {
		port := newTestSerialPort()

		values := []byte{0x20, 0xA1}
		resp := []byte{
			ssm2.PacketMagicByte, ssm2.DeviceDiagnosticTool, ssm2.DeviceEngine,
			byte(len(values) + 1), ssm2.CommandReadAddressesResponse,
		}
		resp = append(resp, values...)
		resp = append(resp, calculateChecksum(resp))
		port.out = bytes.NewBuffer(resp)

		conn := ssm2.NewConnection(port, nil)

		packet, err := conn.SendReadAddressesRequest(context.Background(), addresses, false)
		if err != nil {
			t.Fatal(err)
		}

		data := packet.Data()
		if len(data) != 2 {
			t.Fatalf("invalid data length. want: 2. got: %d.", len(data))
		}
		if data[0] != values[0] {
			t.Fatalf("invalid first byte. want: %x. got: %x.", values[0], data[0])
		}
		if data[1] != values[1] {
			t.Fatalf("invalid second byte. want: %x. got: %x.", values[1], data[1])
		}
	})
}

// shortWritePort reports writing fewer bytes than it was given.
type shortWritePort struct{ *testSerialPort }

func (p *shortWritePort) Write(b []byte) (int, error) { return len(b) - 1, nil }

// errPort fails every read.
type errPort struct{ *testSerialPort }

func (p *errPort) Read(b []byte) (int, error) { return 0, errors.New("boom") }

func TestConnectionErrors(t *testing.T) {
	t.Run("ShortWrite", func(t *testing.T) {
		port := &shortWritePort{newTestSerialPort()}
		conn := ssm2.NewConnection(port, nil)

		_, err := conn.InitECU(context.Background())
		if err == nil {
			t.Fatal("expected an error for a short write")
		}
	})

	t.Run("ReadError", func(t *testing.T) {
		port := &errPort{newTestSerialPort()}
		conn := ssm2.NewConnection(port, nil)

		_, err := conn.NextPacket(context.Background())
		if err == nil {
			t.Fatal("expected the read error to be returned")
		}
	})

	t.Run("CanceledContext", func(t *testing.T) {
		port := newTestSerialPort()
		port.out = bytes.NewBuffer(minimalInitResponse())
		conn := ssm2.NewConnection(port, nil)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := conn.NextPacket(ctx)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("want context.Canceled. got: %v.", err)
		}
	})
}

func TestNextPacketResync(t *testing.T) {
	t.Run("SkipsLeadingGarbage", func(t *testing.T) {
		port := newTestSerialPort()
		// two stray bytes from a previous packet precede a valid packet
		port.out = bytes.NewBuffer(append([]byte{0x01, 0x02}, minimalInitResponse()...))
		conn := ssm2.NewConnection(port, nil)

		p, err := conn.NextPacket(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(p, minimalInitResponse()) {
			t.Fatalf("unexpected packet. want: %x. got: %x.", minimalInitResponse(), p)
		}
	})

	t.Run("SkipsAWholeHeaderOfGarbage", func(t *testing.T) {
		port := newTestSerialPort()
		garbage := make([]byte, ssm2.PacketHeaderSize)
		port.out = bytes.NewBuffer(append(garbage, minimalInitResponse()...))
		conn := ssm2.NewConnection(port, nil)

		p, err := conn.NextPacket(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(p, minimalInitResponse()) {
			t.Fatalf("unexpected packet. want: %x. got: %x.", minimalInitResponse(), p)
		}
	})
}

// blockingPort returns each chunk sent on chunks from a separate Read,
// blocking until one is available.
type blockingPort struct {
	testSerialPort
	chunks chan []byte
}

func (p *blockingPort) Read(b []byte) (int, error) {
	return copy(b, <-p.chunks), nil
}

func TestCanceledReadDoesNotLoseTheNextPacket(t *testing.T) {
	port := &blockingPort{chunks: make(chan []byte)}
	conn := ssm2.NewConnection(port, nil)

	// nothing arrives before the read is canceled
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := conn.NextPacket(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want context.DeadlineExceeded. got: %v.", err)
	}

	// then the next packet arrives, in two parts
	resp := minimalInitResponse()
	go func() {
		port.chunks <- resp[:3]
		port.chunks <- resp[3:]
	}()

	p, err := conn.NextPacket(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(p, resp) {
		t.Fatalf("unexpected packet. want: %x. got: %x.", resp, p)
	}
}

func TestReadOnClosedConnection(t *testing.T) {
	port := &blockingPort{chunks: make(chan []byte)}
	conn := ssm2.NewConnection(port, nil)

	go func() {
		time.Sleep(20 * time.Millisecond)
		conn.Close()
	}()
	start := time.Now()
	if _, err := conn.NextPacket(context.Background()); !errors.Is(err, ssm2.ErrConnectionClosed) {
		t.Fatalf("want ErrConnectionClosed. got: %v.", err)
	}
	if d := time.Since(start); d > time.Second {
		t.Fatalf("closing took %s to interrupt the read", d)
	}

	if _, err := conn.NextPacket(context.Background()); !errors.Is(err, ssm2.ErrConnectionClosed) {
		t.Fatalf("want ErrConnectionClosed after closing. got: %v.", err)
	}
}
