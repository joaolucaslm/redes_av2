package tcp

import (
	"encoding/binary"
	"fmt"
	"slices"

	"bancada/internal/fsm"
	"bancada/internal/host"
	"bancada/internal/sim"
	"bancada/internal/wire"
	"bancada/internal/workload"
)

type outstanding struct {
	seq           Seq
	flags         wire.Flags
	payload       []byte
	retransmitted bool
}

func (o outstanding) end() Seq {
	n := len(o.payload)
	if o.flags.Has(wire.FlagSYN) {
		n++
	}
	if o.flags.Has(wire.FlagFIN) {
		n++
	}
	return o.seq.Add(n)
}

type rttProbe struct {
	active bool
	end    Seq
	sentAt sim.VirtualTime
}

type Endpoint struct {
	stack     *Stack
	local     uint16
	remote    uint16
	passive   bool
	transient bool

	conn *fsm.Machine[*Endpoint]
	cc   *fsm.Machine[*Endpoint]

	in    wire.Segment
	write []byte

	iss        Seq
	irs        Seq
	synced     bool
	sndUna     Seq
	sndNxt     Seq
	finSeq     Seq
	finQueued  bool
	finSent    bool
	unsent     []byte
	inflight   []outstanding
	peerWindow int

	rcvNxt  Seq
	ooo     map[Seq][]byte
	finRcvd bool

	rtt   RTTEstimator
	probe rttProbe

	cwnd     int
	ssthresh int
	dupAcks  int

	rto      sim.TimerID
	timeWait sim.TimerID
	reasm    workload.Reassembler
}

func (s *Stack) newEndpoint(local, remote uint16) (*Endpoint, error) {
	ep := &Endpoint{
		stack:      s,
		local:      local,
		remote:     remote,
		ooo:        make(map[Seq][]byte),
		rtt:        NewRTTEstimator(s.cfg),
		cwnd:       s.cfg.InitialCwnd * s.cfg.MSS,
		ssthresh:   s.cfg.InitialSsthresh,
		peerWindow: s.cfg.MSS,
	}
	conn, err := s.programs.Connection.Start(ep, host.Observer(s.env, s.programs.Connection.Protocol()))
	if err != nil {
		return nil, err
	}
	cc, err := s.programs.Congestion.Start(ep, host.Observer(s.env, s.programs.Congestion.Protocol()))
	if err != nil {
		return nil, err
	}
	ep.conn, ep.cc = conn, cc
	return ep, nil
}

func (ep *Endpoint) env() host.Env { return ep.stack.env }

func (ep *Endpoint) cfg() Config { return ep.stack.cfg }

func (ep *Endpoint) dispatch(event fsm.Event) error {
	_, err := ep.conn.Fire(ep, event)
	if err != nil {
		host.Reject(ep.env(), ep.conn.Protocol(), err)
	}
	return err
}

func (ep *Endpoint) signal(event fsm.Event) {
	if _, err := ep.cc.Fire(ep, event); err != nil {
		host.Reject(ep.env(), ep.cc.Protocol(), err)
	}
}

func (ep *Endpoint) chooseISN() {
	ep.iss = 0
	if !ep.cfg().ZeroISN {
		ep.iss = Seq(ep.stack.isn.Uint32())
	}
	ep.sndUna, ep.sndNxt = ep.iss, ep.iss
}

func (ep *Endpoint) acceptSyn() {
	ep.irs = Seq(ep.in.Seq)
	ep.rcvNxt = ep.irs.Add(1)
	ep.synced = true
	ep.remote = ep.in.SrcPort
	ep.peerWindow = int(ep.in.Window)
}

func (ep *Endpoint) advertisedWindow() uint16 {
	buffered := 0
	for _, chunk := range ep.ooo {
		buffered += len(chunk)
	}
	return uint16(min(max(ep.cfg().ReceiveBuffer-buffered, 0), 65535))
}

func (ep *Endpoint) sendWindow() int {
	if ep.cfg().CongestionControl {
		return min(ep.peerWindow, ep.cwnd)
	}
	return ep.peerWindow
}

