package workload

import (
	"fmt"
	"strconv"
)

const HeaderLen = 13

type Verdict string

const (
	VerdictFresh     Verdict = "nova"
	VerdictLate      Verdict = "fora de ordem"
	VerdictDuplicate Verdict = "duplicada"
)

func Payload(id, size int) []byte {
	size = max(size, HeaderLen+1)
	b := make([]byte, size)
	copy(b, fmt.Sprintf("M%05d:%05d|", id%100000, size%100000))
	for i := HeaderLen; i < size; i++ {
		b[i] = 'a' + byte((id+i)%26)
	}
	return b
}

func Parse(p []byte) (id, size int, ok bool) {
	if len(p) < HeaderLen || p[0] != 'M' || p[6] != ':' || p[12] != '|' {
		return 0, 0, false
	}
	id, errID := strconv.Atoi(string(p[1:6]))
	size, errSize := strconv.Atoi(string(p[7:12]))
	if errID != nil || errSize != nil || size < HeaderLen {
		return 0, 0, false
	}
	return id, size, true
}

type Source struct {
	next int
}

func (s *Source) Next(size int) (int, []byte) {
	s.next++
	return s.next, Payload(s.next, size)
}

func (s *Source) Offered() int { return s.next }

type Tally struct {
	Offered    int `json:"offered"`
	Delivered  int `json:"delivered"`
	InOrder    int `json:"inOrder"`
	OutOfOrder int `json:"outOfOrder"`
	Duplicates int `json:"duplicates"`
	Last       int `json:"last"`
}

type Sink struct {
	seen    map[int]bool
	highest int
	tally   Tally
}

func NewSink() *Sink {
	return &Sink{seen: make(map[int]bool)}
}

func (s *Sink) Accept(id int) Verdict {
	s.tally.Last = id
	if s.seen[id] {
		s.tally.Duplicates++
		return VerdictDuplicate
	}
	s.seen[id] = true
	s.tally.Delivered++
	if id < s.highest {
		s.tally.OutOfOrder++
		return VerdictLate
	}
	s.highest = id
	s.tally.InOrder++
	return VerdictFresh
}

func (s *Sink) Tally() Tally { return s.tally }

type Reassembler struct {
	buffer []byte
}

func (r *Reassembler) Feed(chunk []byte) []int {
	r.buffer = append(r.buffer, chunk...)
	var completed []int
	for len(r.buffer) >= HeaderLen {
		id, size, ok := Parse(r.buffer)
		if !ok {
			r.buffer = nil
			break
		}
		if len(r.buffer) < size {
			break
		}
		completed = append(completed, id)
		r.buffer = r.buffer[size:]
	}
	return completed
}

func (r *Reassembler) Buffered() int { return len(r.buffer) }
