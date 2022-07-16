package ssm2

import (
	"bytes"
	"errors"
	"fmt"
)

// Packet defines the common packet structure used in requests and respones.
// The format is:
//	Magic byte (0x80)
//	Dest
//	Src
//	Payload Size (count of data bytes + 1 checksum byte)
//	Command
// 	Data
// 	Checksum
type Packet []byte

// Constant values used to describe pieces of a packet.
const (
	PacketIndexMagicByte    int = 0
	PacketIndexDestination  int = 1
	PacketIndexSource       int = 2
	PacketIndexPayloadSize  int = 3
	PacketIndexCommand      int = 4
	PacketIndexPayloadStart int = 5

	PacketMagicByte byte = 0x80

	PacketHeaderSize int = 5
)

func (p Packet) Data() []byte {
	return p[PacketIndexPayloadStart : len(p)-1]
}

func newPacket(src, dest byte, cmd byte, data []byte) Packet {
	packet := make(Packet, PacketHeaderSize+len(data)+1)
	packet[PacketIndexMagicByte] = PacketMagicByte
	packet[PacketIndexDestination] = byte(dest)
	packet[PacketIndexSource] = byte(src)
	packet[PacketIndexPayloadSize] = byte(len(data)) + 1
	packet[PacketIndexCommand] = byte(cmd)

	if len(data) > 0 {
		copy(packet[PacketIndexPayloadStart:len(packet)-1], data)
	}

	packet[len(packet)-1] = calculateChecksum(packet)

	return packet
}

func calculateChecksum(p Packet) byte {
	checksum := 0
	for _, b := range p[:len(p)-1] {
		checksum += int(b)
	}
	return byte(checksum)
}

var devices = []byte{
	DeviceEngine, DeviceTransmission, DeviceDiagnosticTool, DeviceFastModeDiagnosticTool,
}

var commands = []byte{
	CommandReadBlockRequest, CommandReadBlockResponse,
	CommandReadAddressesRequest, CommandReadAddressesResponse,
	CommandWriteBlockRequest, CommandWriteBlockResponse,
	CommandWriteAddressRequest, CommandWriteAddressResponse,
	CommandInitRequest, CommandInitResponse,
}

func validateHeader(b []byte) error {
	if len(b) != PacketHeaderSize {
		return fmt.Errorf("invalid header size: %d", len(b))
	}
	if b[PacketIndexMagicByte] != PacketMagicByte {
		return errors.New("invalid magic byte")
	}
	if !bytes.Contains(devices, []byte{b[PacketIndexDestination]}) {
		return errors.New("invalid destination")
	}
	if !bytes.Contains(devices, []byte{b[PacketIndexSource]}) {
		return errors.New("invalid source")
	}
	if !bytes.Contains(commands, []byte{b[PacketIndexCommand]}) {
		return errors.New("invalid command")
	}
	if b[PacketIndexPayloadSize] < 1 {
		return errors.New("invalid payload size")
	}
	return nil
}