func (ep *Endpoint) inFlight() int { return ep.sndNxt.Minus(ep.sndUna) }

func (ep *Endpoint) emit(seg wire.Segment, retransmission bool, reason string) {
	env := ep.env()
	seg.SrcPort, seg.DstPort = ep.local, ep.remote
	seg.Window = ep.advertisedWindow()
	frame := wire.Frame{Src: env.Addr(), Dst: env.PeerAddr(), Proto: wire.ProtoTCP, Bytes: wire.EncodeTCP(env.Addr(), env.PeerAddr(), seg)}
	header := sim.Header{
		Proto:          "tcp",
		Flags:          seg.Flags.String(),
		SrcPort:        seg.SrcPort,
		DstPort:        seg.DstPort,
		Seq:            seg.Seq,
		Ack:            seg.Ack,
		RelSeq:         uint32(Seq(seg.Seq) - ep.iss),
		Length:         len(frame.Bytes),
		Payload:        len(seg.Payload),
		Window:         seg.Window,
		Checksum:       binary.BigEndian.Uint16(frame.Bytes[16:18]),
		Retransmission: retransmission,
		Reason:         reason,
	}
	if seg.Flags.Has(wire.FlagACK) && ep.synced {
		header.RelAck = uint32(Seq(seg.Ack) - ep.irs)
	}
	if id, _, ok := workload.Parse(seg.Payload); ok {
		header.Message = id
	}
	header.Label = segmentLabel(seg.Flags, header)
	env.Transmit(frame, header)
}

func segmentLabel(flags wire.Flags, h sim.Header) string {
	switch {
	case flags.Has(wire.FlagRST) && flags.Has(wire.FlagACK):
		return fmt.Sprintf("RST ACK=%d", h.RelAck)
	case flags.Has(wire.FlagRST):
		return fmt.Sprintf("RST seq=%d", h.RelSeq)
	case flags.Has(wire.FlagSYN | wire.FlagACK):
		return fmt.Sprintf("SYNACK seq=%d ACK=%d", h.RelSeq, h.RelAck)
	case flags.Has(wire.FlagSYN):
		return fmt.Sprintf("SYN seq=%d", h.RelSeq)
	case flags.Has(wire.FlagFIN):
		return fmt.Sprintf("FIN seq=%d ACK=%d", h.RelSeq, h.RelAck)
	case h.Payload > 0:
		return fmt.Sprintf("seq=%d len=%d", h.RelSeq, h.Payload)
	default:
		return fmt.Sprintf("ACK=%d", h.RelAck)
	}
}

func (ep *Endpoint) emitOutstanding(o outstanding, retransmission bool, reason string) {
	seg := wire.Segment{Seq: uint32(o.seq), Flags: o.flags, Payload: o.payload}
	if o.flags.Has(wire.FlagACK) {
		seg.Ack = uint32(ep.rcvNxt)
	}
	ep.emit(seg, retransmission, reason)
}

func (ep *Endpoint) sendNew(o outstanding) {
	ep.inflight = append(ep.inflight, o)
	ep.sndNxt = o.end()
	if !ep.probe.active {
		ep.probe = rttProbe{active: true, end: o.end(), sentAt: ep.env().Now()}
	}
	ep.emitOutstanding(o, false, "")
	ep.armRTOIfIdle()
}

func (ep *Endpoint) sendControl(flags wire.Flags) {
	switch {
	case flags == wire.FlagSYN:
		ep.sendNew(outstanding{seq: ep.iss, flags: wire.FlagSYN})
	case flags == wire.FlagSYN|wire.FlagACK && len(ep.inflight) > 0 && ep.inflight[0].flags.Has(wire.FlagSYN):
		ep.inflight[0].flags = wire.FlagSYN | wire.FlagACK
		ep.emitOutstanding(ep.inflight[0], false, "")
	case flags == wire.FlagSYN|wire.FlagACK:
		ep.sendNew(outstanding{seq: ep.iss, flags: wire.FlagSYN | wire.FlagACK})
	case flags == wire.FlagACK:
		ep.emit(wire.Segment{Seq: uint32(ep.sndNxt), Ack: uint32(ep.rcvNxt), Flags: wire.FlagACK}, false, "")
	case flags == wire.FlagFIN:
		ep.finQueued = true
		ep.transmit()
	case flags == wire.FlagRST:
		ep.sendReset()
	}
}

