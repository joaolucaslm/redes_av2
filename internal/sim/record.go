package sim

type Kind string

const (
	KindTransition Kind = "transition"
	KindRejection  Kind = "rejection"
	KindSend       Kind = "send"
	KindArrival    Kind = "arrival"
	KindLoss       Kind = "loss"
	KindDiscard    Kind = "discard"
	KindTimerArm   Kind = "timer-arm"
	KindTimerStop  Kind = "timer-stop"
	KindTimeout    Kind = "timeout"
	KindDelivery   Kind = "delivery"
	KindApp        Kind = "app"
	KindOperator   Kind = "operator"
	KindMetric     Kind = "metric"
)

type Fate string

const (
	FateDelivered Fate = "delivered"
	FateLost      Fate = "lost"
)

type Header struct {
	Proto          string `json:"proto"`
	Label          string `json:"label"`
	Flags          string `json:"flags,omitzero"`
	SrcPort        uint16 `json:"srcPort,omitzero"`
	DstPort        uint16 `json:"dstPort,omitzero"`
	Seq            uint32 `json:"seq,omitzero"`
	Ack            uint32 `json:"ack,omitzero"`
	RelSeq         uint32 `json:"relSeq,omitzero"`
	RelAck         uint32 `json:"relAck,omitzero"`
	Length         int    `json:"length,omitzero"`
	Payload        int    `json:"payload,omitzero"`
	Window         uint16 `json:"window,omitzero"`
	Checksum       uint16 `json:"checksum,omitzero"`
	Message        int    `json:"message,omitzero"`
	Retransmission bool   `json:"retransmission,omitzero"`
	Reason         string `json:"reason,omitzero"`
}

type Flight struct {
	ID        int         `json:"id"`
	Src       string      `json:"src"`
	Dst       string      `json:"dst"`
	SentAt    VirtualTime `json:"sentAt"`
	EndAt     VirtualTime `json:"endAt"`
	Fate      Fate        `json:"fate"`
	Cause     string      `json:"cause,omitzero"`
	Corrupted bool        `json:"corrupted,omitzero"`
	Flipped   []int       `json:"flipped,omitzero"`
	Duplicate bool        `json:"duplicate,omitzero"`
	Reordered bool        `json:"reordered,omitzero"`
	Header    Header      `json:"header"`
}

type Record struct {
	Seq      int         `json:"seq"`
	Step     int         `json:"step"`
	At       VirtualTime `json:"at"`
	Kind     Kind        `json:"kind"`
	Host     string      `json:"host,omitzero"`
	Protocol string      `json:"protocol,omitzero"`
	Machine  string      `json:"machine,omitzero"`
	From     string      `json:"from,omitzero"`
	Event    string      `json:"event,omitzero"`
	Guard    string      `json:"guard,omitzero"`
	Actions  []string    `json:"actions,omitzero"`
	To       string      `json:"to,omitzero"`
	Rule     int         `json:"rule"`
	Edge     int         `json:"edge"`
	Reason   string      `json:"reason,omitzero"`
	Timer    string      `json:"timer,omitzero"`
	Deadline VirtualTime `json:"deadline,omitzero"`
	Flight   Flight      `json:"flight,omitzero"`
	Message  int         `json:"message,omitzero"`
	Verdict  string      `json:"verdict,omitzero"`
	Metric   string      `json:"metric,omitzero"`
	Value    float64     `json:"value,omitzero"`
	Detail   string      `json:"detail,omitzero"`
}

func (r Record) ChangesState() bool {
	return r.Kind == KindTransition && r.From != r.To
}
