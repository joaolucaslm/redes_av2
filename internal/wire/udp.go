package wire

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"slices"
)

const UDPHeaderLen = 8

type Datagram struct {
	SrcPort  uint16
	DstPort  uint16
	Length   uint16
	Checksum uint16
	Payload  []byte
}

func EncodeUDP(src, dst netip.Addr, srcPort, dstPort uint16, payload []byte) []byte {
	length := UDPHeaderLen + len(payload)
	b := make([]byte, length)
	binary.BigEndian.PutUint16(b[0:2], srcPort)
	binary.BigEndian.PutUint16(b[2:4], dstPort)
	binary.BigEndian.PutUint16(b[4:6], uint16(length))
	copy(b[UDPHeaderLen:], payload)
	sum := Checksum(PseudoHeader(src, dst, ProtoUDP, length), b)
	if sum == 0 {
		sum = 0xffff
	}
	binary.BigEndian.PutUint16(b[6:8], sum)
	return b
}

func DecodeUDP(src, dst netip.Addr, b []byte) (Datagram, error) {
	if len(b) < UDPHeaderLen {
		return Datagram{}, fmt.Errorf("udp: %w", ErrTruncated)
	}
	d := Datagram{
		SrcPort:  binary.BigEndian.Uint16(b[0:2]),
		DstPort:  binary.BigEndian.Uint16(b[2:4]),
		Length:   binary.BigEndian.Uint16(b[4:6]),
		Checksum: binary.BigEndian.Uint16(b[6:8]),
	}
	if int(d.Length) != len(b) {
		return d, fmt.Errorf("udp: %w (campo=%d, quadro=%d)", ErrLengthMismatch, d.Length, len(b))
	}
	if d.Checksum == 0 {
		return d, fmt.Errorf("udp: %w", ErrMissingChecksum)
	}
	if !Verify(PseudoHeader(src, dst, ProtoUDP, len(b)), b) {
		return d, fmt.Errorf("udp: %w", ErrBadChecksum)
	}
	d.Payload = slices.Clone(b[UDPHeaderLen:])
	return d, nil
}
