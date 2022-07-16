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

		conn := ssm2.NewConnection(port, ssm2.NopLogger)

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

		conn := ssm2.NewConnection(port, ssm2.NopLogger)

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

		conn := ssm2.NewConnection(port, ssm2.NopLogger)

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

		conn := ssm2.NewConnection(port, ssm2.NopLogger)

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

		conn := ssm2.NewConnection(port, ssm2.NopLogger)

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

		conn := ssm2.NewConnection(port, ssm2.NopLogger)

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

		conn := ssm2.NewConnection(port, ssm2.NopLogger)

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

		conn := ssm2.NewConnection(port, ssm2.NopLogger)

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

		conn := ssm2.NewConnection(port, ssm2.NopLogger)

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
		if ecu.SupportedParameters["P8"] == nil {
			t.Fatal("expected P8 param to be supported")
		}
		if ecu.SupportedParameters["P12"] == nil {
			t.Fatal("expected P12 param to be supported")
		}
		if ecu.SupportedParameters["P239"] == nil {
			t.Fatal("expected P239 param to be supported")
		}
		if ecu.SupportedParameters["P240"] == nil {
			t.Fatal("expected P240 param to be supported")
		}
		if ecu.SupportedParameters["P241"] == nil {
			t.Fatal("expected P241 param to be supported")
		}

		if len(ecu.SupportedDerivedParameters) != 1 {
			t.Fatalf("expected 1 supported derived param. got: %d.", len(ecu.SupportedDerivedParameters))
		}
		if ecu.SupportedDerivedParameters["P200"] == nil {
			t.Fatal("expected P200 derived param to be supported")
		}
	})
}