func (ep *Endpoint) sendReset() {
	in := ep.in
	ep.remote = in.SrcPort
	if in.Flags.Has(wire.FlagACK) {
		ep.emit(wire.Segment{Seq: in.Ack, Flags: wire.FlagRST}, false, "porta fechada")
		return
	}
	if !ep.synced {
		ep.irs = Seq(in.Seq)
		ep.synced = true
	}
	ep.emit(wire.Segment{Seq: 0, Ack: uint32(Seq(in.Seq).Add(in.SeqLen())), Flags: wire.FlagRST | wire.FlagACK}, false, "porta fechada")
}

func (ep *Endpoint) transmit() {
	mss := ep.cfg().MSS
	for len(ep.unsent) > 0 {
		size := min(mss, len(ep.unsent))
		if ep.sendWindow()-ep.inFlight() < size {
			break
		}
		payload := slices.Clone(ep.unsent[:size])
		ep.unsent = ep.unsent[size:]
		ep.sendNew(outstanding{seq: ep.sndNxt, flags: wire.FlagACK, payload: payload})
	}
	if len(ep.unsent) == 0 && ep.finQueued && !ep.finSent {
		ep.finSeq = ep.sndNxt
		ep.finSent = true
		ep.sendNew(outstanding{seq: ep.sndNxt, flags: wire.FlagFIN | wire.FlagACK})
	}
}

func (ep *Endpoint) retransmit(reason string) {
	if len(ep.inflight) == 0 {
		return
	}
	ep.inflight[0].retransmitted = true
	ep.probe.active = false
	oldest := ep.inflight[0]
	switch reason {
	case "timeout":
		ep.rtt.Backoff()
		ep.emitOutstanding(oldest, true, reason)
		ep.restartRTO()
	case "fast":
		if deadline, ok := ep.env().Deadline(ep.rto); ok && ep.rto != 0 {
			saved := deadline - ep.env().Now()
			ep.env().Publish(sim.Record{Kind: sim.KindMetric, Metric: "fast_retransmit_saved", Value: float64(saved), Detail: fmt.Sprintf("timeout previsto em %v; retransmissão rápida agora economiza %v", deadline, saved)})
		}
		ep.emitOutstanding(oldest, true, reason)
		ep.restartRTO()
	default:
		ep.emitOutstanding(oldest, true, reason)
		ep.restartRTO()
	}
}

func (ep *Endpoint) ingestAck() {
	in := ep.in
	if !in.Flags.Has(wire.FlagACK) {
		return
	}
	ack := Seq(in.Ack)
	if ep.sndNxt.Before(ack) {
		return
	}
	switch {
	case ep.sndUna.Before(ack):
		ep.peerWindow = int(in.Window)
		ep.acceptNewAck(ack)
	case ack == ep.sndUna && ep.isDuplicateAck():
		ep.signal(EventAckDup)
	default:
		ep.peerWindow = int(in.Window)
	}
}

func (ep *Endpoint) isDuplicateAck() bool {
	in := ep.in
	return len(in.Payload) == 0 && !in.Flags.Has(wire.FlagSYN) && !in.Flags.Has(wire.FlagFIN) && ep.sndUna != ep.sndNxt
}

