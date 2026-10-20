package wire

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"slices"
	"strings"
)

type Flags uint8

const (
	FlagFIN Flags = 1 << iota
	FlagSYN
	FlagRST
	FlagPSH
	FlagACK
	FlagURG
	FlagECE
	FlagCWR
)

var flagNames = []struct {
	flag Flags
	name string
}{
	{FlagSYN, "SYN"},
	{FlagRST, "RST"},
	{FlagFIN, "FIN"},
	{FlagPSH, "PSH"},
	{FlagACK, "ACK"},
	{FlagURG, "URG"},
	{FlagECE, "ECE"},
	{FlagCWR, "CWR"},
}

func (f Flags) Has(x Flags) bool { return f&x == x }

func (f Flags) String() string {
	var names []string
	for _, fn := range flagNames {
		if f.Has(fn.flag) {
			names = append(names, fn.name)
		}
	}
	return strings.Join(names, "|")
}

func ParseFlags(text string) (Flags, error) {
	var flags Flags
	for _, part := range strings.Split(text, "|") {
		part = strings.TrimSpace(strings.ToUpper(part))
		if part == "" {
			continue
		}
		found := false
		for _, fn := range flagNames {
			if fn.name == part {
				flags |= fn.flag
				found = true
			}
		}
		if !found {
			return 0, fmt.Errorf("%w: flag %q", ErrMalformedHeader, part)
		}
	}
	return flags, nil
}

const TCPHeaderLen = 20

type Segment struct {
	SrcPort  uint16
	DstPort  uint16
	Seq      uint32
	Ack      uint32
	Flags    Flags
	Window   uint16
	Checksum uint16
	Payload  []byte
}

func (s Segment) SeqLen() int {
	n := len(s.Payload)
	if s.Flags.Has(FlagSYN) {
		n++
	}
	if s.Flags.Has(FlagFIN) {
		n++
	}
	return n
}

func EncodeTCP(src, dst netip.Addr, s Segment) []byte {
	b := make([]byte, TCPHeaderLen+len(s.Payload))
	binary.BigEndian.PutUint16(b[0:2], s.SrcPort)
	binary.BigEndian.PutUint16(b[2:4], s.DstPort)
	binary.BigEndian.PutUint32(b[4:8], s.Seq)
	binary.BigEndian.PutUint32(b[8:12], s.Ack)
	b[12] = (TCPHeaderLen / 4) << 4
	b[13] = byte(s.Flags)
	binary.BigEndian.PutUint16(b[14:16], s.Window)
	copy(b[TCPHeaderLen:], s.Payload)
	binary.BigEndian.PutUint16(b[16:18], Checksum(PseudoHeader(src, dst, ProtoTCP, len(b)), b))
	return b
}

func DecodeTCP(src, dst netip.Addr, b []byte) (Segment, error) {
	if len(b) < TCPHeaderLen {
		return Segment{}, fmt.Errorf("tcp: %w", ErrTruncated)
	}
	if !Verify(PseudoHeader(src, dst, ProtoTCP, len(b)), b) {
		return Segment{}, fmt.Errorf("tcp: %w", ErrBadChecksum)
	}
	offset := int(b[12]>>4) * 4
	if offset < TCPHeaderLen || offset > len(b) {
		return Segment{}, fmt.Errorf("tcp: %w (data offset %d)", ErrMalformedHeader, offset)
	}
	return Segment{
		SrcPort:  binary.BigEndian.Uint16(b[0:2]),
		DstPort:  binary.BigEndian.Uint16(b[2:4]),
		Seq:      binary.BigEndian.Uint32(b[4:8]),
		Ack:      binary.BigEndian.Uint32(b[8:12]),
		Flags:    Flags(b[13]),
		Window:   binary.BigEndian.Uint16(b[14:16]),
		Checksum: binary.BigEndian.Uint16(b[16:18]),
		Payload:  slices.Clone(b[offset:]),
	}, nil
}
