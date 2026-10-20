package channel

import (
	"errors"
	"fmt"
	"math"

	"bancada/internal/sim"
)

var (
	ErrInvalidParams = errors.New("parâmetros de canal inválidos")
	ErrInvalidRule   = errors.New("regra de falha inválida")
)

const MaxCorruptBits = 8

type Params struct {
	Loss        float64         `json:"loss"`
	DelayMin    sim.VirtualTime `json:"delayMin"`
	DelayMax    sim.VirtualTime `json:"delayMax"`
	Corrupt     float64         `json:"corrupt"`
	CorruptBits int             `json:"corruptBits"`
	Duplicate   float64         `json:"duplicate"`
	Reorder     float64         `json:"reorder"`
	ReorderLag  sim.VirtualTime `json:"reorderLag"`
}

func DefaultParams() Params {
	return Params{
		DelayMin:    40 * sim.Millisecond,
		DelayMax:    60 * sim.Millisecond,
		CorruptBits: 1,
		ReorderLag:  80 * sim.Millisecond,
	}
}

func (p Params) Validate() error {
	var problems []error
	probability := func(name string, v float64) {
		if math.IsNaN(v) || v < 0 || v > 1 {
			problems = append(problems, fmt.Errorf("%w: %s=%v fora de [0,1]", ErrInvalidParams, name, v))
		}
	}
	probability("loss", p.Loss)
	probability("corrupt", p.Corrupt)
	probability("duplicate", p.Duplicate)
	probability("reorder", p.Reorder)
	if p.DelayMin < 0 || p.DelayMax < 0 || p.ReorderLag < 0 {
		problems = append(problems, fmt.Errorf("%w: atrasos não podem ser negativos", ErrInvalidParams))
	}
	if p.DelayMin > p.DelayMax {
		problems = append(problems, fmt.Errorf("%w: delayMin (%v) > delayMax (%v)", ErrInvalidParams, p.DelayMin, p.DelayMax))
	}
	if p.CorruptBits < 1 || p.CorruptBits > MaxCorruptBits {
		problems = append(problems, fmt.Errorf("%w: corruptBits=%d fora de [1,%d]", ErrInvalidParams, p.CorruptBits, MaxCorruptBits))
	}
	return errors.Join(problems...)
}
