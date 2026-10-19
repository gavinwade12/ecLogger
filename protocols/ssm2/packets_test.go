package ssm2_test

import (
	"bytes"
	"testing"

	"github.com/gavinwade12/ecLogger/protocols/ssm2"
)

func TestNewPacket(t *testing.T) {
	tests := []struct {
		name string
		src  byte
		dest byte
		cmd  byte
		data []byte
		want ssm2.Packet
	}{
		{
			"no data",
			ssm2.DeviceDiagnosticTool, ssm2.DeviceEngine, ssm2.CommandInitRequest, nil,
			ssm2.Packet{0x80, 0x10, 0xf0, 0x01, 0xbf, 0x40},
		},
		{
			"with data",
			ssm2.DeviceEngine, ssm2.DeviceDiagnosticTool, ssm2.CommandReadAddressesResponse, []byte{0xaa, 0xbb},
			ssm2.Packet{0x80, 0xf0, 0x10, 0x03, 0xe8, 0xaa, 0xbb, 0xd0},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ssm2.NewPacket(tt.src, tt.dest, tt.cmd, tt.data)
			if !bytes.Equal(got, tt.want) {
				t.Errorf("NewPacket() = %x, want %x", []byte(got), []byte(tt.want))
			}
			if !bytes.Equal(got.Data(), tt.data) {
				t.Errorf("Data() = %x, want %x", got.Data(), tt.data)
			}
		})
	}
}

func TestCalculateChecksum(t *testing.T) {
	tests := []struct {
		name   string
		packet ssm2.Packet
		want   byte
	}{
		{"init request", initRequestPacket, 0x40},
		{"overflow wraps", ssm2.Packet{0xff, 0xff, 0x03, 0x00}, 0x01},
		{"checksum byte is ignored", ssm2.Packet{0x01, 0x02, 0xff}, 0x03},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ssm2.CalculateChecksum(tt.packet); got != tt.want {
				t.Errorf("CalculateChecksum() = %#x, want %#x", got, tt.want)
			}
		})
	}
}

func TestPacket_Data(t *testing.T) {
	p := ssm2.Packet{
		ssm2.PacketMagicByte, ssm2.DeviceDiagnosticTool, ssm2.DeviceEngine,
		0x03, ssm2.CommandReadAddressesResponse, 0xaa, 0xbb, 0x00,
	}
	if got := p.Data(); !bytes.Equal(got, []byte{0xaa, 0xbb}) {
		t.Errorf("Data() = %x, want aabb", got)
	}

	empty := ssm2.Packet{
		ssm2.PacketMagicByte, ssm2.DeviceDiagnosticTool, ssm2.DeviceEngine,
		0x01, ssm2.CommandReadAddressesResponse, 0x00,
	}
	if got := empty.Data(); len(got) != 0 {
		t.Errorf("Data() = %x, want empty", got)
	}
}
