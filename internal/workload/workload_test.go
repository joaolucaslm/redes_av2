package workload

import (
	"slices"
	"testing"
)

func TestPayloadRoundTrip(t *testing.T) {
	for _, tc := range []struct{ id, size int }{{1, 14}, {42, 500}, {99999, 1000}, {7, 3}} {
		id, size, ok := Parse(Payload(tc.id, tc.size))
		if !ok || id != tc.id || size != max(tc.size, HeaderLen+1) {
			t.Errorf("Parse(Payload(%d, %d)) = %d, %d, %v", tc.id, tc.size, id, size, ok)
		}
	}
	if _, _, ok := Parse([]byte("lixo qualquer")); ok {
		t.Error("cabeçalho inválido aceito")
	}
}

func TestReassemblerAcrossArbitraryChunks(t *testing.T) {
	var stream []byte
	for id := 1; id <= 5; id++ {
		stream = append(stream, Payload(id, 37+id)...)
	}
	for _, chunk := range []int{1, 7, 13, 64, len(stream)} {
		var r Reassembler
		var got []int
		for start := 0; start < len(stream); start += chunk {
			got = append(got, r.Feed(stream[start:min(start+chunk, len(stream))])...)
		}
		if !slices.Equal(got, []int{1, 2, 3, 4, 5}) || r.Buffered() != 0 {
			t.Errorf("blocos de %d: mensagens %v, sobra %d", chunk, got, r.Buffered())
		}
	}
}

func TestSinkVerdicts(t *testing.T) {
	sink := NewSink()
	var verdicts []Verdict
	for _, id := range []int{1, 3, 2, 3, 4} {
		verdicts = append(verdicts, sink.Accept(id))
	}
	want := []Verdict{VerdictFresh, VerdictFresh, VerdictLate, VerdictDuplicate, VerdictFresh}
	if !slices.Equal(verdicts, want) {
		t.Fatalf("vereditos %v, quero %v", verdicts, want)
	}
	tally := sink.Tally()
	if tally.Delivered != 4 || tally.Duplicates != 1 || tally.OutOfOrder != 1 || tally.InOrder != 3 {
		t.Fatalf("contagem errada: %+v", tally)
	}
}
