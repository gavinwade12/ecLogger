package ssm2_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/gavinwade12/ssm2/protocols/ssm2"
)

type testSerialPort struct {
	out    *bytes.Buffer
	in     *bytes.Buffer
	closed bool
}

func (p *testSerialPort) Read(b []byte) (int, error) {
	return p.out.Read(b)
}

func (p *testSerialPort) Write(b []byte) (int, error) {
	return p.in.Write(b)
}

func (p *testSerialPort) Close() error {
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

		_, err := conn.InitECU(context.Background())
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

		resp := []byte{
			ssm2.PacketMagicByte, ssm2.DeviceDiagnosticTool, ssm2.DeviceEngine,
			0x01, ssm2.CommandInitResponse,
		}
		resp = append(resp, calculateChecksum(resp))
		resp = append(initRequestPacket, resp...)
		port.out = bytes.NewBuffer(resp)

		conn := ssm2.NewConnection(port, nil)

		_, err := conn.InitECU(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		b := make([]byte, 1)
		_, err = port.out.Read(b)
		if err != io.EOF {
			t.Fatal("expected EOF / buffer to have been read in full")
		}
	})
}

func TestInitECU(t *testing.T) {
	t.Run("ValidRequest", func(t *testing.T) {
		port := newTestSerialPort()

		resp := []byte{
			ssm2.PacketMagicByte, ssm2.DeviceDiagnosticTool, ssm2.DeviceEngine,
			0x01, ssm2.CommandInitResponse,
		}
		resp = append(resp, calculateChecksum(resp))
		port.out = bytes.NewBuffer(resp)

		conn := ssm2.NewConnection(port, nil)

		_, err := conn.InitECU(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		got := port.in.Bytes()
		if bytes.Compare(initRequestPacket, got) != 0 {
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

		if bytes.Compare(ecu.SSM_ID, ssmID) != 0 {
			t.Fatalf("invalid SSM_ID. want: %x. got: %x.", ssmID, ecu.SSM_ID)
		}
		if bytes.Compare(ecu.ROM_ID, romID) != 0 {
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

		got := port.in.Bytes()
		want := []byte{
			ssm2.PacketMagicByte, ssm2.DeviceEngine, ssm2.DeviceDiagnosticTool,
			byte((len(addresses) * 3) + 2), ssm2.CommandReadAddressesRequest, 0x00,
		}
		for _, a := range addresses {
			want = append(want, a[:]...)
		}
		want = append(want, calculateChecksum(want))

		if bytes.Compare(want, got) != 0 {
			t.Fatalf("unexpected read addresses request. want: 0x%x. got: 0x%x.", want, got)
		}

		port.in = &bytes.Buffer{}
		port.out = bytes.NewBuffer(resp)
		_, err = conn.SendReadAddressesRequest(context.Background(), addresses, true)
		if err != nil {
			t.Fatal(err)
		}

		got = port.in.Bytes()
		want = []byte{
			ssm2.PacketMagicByte, ssm2.DeviceEngine, ssm2.DeviceDiagnosticTool,
			byte((len(addresses) * 3) + 2), ssm2.CommandReadAddressesRequest, 0x01,
		}
		for _, a := range addresses {
			want = append(want, a[:]...)
		}
		want = append(want, calculateChecksum(want))

		if bytes.Compare(want, got) != 0 {
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
