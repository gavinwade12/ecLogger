package ssm2

import (
	"bytes"
	"errors"
	"fmt"
)

// Packet defines the common packet structure used in requests and respones.
// The format is:
//
//	Magic byte (0x80)
//	Dest
//	Src
//	Payload Size (count of data bytes + 1 checksum byte)
//	Command
//	Data
//	Checksum
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

const (
	DeviceEngine                 byte = 0x10
	DeviceTransmission           byte = 0x18
	DeviceDiagnosticTool         byte = 0xf0
	DeviceFastModeDiagnosticTool byte = 0xf2

	CommandReadBlockRequest      byte = 0xa0
	CommandReadBlockResponse     byte = 0xe0
	CommandReadAddressesRequest  byte = 0xa8
	CommandReadAddressesResponse byte = 0xe8
	CommandWriteBlockRequest     byte = 0xb0
	CommandWriteBlockResponse    byte = 0xf0
	CommandWriteAddressRequest   byte = 0xb8
	CommandWriteAddressResponse  byte = 0xf8
	CommandInitRequest           byte = 0xbf
	CommandInitResponse          byte = 0xff
)

var (
	// ErrInvalidResponseCommand is returned when a packet is sent and
	// the ECU doesn't respond with the proper command corresponding to the
	// sent command.
	ErrInvalidResponseCommand = errors.New("invalid response command")

	// ErrInvalidChecksumByte is returned when a packet is received from
	// the ECU and the checksum byte doesn't match the calculated checksum byte.
	ErrInvalidChecksumByte = errors.New("invalid checksum byte")
)

// Data returns the section of the packet corresponding to the payload data.
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

	packet[len(packet)-1] = CalculateChecksum(packet)

	return packet
}

// CalculateChecksum calculates the checksum for a fully-allocated (including
// the checksum byte itself) packet.
func CalculateChecksum(p Packet) byte {
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

// ECU describes an ECU and the different parameters it supports.
type ECU struct {
	SSM_ID []byte
	ROM_ID []byte

	SupportedParameters        []Parameter
	SupportedDerivedParameters []DerivedParameter
}

func parseECUFromInitResponse(p Packet) *ECU {
	data := p.Data()
	dLen := uint(len(data))

	ecu := &ECU{
		SSM_ID:                     data[:3],
		ROM_ID:                     data[3:8],
		SupportedParameters:        make([]Parameter, 0),
		SupportedDerivedParameters: make([]DerivedParameter, 0),
	}

	for _, p := range Parameters {
		if p.CapabilityByteIndex >= dLen {
			continue // capability byte isn't in the data
		}

		if (data[p.CapabilityByteIndex] & (1 << p.CapabilityBitIndex)) != 0 {
			ecu.SupportedParameters = append(ecu.SupportedParameters, p)
		}
	}

	ecu.SupportedDerivedParameters = AvailableDerivedParameters(ecu.SupportedParameters)

	return ecu
}