func (ep *Endpoint) acceptNewAck(ack Seq) {
	ackedData := 0
	kept := make([]outstanding, 0, len(ep.inflight))
	for _, o := range ep.inflight {
		switch {
		case o.end().AtOrBefore(ack):
			ackedData += len(o.payload)
		case o.seq.Before(ack) && len(o.payload) > 0:
			cut := ack.Minus(o.seq)
			ackedData += cut
			o.payload = o.payload[cut:]
			o.seq = ack
			kept = append(kept, o)
		default:
			kept = append(kept, o)
		}
	}
	ep.inflight = kept
	ep.sndUna = ack
	if ep.probe.active && ep.probe.end.AtOrBefore(ack) {
		ep.probe.active = false
		sample := ep.env().Now() - ep.probe.sentAt
		ep.rtt.Observe(sample)
		ep.env().Publish(sim.Record{
			Kind:   sim.KindMetric,
			Metric: "rtt_sample",
			Value:  float64(sample),
			Detail: fmt.Sprintf("SampleRTT=%v EstimatedRTT=%v DevRTT=%v TimeoutInterval=%v", sample, ep.rtt.Estimated(), ep.rtt.Deviation(), ep.rtt.TimeoutInterval()),
		})
	}
	ep.rtt.ResetBackoff()
	if ep.sndUna == ep.sndNxt {
		ep.stopRTO()
	} else {
		ep.restartRTO()
	}
	if ackedData > 0 {
		ep.signal(EventAckNew)
	}
}

func (ep *Endpoint) acceptData() {
	in := ep.in
	if len(in.Payload) == 0 {
		return
	}
	seq := Seq(in.Seq)
	switch {
	case seq == ep.rcvNxt:
		ep.deliver(in.Payload)
		ep.drainReordered()
	case ep.rcvNxt.Before(seq):
		if _, held := ep.ooo[seq]; held {
			host.Discard(ep.env(), "segmento fora de ordem já armazenado")
			return
		}
		ep.ooo[seq] = slices.Clone(in.Payload)
	default:
		end := seq.Add(len(in.Payload))
		if !ep.rcvNxt.Before(end) {
			host.Discard(ep.env(), "segmento duplicado (já entregue)")
			return
		}
		ep.deliver(in.Payload[ep.rcvNxt.Minus(seq):])
		ep.drainReordered()
	}
}

func (ep *Endpoint) deliver(chunk []byte) {
	ep.rcvNxt = ep.rcvNxt.Add(len(chunk))
	for _, id := range ep.reasm.Feed(chunk) {
		host.Deliver(ep.env(), ep.stack.sink, id)
	}
}

func (ep *Endpoint) drainReordered() {
	for {
		chunk, ok := ep.ooo[ep.rcvNxt]
		if !ok {
			break
		}
		delete(ep.ooo, ep.rcvNxt)
		ep.deliver(chunk)
	}
	for seq, chunk := range ep.ooo {
		if seq.Add(len(chunk)).AtOrBefore(ep.rcvNxt) {
			delete(ep.ooo, seq)
		}
	}
}

func (ep *Endpoint) acceptFin() {
	ep.rcvNxt = ep.rcvNxt.Add(1)
	ep.finRcvd = true
}

func (ep *Endpoint) armRTOIfIdle() {
	if ep.rto == 0 {
		ep.restartRTO()
	}
}

func (ep *Endpoint) restartRTO() {
	ep.stopRTO()
	ep.rto = ep.env().Arm("rto", ep.rtt.TimeoutInterval(), func() {
		ep.rto = 0
		ep.dispatch(EventTimeoutRTO)
	})
}

func (ep *Endpoint) stopRTO() {
	if ep.rto != 0 {
		ep.env().Disarm(ep.rto)
		ep.rto = 0
	}
}

func (ep *Endpoint) startTimeWait() {
	ep.stopTimeWait()
	ep.timeWait = ep.env().Arm("2msl", 2*ep.cfg().MSL, func() {
		ep.timeWait = 0
		ep.dispatch(EventTimeout2MSL)
	})
}

func (ep *Endpoint) stopTimeWait() {
	if ep.timeWait != 0 {
		ep.env().Disarm(ep.timeWait)
		ep.timeWait = 0
	}
}

func (ep *Endpoint) release() {
	ep.stopRTO()
	ep.stopTimeWait()
	ep.inflight = nil
	ep.unsent = nil
	ep.probe.active = false
}

func (ep *Endpoint) drained() bool {
	return len(ep.unsent) == 0 && !slices.ContainsFunc(ep.inflight, func(o outstanding) bool { return len(o.payload) > 0 })
}
