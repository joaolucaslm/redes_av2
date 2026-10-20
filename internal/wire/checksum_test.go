package wire

import (
	"bytes"
	"errors"
	"net/netip"
	"testing"

	"pgregory.net/rapid"
)

var (
	hostA = netip.MustParseAddr("10.0.0.1")
	hostB = netip.MustParseAddr("10.0.0.2")
)

func TestOnesComplementKnownAnswers(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		sum   uint16
		check uint16
	}{
		{"exemplo da seção 3.3", []byte{0b01100110, 0b01100000, 0b01010101, 0b01010101, 0b10001111, 0b00001100}, 0b0100101011000010, 0b1011010100111101},
		{"exemplo da RFC 1071", []byte{0x00, 0x01, 0xf2, 0x03, 0xf4, 0xf5, 0xf6, 0xf7}, 0xddf2, 0x220d},
		{"comprimento ímpar completa com zero", []byte{0x12, 0x34, 0x56}, 0x6834, 0x97cb},
		{"vazio", nil, 0x0000, 0xffff},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := OnesComplementSum(tc.input); got != tc.sum {
				t.Errorf("soma = %016b, quero %016b", got, tc.sum)
			}
			if got := Checksum(tc.input); got != tc.check {
				t.Errorf("checksum = %016b, quero %016b", got, tc.check)
			}
		})
	}
}

func TestAccumulatorSplitsAcrossOddChunks(t *testing.T) {
	data := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9}
	if OnesComplementSum(data[:3], data[3:4], data[4:]) != OnesComplementSum(data) {
		t.Fatal("a soma deve ser independente da fragmentação em blocos")
	}
}

func TestUDPRoundTripAndDiscardReasons(t *testing.T) {
	frame := EncodeUDP(hostA, hostB, 40000, 9000, []byte("olá, receptor"))
	d, err := DecodeUDP(hostA, hostB, frame)
	if err != nil || string(d.Payload) != "olá, receptor" || d.Length != uint16(len(frame)) {
		t.Fatalf("ida e volta falhou: %+v %v", d, err)
	}
	tests := []struct {
		name   string
		mutate func([]byte) []byte
		want   error
	}{
		{"truncado", func(b []byte) []byte { return b[:5] }, ErrTruncated},
		{"comprimento inconsistente", func(b []byte) []byte { return append(b, 0) }, ErrLengthMismatch},
		{"checksum zerado", func(b []byte) []byte { b[6], b[7] = 0, 0; return b }, ErrMissingChecksum},
		{"bit trocado no payload", func(b []byte) []byte { b[10] ^= 0x04; return b }, ErrBadChecksum},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecodeUDP(hostA, hostB, tc.mutate(bytes.Clone(frame)))
			if !errors.Is(err, tc.want) {
				t.Fatalf("erro = %v, quero %v", err, tc.want)
			}
		})
	}
	if _, err := DecodeUDP(hostA, netip.MustParseAddr("10.0.0.3"), frame); !errors.Is(err, ErrBadChecksum) {
		t.Fatalf("o pseudo-cabeçalho deveria cobrir o IP de destino: %v", err)
	}
}

func TestTCPRoundTrip(t *testing.T) {
	seg := Segment{SrcPort: 49152, DstPort: 80, Seq: 0xfffffff0, Ack: 7, Flags: FlagSYN | FlagACK, Window: 65535, Payload: []byte("x")}
	frame := EncodeTCP(hostA, hostB, seg)
	got, err := DecodeTCP(hostA, hostB, frame)
	if err != nil {
		t.Fatal(err)
	}
	if got.Seq != seg.Seq || got.Ack != seg.Ack || got.Flags != seg.Flags || got.Window != seg.Window || string(got.Payload) != "x" {
		t.Fatalf("segmento divergente: %+v", got)
	}
	if got.Flags.String() != "SYN|ACK" || got.SeqLen() != 2 {
		t.Fatalf("flags=%s seqlen=%d", got.Flags, got.SeqLen())
	}
	if flags, err := ParseFlags("rst|ack"); err != nil || flags != FlagRST|FlagACK {
		t.Fatalf("ParseFlags = %v, %v", flags, err)
	}
}

func TestEverySingleBitFlipIsDetected(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		payload := rapid.SliceOfN(rapid.Byte(), 0, 64).Draw(t, "payload")
		srcPort := rapid.Uint16().Draw(t, "srcPort")
		dstPort := rapid.Uint16().Draw(t, "dstPort")
		proto := rapid.SampledFrom([]Proto{ProtoUDP, ProtoTCP, ProtoRDT}).Draw(t, "proto")

		var original []byte
		var verify func([]byte) bool
		switch proto {
		case ProtoUDP:
			original = EncodeUDP(hostA, hostB, srcPort, dstPort, payload)
			verify = func(b []byte) bool { _, err := DecodeUDP(hostA, hostB, b); return err == nil }
		case ProtoTCP:
			seq := rapid.Uint32().Draw(t, "seq")
			original = EncodeTCP(hostA, hostB, Segment{SrcPort: srcPort, DstPort: dstPort, Seq: seq, Flags: FlagACK, Payload: payload})
			verify = func(b []byte) bool { _, err := DecodeTCP(hostA, hostB, b); return err == nil }
		default:
			original = EncodeRDT(RDTPacket{Kind: RDTData, Seq: uint8(srcPort % 2), Payload: payload})
			verify = func(b []byte) bool { _, ok := DecodeRDT(b); return ok }
		}
		if !verify(original) {
			t.Fatalf("quadro íntegro rejeitado")
		}
		bit := rapid.IntRange(0, len(original)*8-1).Draw(t, "bit")
		frame := Frame{Bytes: original}.FlipBit(bit)
		if verify(frame.Bytes) {
			t.Fatalf("troca do bit %d não detectada em %s", bit, proto)
		}
	})
}

func TestTwoCompensatingFlipsEscapeTheChecksum(t *testing.T) {
	original := EncodeUDP(hostA, hostB, 1, 2, []byte{0x00, 0x00, 0x00, 0x01})
	corrupted := Frame{Bytes: original}.FlipBit(8*UDPHeaderLen + 15).FlipBit(8*UDPHeaderLen + 31)
	d, err := DecodeUDP(hostA, hostB, corrupted.Bytes)
	if err != nil {
		t.Fatalf("esperava que o checksum não detectasse a dupla troca: %v", err)
	}
	if bytes.Equal(d.Payload, original[UDPHeaderLen:]) {
		t.Fatal("o payload deveria ter mudado")
	}
}

func FuzzChecksum(f *testing.F) {
	f.Add([]byte("segmento"), uint16(40000), uint16(9000), uint(3))
	f.Add([]byte{}, uint16(0), uint16(0), uint(0))
	f.Add([]byte{0xff, 0xff, 0x00}, uint16(65535), uint16(1), uint(77))
	f.Fuzz(func(t *testing.T, payload []byte, srcPort, dstPort uint16, bit uint) {
		if len(payload) > 1<<15 {
			t.Skip()
		}
		frame := EncodeUDP(hostA, hostB, srcPort, dstPort, payload)
		d, err := DecodeUDP(hostA, hostB, frame)
		if err != nil || !bytes.Equal(d.Payload, payload) {
			t.Fatalf("ida e volta falhou: %v", err)
		}
		flipped := Frame{Bytes: frame}.FlipBit(int(bit % uint(len(frame)*8)))
		if _, err := DecodeUDP(hostA, hostB, flipped.Bytes); err == nil {
			t.Fatalf("troca de um bit não detectada")
		}
	})
}
