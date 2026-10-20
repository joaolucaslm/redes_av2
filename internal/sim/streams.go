package sim

import (
	"hash/fnv"
	"math/rand/v2"
)

type Streams struct {
	seed uint64
}

func NewStreams(seed uint64) Streams {
	return Streams{seed: seed}
}

func (s Streams) Seed() uint64 { return s.seed }

func (s Streams) Stream(name string) *rand.Rand {
	digest := fnv.New64a()
	digest.Write([]byte(name))
	return rand.New(rand.NewPCG(s.seed, digest.Sum64()))
}
