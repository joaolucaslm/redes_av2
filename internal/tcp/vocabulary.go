package tcp

import (
	"bancada/internal/fsm"
	"bancada/internal/host"
	"bancada/internal/wire"
)

const (
	EventConnect     fsm.Event = "connect"
	EventListen      fsm.Event = "listen"
	EventClose       fsm.Event = "close"
	EventSend        fsm.Event = "send"
	EventSYN         fsm.Event = "rcv.SYN"
	EventSYNACK      fsm.Event = "rcv.SYNACK"
	EventACK         fsm.Event = "rcv.ACK"
	EventData        fsm.Event = "rcv.data"
	EventFIN         fsm.Event = "rcv.FIN"
	EventRST         fsm.Event = "rcv.RST"
	EventUnknown     fsm.Event = "rcv.?"
	EventTimeoutRTO  fsm.Event = "timeout.rto"
	EventTimeout2MSL fsm.Event = "timeout.2msl"
	EventAckNew      fsm.Event = "ack.new"
	EventAckDup      fsm.Event = "ack.dup"
	EventTimeout     fsm.Event = "timeout"
)

var segmentEvents = []struct {
	flags   wire.Flags
	payload bool
	event   fsm.Event
}{
	{flags: wire.FlagRST, event: EventRST},
	{flags: wire.FlagSYN | wire.FlagACK, event: EventSYNACK},
	{flags: wire.FlagSYN, event: EventSYN},
	{flags: wire.FlagFIN, event: EventFIN},
	{payload: true, event: EventData},
	{flags: wire.FlagACK, event: EventACK},
}

func Classify(seg wire.Segment) fsm.Event {
	for _, rule := range segmentEvents {
		if seg.Flags.Has(rule.flags) && (!rule.payload || len(seg.Payload) > 0) {
			return rule.event
		}
	}
	return EventUnknown
}

var controlFlags = map[string]wire.Flags{
	"SYN":     wire.FlagSYN,
	"SYN|ACK": wire.FlagSYN | wire.FlagACK,
	"ACK":     wire.FlagACK,
	"FIN":     wire.FlagFIN,
	"RST":     wire.FlagRST,
}

func Vocabulary() *fsm.Registry[*Endpoint] {
	return fsm.NewRegistry[*Endpoint]().
		Guard("ack_ok", func(ep *Endpoint) bool {
			return ep.in.Flags.Has(wire.FlagACK) && Seq(ep.in.Ack) == ep.sndNxt
		}).
		Guard("fin_acked", func(ep *Endpoint) bool {
			return ep.finSent && ep.in.Flags.Has(wire.FlagACK) && ep.finSeq.Add(1).AtOrBefore(Seq(ep.in.Ack))
		}).
		Guard("in_order", func(ep *Endpoint) bool { return Seq(ep.in.Seq) == ep.rcvNxt }).
		Guard("passive", func(ep *Endpoint) bool { return ep.passive }).
		Guard("third_dup_ack", func(ep *Endpoint) bool { return ep.dupAcks == ep.cfg().DupAckThreshold-1 }).
		Guard("fast_retransmit", func(ep *Endpoint) bool { return ep.cfg().FastRetransmit }).
		Guard("reaches_ssthresh", func(ep *Endpoint) bool { return ep.cwnd+ep.cfg().MSS >= ep.ssthresh }).
		Effect("passive.open", func(ep *Endpoint) { ep.passive = true }).
		Effect("isn.choose", (*Endpoint).chooseISN).
		Effect("syn.accept", (*Endpoint).acceptSyn).
		Effect("ack.ingest", (*Endpoint).ingestAck).
		Effect("data.enqueue", func(ep *Endpoint) {
			ep.unsent = append(ep.unsent, ep.write...)
			ep.write = nil
		}).
		Effect("data.accept", (*Endpoint).acceptData).
		Effect("fin.accept", (*Endpoint).acceptFin).
		Effect("tcb.release", (*Endpoint).release).
		Action("send", func(ep *Endpoint, arg string) error {
			ep.sendControl(controlFlags[arg])
			return nil
		}, "SYN", "SYN|ACK", "ACK", "FIN", "RST").
		Action("retransmit", func(ep *Endpoint, reason string) error {
			ep.retransmit(reason)
			return nil
		}, "timeout", "fast", "duplicate").
		Action("transmit", func(ep *Endpoint, _ string) error {
			ep.transmit()
			return nil
		}, "new").
		Action("timer.start", func(ep *Endpoint, _ string) error {
			ep.startTimeWait()
			return nil
		}, "2msl").
		Action("timer.restart", func(ep *Endpoint, _ string) error {
			ep.startTimeWait()
			return nil
		}, "2msl").
		Action("cc", func(ep *Endpoint, event string) error {
			ep.signal(fsm.Event(event))
			return nil
		}, "timeout").
		Action("app.notify", func(ep *Endpoint, event string) error {
			host.Notify(ep.env(), event, "")
			return nil
		}, "connected", "refused", "reset", "peer-closed", "closed").
		Effect("dupacks.inc", func(ep *Endpoint) { ep.dupAcks++ }).
		Effect("dupacks.reset", func(ep *Endpoint) { ep.dupAcks = 0 }).
		Action("cwnd.add", func(ep *Endpoint, amount string) error {
			mss := ep.cfg().MSS
			switch amount {
			case "MSS":
				ep.cwnd += mss
			default:
				ep.cwnd += max(1, mss*mss/max(ep.cwnd, 1))
			}
			return nil
		}, "MSS", "MSS*MSS/cwnd").
		Action("cwnd.set", func(ep *Endpoint, value string) error {
			mss := ep.cfg().MSS
			switch value {
			case "1MSS":
				ep.cwnd = mss
			case "ssthresh":
				ep.cwnd = ep.ssthresh
			default:
				ep.cwnd = ep.ssthresh + 3*mss
			}
			return nil
		}, "1MSS", "ssthresh", "ssthresh+3MSS").
		Action("ssthresh.set", func(ep *Endpoint, _ string) error {
			ep.ssthresh = max(ep.cwnd/2, 2*ep.cfg().MSS)
			return nil
		}, "cwnd/2")
}
