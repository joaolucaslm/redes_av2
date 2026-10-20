package sim

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"
)

type VirtualTime int64

const (
	Microsecond VirtualTime = 1
	Millisecond             = 1000 * Microsecond
	Second                  = 1000 * Millisecond
)

var ErrBadDuration = errors.New("duração virtual inválida")

func (t VirtualTime) Milliseconds() float64 {
	return float64(t) / float64(Millisecond)
}

func (t VirtualTime) String() string {
	if t >= 10*Second || t <= -10*Second {
		return strconv.FormatFloat(float64(t)/float64(Second), 'f', 3, 64) + "s"
	}
	return strconv.FormatFloat(t.Milliseconds(), 'f', -1, 64) + "ms"
}

func ParseVirtualTime(text string) (VirtualTime, error) {
	d, err := time.ParseDuration(text)
	if err != nil {
		return 0, fmt.Errorf("%w: %q", ErrBadDuration, text)
	}
	return VirtualTime(d / time.Microsecond), nil
}

func (t *VirtualTime) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return fmt.Errorf("%w: %s", ErrBadDuration, data)
		}
		parsed, err := ParseVirtualTime(text)
		if err != nil {
			return err
		}
		*t = parsed
		return nil
	}
	var micros int64
	if err := json.Unmarshal(data, &micros); err != nil {
		return fmt.Errorf("%w: %s", ErrBadDuration, data)
	}
	*t = VirtualTime(micros)
	return nil
}
