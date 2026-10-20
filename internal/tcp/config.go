package tcp

import (
	"errors"
	"fmt"

	"bancada/internal/sim"
)

var ErrInvalidConfig = errors.New("configuração TCP inválida")

type Config struct {
	MSS               int             `json:"mss"`
	InitialCwnd       int             `json:"initialCwnd"`
	InitialSsthresh   int             `json:"initialSsthresh"`
	ReceiveBuffer     int             `json:"receiveBuffer"`
	MSL               sim.VirtualTime `json:"msl"`
	InitialRTO        sim.VirtualTime `json:"initialRto"`
	MinRTO            sim.VirtualTime `json:"minRto"`
	MaxRTO            sim.VirtualTime `json:"maxRto"`
	Alpha             float64         `json:"alpha"`
	Beta              float64         `json:"beta"`
	DupAckThreshold   int             `json:"dupAckThreshold"`
	FastRetransmit    bool            `json:"fastRetransmit"`
	CongestionControl bool            `json:"congestionControl"`
	ZeroISN           bool            `json:"zeroIsn"`
	ServerPort        uint16          `json:"serverPort"`
	ClientPort        uint16          `json:"clientPort"`
}

func DefaultConfig() Config {
	return Config{
		MSS:               1000,
		InitialCwnd:       1,
		InitialSsthresh:   64000,
		ReceiveBuffer:     65535,
		MSL:               1 * sim.Second,
		InitialRTO:        1 * sim.Second,
		MinRTO:            200 * sim.Millisecond,
		MaxRTO:            60 * sim.Second,
		Alpha:             0.125,
		Beta:              0.25,
		DupAckThreshold:   3,
		FastRetransmit:    true,
		CongestionControl: true,
		ServerPort:        80,
		ClientPort:        49152,
	}
}

func (c Config) Validate() error {
	var problems []error
	check := func(ok bool, format string, args ...any) {
		if !ok {
			problems = append(problems, fmt.Errorf("%w: "+format, append([]any{ErrInvalidConfig}, args...)...))
		}
	}
	check(c.MSS > 0 && c.MSS <= 65535, "mss=%d", c.MSS)
	check(c.InitialCwnd >= 1, "initialCwnd=%d", c.InitialCwnd)
	check(c.InitialSsthresh >= c.MSS, "initialSsthresh=%d < mss", c.InitialSsthresh)
	check(c.ReceiveBuffer >= c.MSS && c.ReceiveBuffer <= 65535, "receiveBuffer=%d", c.ReceiveBuffer)
	check(c.MSL > 0, "msl=%v", c.MSL)
	check(c.InitialRTO > 0 && c.MinRTO > 0 && c.MinRTO <= c.MaxRTO, "rto inicial/mín/máx = %v/%v/%v", c.InitialRTO, c.MinRTO, c.MaxRTO)
	check(c.Alpha > 0 && c.Alpha < 1 && c.Beta > 0 && c.Beta < 1, "alpha=%v beta=%v", c.Alpha, c.Beta)
	check(c.DupAckThreshold >= 1, "dupAckThreshold=%d", c.DupAckThreshold)
	check(c.ServerPort != 0 && c.ClientPort != 0, "portas não podem ser zero")
	return errors.Join(problems...)
}
