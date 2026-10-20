package wire

import (
	"encoding/binary"
	"net/netip"
)

type Proto uint8

const (
	ProtoTCP Proto = 6
	ProtoUDP Proto = 17
	ProtoRDT Proto = 253
)

func (p Proto) String() string {
	switch p {
	case ProtoTCP:
		return "tcp"
	case ProtoUDP:
		return "udp"
	case ProtoRDT:
		return "rdt"
	default:
		return "desconhecido"
	}
}

type Accumulator struct {
	sum     uint64
	pending bool
	high    byte
}

func (a *Accumulator) Add(chunk []byte) {
	for _, b := range chunk {
		if a.pending {
			a.sum += uint64(a.high)<<8 | uint64(b)
			a.pending = false
			continue
		}
		a.high = b
		a.pending = true
	}
}

func (a *Accumulator) Sum16() uint16 {
	sum := a.sum
	if a.pending {
		sum += uint64(a.high) << 8
	}
	for sum>>16 != 0 {
		sum = sum&0xffff + sum>>16
	}
	return uint16(sum)
}

func OnesComplementSum(chunks ...[]byte) uint16 {
	var acc Accumulator
	for _, chunk := range chunks {
		acc.Add(chunk)
	}
	return acc.Sum16()
}

func Checksum(chunks ...[]byte) uint16 {
	return ^OnesComplementSum(chunks...)
}

func Verify(chunks ...[]byte) bool {
	return OnesComplementSum(chunks...) == 0xffff
}

func PseudoHeader(src, dst netip.Addr, proto Proto, length int) []byte {
	header := make([]byte, 12)
	s, d := src.As4(), dst.As4()
	copy(header[0:4], s[:])
	copy(header[4:8], d[:])
	header[9] = byte(proto)
	binary.BigEndian.PutUint16(header[10:12], uint16(length))
	return header
}
