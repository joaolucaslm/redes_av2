package wire

import (
	"encoding/binary"
	"slices"
)

type RDTKind uint8

const (
	RDTData RDTKind = 1
	RDTAck  RDTKind = 2
	RDTNak  RDTKind = 3
)

func (k RDTKind) String() string {
	switch k {
	case RDTData:
		return "DATA"
	case RDTAck:
		return "ACK"
	case RDTNak:
		return "NAK"
	default:
		return "?"
	}
}

const RDTHeaderLen = 4

type RDTPacket struct {
	Kind     RDTKind
	Seq      uint8
	Checksum uint16
	Payload  []byte
}

func EncodeRDT(p RDTPacket) []byte {
	b := make([]byte, RDTHeaderLen+len(p.Payload))
	b[0] = byte(p.Kind)
	b[1] = p.Seq
	copy(b[RDTHeaderLen:], p.Payload)
	binary.BigEndian.PutUint16(b[2:4], Checksum(b))
	return b
}

func DecodeRDT(b []byte) (RDTPacket, bool) {
	if len(b) < RDTHeaderLen {
		return RDTPacket{}, false
	}
	p := RDTPacket{
		Kind:     RDTKind(b[0]),
		Seq:      b[1],
		Checksum: binary.BigEndian.Uint16(b[2:4]),
		Payload:  slices.Clone(b[RDTHeaderLen:]),
	}
	intact := Verify(b) && p.Kind >= RDTData && p.Kind <= RDTNak
	return p, intact
}
