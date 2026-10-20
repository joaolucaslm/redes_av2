package tcp

type Seq uint32

func (s Seq) Add(n int) Seq { return s + Seq(uint32(n)) }

func (s Seq) Before(o Seq) bool { return int32(uint32(s)-uint32(o)) < 0 }

func (s Seq) AtOrBefore(o Seq) bool { return !o.Before(s) }

func (s Seq) Minus(o Seq) int { return int(int32(uint32(s) - uint32(o))) }
