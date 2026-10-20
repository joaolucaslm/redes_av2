package wire

import (
	"errors"
	"net/netip"
	"slices"
)

var (
	ErrTruncated       = errors.New("quadro truncado")
	ErrLengthMismatch  = errors.New("campo de comprimento inconsistente com o quadro")
	ErrBadChecksum     = errors.New("checksum inválido")
	ErrMissingChecksum = errors.New("checksum ausente")
	ErrMalformedHeader = errors.New("cabeçalho malformado")
)

type Frame struct {
	Src   netip.Addr
	Dst   netip.Addr
	Proto Proto
	Bytes []byte
}

func (f Frame) Clone() Frame {
	f.Bytes = slices.Clone(f.Bytes)
	return f
}

func (f Frame) FlipBit(position int) Frame {
	flipped := f.Clone()
	if len(flipped.Bytes) == 0 {
		return flipped
	}
	position %= len(flipped.Bytes) * 8
	flipped.Bytes[position/8] ^= 0x80 >> (position % 8)
	return flipped
}
