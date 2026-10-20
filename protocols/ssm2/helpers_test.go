package ssm2_test

import (
	"context"
	"errors"

	"github.com/gavinwade12/ecLogger/protocols/ssm2"
)

// memoryConnection answers read address requests from an in-memory RAM map.
// Addresses missing from the map read as 0x00.
type memoryConnection struct {
	ram      map[[3]byte]byte
	requests [][][3]byte

	// truncate drops this many bytes from each response when set
	truncate int
}

func (c *memoryConnection) InitECU(ctx context.Context) (*ssm2.ECU, error) {
	return nil, errors.New("not implemented")
}

func (c *memoryConnection) SendReadAddressesRequest(ctx context.Context, addresses [][3]byte, continous bool) (ssm2.Packet, error) {
	c.requests = append(c.requests, append([][3]byte(nil), addresses...))

	p := ssm2.Packet{
		ssm2.PacketMagicByte, ssm2.DeviceDiagnosticTool, ssm2.DeviceEngine,
		0x00, ssm2.CommandReadAddressesResponse,
	}
	for _, a := range addresses[:len(addresses)-c.truncate] {
		p = append(p, c.ram[a])
	}
	p[ssm2.PacketIndexPayloadSize] = byte(len(p) - ssm2.PacketHeaderSize + 1)
	return append(p, 0x00), nil
}

func (c *memoryConnection) NextPacket(ctx context.Context) (ssm2.Packet, error) {
	return nil, errors.New("not implemented")
}

func (c *memoryConnection) Close() error { return nil }
