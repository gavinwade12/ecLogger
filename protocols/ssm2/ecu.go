package ssm2

type ECU struct {
	SSM_ID []byte
	ROM_ID []byte

	SupportedParameters        map[string]*Parameter
	SupportedDerivedParameters map[string]*DerivedParameter
}

func parseECUFromInitResponse(p Packet) *ECU {
	data := p.Data()
	dLen := uint(len(data))

	ecu := &ECU{
		SSM_ID:                     data[:3],
		ROM_ID:                     data[3:8],
		SupportedParameters:        make(map[string]*Parameter),
		SupportedDerivedParameters: make(map[string]*DerivedParameter),
	}

	for _, p := range Parameters {
		if p.CapabilityByteIndex >= dLen {
			continue // capability byte isn't in the data
		}

		if (data[p.CapabilityByteIndex] & (1 << p.CapabilityBitIndex)) != 0 {
			ecu.SupportedParameters[p.Id] = &p
		}
	}
	for _, p := range DerivedParameters {
		supported := true
		for _, d := range p.DependsOnParameters {
			if ecu.SupportedParameters[d] == nil {
				supported = false
				break
			}
		}
		if supported {
			ecu.SupportedDerivedParameters[p.Id] = &p
		}
	}

	return ecu
}
