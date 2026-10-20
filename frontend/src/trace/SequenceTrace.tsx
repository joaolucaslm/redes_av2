import { useLayoutEffect, useMemo, type ReactNode } from "react";
import { buildTrace, currentFrame, laneRecords, visibleCount } from "../derive";
import { hostColor, protocolLabel, reasonLabel, timeLong, timeShort } from "../format";
import { useSize } from "../hooks";
import type { Flight, HostView, SimRecord } from "../model";
import { act, NO_FRAMES, setState, useStore } from "../store";
import { buildAxis, tickStepMicros, yOf } from "./axis";

const TOP = 18;
const IDLE = 180_000;
const GAP_PX = 34;
const COMPONENTS = new Set(["tcp-congestion"]);

type Props = { lane: number };

function zigzag(x0: number, y0: number, x1: number, y1: number): string {
  const len = Math.hypot(x1 - x0, y1 - y0);
  const steps = Math.max(2, Math.floor(len / 7));
  const nx = -(y1 - y0) / (len || 1);
  const ny = (x1 - x0) / (len || 1);
  let d = `M ${x0.toFixed(1)} ${y0.toFixed(1)}`;
  for (let i = 1; i <= steps; i++) {
    const t = i / steps;
    const amp = i === steps ? 0 : (i % 2 === 0 ? 1 : -1) * 3.2;
    d += ` L ${(x0 + (x1 - x0) * t + nx * amp).toFixed(1)} ${(y0 + (y1 - y0) * t + ny * amp).toFixed(1)}`;
  }
  return d;
}

function flightColor(f: Flight): string {
  if (f.header.retransmission) {
    return "var(--amber)";
  }
  if (f.corrupted) {
    return "var(--magenta)";
  }
  return hostColor(f.src);
}

function flightLabel(f: Flight): string {
  const marks = [
    f.fate === "lost" ? "✕" : "",
    f.header.retransmission ? "↻" : "",
    f.duplicate ? "⧉" : "",
    f.corrupted ? "⚡" : "",
    f.reordered ? "↯" : "",
  ].join("");
  return `${marks ? `${marks} ` : ""}${f.header.label}`;
}

function tooltip(f: Flight): string {
  const h = f.header;
  const lines = [
    `#${f.id} ${f.src}→${f.dst} ${h.proto.toUpperCase()} ${h.flags ?? ""}`,
    h.label,
    `enviado ${timeLong(f.sentAt)} · ${f.fate === "lost" ? "perdido" : "chega"} ${timeLong(f.endAt)}`,
  ];
  if (h.seq !== undefined || h.ack !== undefined) {
    lines.push(`seq=${h.seq ?? 0} ack=${h.ack ?? 0} (relativos ${h.relSeq ?? 0}/${h.relAck ?? 0}) janela=${h.window ?? 0}`);
  }
  lines.push(`portas ${h.srcPort ?? "-"}→${h.dstPort ?? "-"} · ${h.length ?? 0} B · checksum 0x${(h.checksum ?? 0).toString(16).padStart(4, "0").toUpperCase()}`);
  if (h.retransmission) {
    lines.push(`retransmissão: ${reasonLabel(h.reason)}`);
  }
  if (f.corrupted) {
    lines.push(`bits trocados: ${(f.flipped ?? []).join(", ")}`);
  }
  if (f.duplicate) {
    lines.push("cópia criada pelo canal");
  }
  if (f.reordered) {
    lines.push("atrasado para reordenar");
  }
  if (f.fate === "lost") {
    lines.push(`perda ${f.cause ?? ""}`);
  }
  return lines.join("\n");
}

function hostLegend(view: HostView | undefined, side: "a" | "b") {
  if (!view) {
    return null;
  }
  return (
    <div className={`trace-host ${side}`}>
      <span className={side === "a" ? "host-a" : "host-b"}>{view.name}</span>
      <span className="faint">{view.addr}</span>
      <span className="label">{view.role}</span>
    </div>
  );
}

export function SequenceTrace({ lane }: Props) {
  const frames = useStore((s) => s.lanes[lane] ?? NO_FRAMES);
  const clock = useStore((s) => s.clock);
  const info = useStore((s) => s.info);
  const zoom = useStore((s) => s.zoom);
  const follow = useStore((s) => s.follow);
  const [scrollRef, size] = useSize<HTMLDivElement>();
  const cursor = clock?.cursors?.[lane] ?? 0;
  const time = clock?.time ?? 0;
  const records = laneRecords(lane, frames);
  const count = visibleCount(records, cursor);
  const trace = useMemo(() => buildTrace(records, count), [records, count]);
  const frame = currentFrame(frames, cursor);
  const width = Math.max(size.width, 260);
  const narrow = width < 420;
  const xA = narrow ? 92 : 112;
  const xB = width - 92;
  const xOf = (host: string) => (host === "B" ? xB : xA);
  const outward = (host: string) => (host === "B" ? 1 : -1);

  const axis = useMemo(() => {
    const spans: [number, number][] = [[0, 0], [time, time]];
    for (const { flight } of trace.flights) {
      spans.push([flight.sentAt, flight.endAt]);
    }
    for (const r of [...trace.states, ...trace.timeouts, ...trace.discards, ...trace.deliveries]) {
      spans.push([r.at, r.at]);
    }
    return buildAxis(spans, zoom, TOP, IDLE, GAP_PX);
  }, [trace, time, zoom]);

  const yNow = yOf(axis, time);
  const height = Math.max(axis.height, yNow) + 60;

  useLayoutEffect(() => {
    const el = scrollRef.current;
    if (el && follow) {
      el.scrollTop = Math.max(0, yNow - el.clientHeight * 0.62);
    }
  }, [yNow, follow, scrollRef]);

  const slots = useMemo(() => {
    const result = new Map<number, number>();
    let groupStart = -1;
    let groupHost = "";
    let index = 0;
    for (const { flight } of trace.flights) {
      if (flight.src === groupHost && flight.sentAt - groupStart < 2000) {
        index++;
      } else {
        groupStart = flight.sentAt;
        groupHost = flight.src;
        index = 0;
      }
      result.set(flight.id, index);
    }
    return result;
  }, [trace]);

  const flights = trace.flights.map(({ record, flight }) => {
    const x0 = xOf(flight.src);
    const lost = flight.fate === "lost";
    const x1 = lost ? (x0 + xOf(flight.dst)) / 2 : xOf(flight.dst);
    const y0 = yOf(axis, flight.sentAt);
    const y1 = yOf(axis, flight.endAt);
    const progress = Math.min(1, Math.max(0, (time - flight.sentAt) / (flight.endAt - flight.sentAt || 1)));
    const hx = x0 + (x1 - x0) * progress;
    const hy = y0 + (y1 - y0) * progress;
    const slot = slots.get(flight.id) ?? 0;
    const t = lost ? 0.62 : 0.5;
    const angle = (Math.atan2(y1 - y0, x1 - x0) * 180) / Math.PI;
    const len = Math.hypot(hx - x0, hy - y0) || 1;
    return {
      record,
      flight,
      lost,
      x0,
      y0,
      x1,
      y1,
      hx,
      hy,
      progress,
      flying: progress < 1,
      color: flightColor(flight),
      t,
      lx: x0 + (x1 - x0) * t,
      ly: y0 + (y1 - y0) * t,
      dy: slot % 2 === 0 ? -5 - Math.floor(slot / 2) * 14 : 13 + Math.floor(slot / 2) * 14,
      angle,
      readable: angle > 90 || angle < -90 ? angle + 180 : angle,
      ox: (-(hy - y0) / len) * 1.6,
      oy: ((hx - x0) / len) * 1.6,
    };
  });

  const tick = tickStepMicros(zoom, 34);
  const ruler: ReactNode[] = [];
  for (const piece of axis.pieces) {
    if (piece.compressed) {
      const y = (piece.y0 + piece.y1) / 2;
      ruler.push(
        <g key={`gap-${piece.t0}`} className="trace-gap">
          <path d={`M ${xA - 10} ${y - 4} l 6 -4 l 6 8 l 6 -8 l 6 8 l 6 -4`} />
          <path d={`M ${xB - 26} ${y - 4} l 6 -4 l 6 8 l 6 -8 l 6 8 l 6 -4`} />
          <text x={(xA + xB) / 2} y={y + 4} textAnchor="middle">
            ≈ +{timeShort(piece.t1 - piece.t0)} ocioso
          </text>
        </g>,
      );
      continue;
    }
    for (let t = Math.ceil(piece.t0 / tick) * tick; t <= piece.t1; t += tick) {
      const y = yOf(axis, t);
      const major = Math.round(t / tick) % 5 === 0;
      ruler.push(
        <g key={`t-${t}`} className={major ? "trace-tick major" : "trace-tick"}>
          <line x1={xA} x2={xB} y1={y} y2={y} />
          <text x={4} y={y + 3}>
            {timeShort(t)}
          </text>
        </g>,
      );
    }
  }

  const hosts = frame?.snapshot.hosts ?? [];
  const laneInfo = info?.lanes[lane];

  return (
    <section className="panel trace-panel">
      <header className="panel-head">
        <span className="label">Diagrama de sequência{laneInfo && (info?.lanes.length ?? 0) > 1 ? ` · ${protocolLabel(laneInfo.name)}` : ""}</span>
        <span className="rule" />
        <button className={follow ? "key on" : "key"} onClick={() => setState((s) => ({ ...s, follow: !s.follow }))} title="Acompanhar o cursor">
          seguir
        </button>
        <div className="segmented">
          <button className="key" onClick={() => setState((s) => ({ ...s, zoom: Math.max(0.3, s.zoom / 1.5) }))} title="Menos pixels por ms">
            −
          </button>
          <span className="lcd zoom-lcd">{zoom.toFixed(1)} px/ms</span>
          <button className="key" onClick={() => setState((s) => ({ ...s, zoom: Math.min(20, s.zoom * 1.5) }))} title="Mais pixels por ms">
            +
          </button>
        </div>
      </header>
      <div className="panel-body">
        <div className="screen trace-screen">
          <div className="trace-legend">
            {hostLegend(hosts[0], "a")}
            {hostLegend(hosts[1], "b")}
          </div>
          <div
            className="trace-scroll scroll"
            ref={scrollRef}
            onWheel={(e) => {
              if (e.deltaY < 0 && follow) {
                setState((s) => ({ ...s, follow: false }));
              }
            }}
          >
            <svg className="trace" width={width} height={height}>
              <defs>
                <filter id={`trace-glow-${lane}`} x="-20%" y="-20%" width="140%" height="140%">
                  <feGaussianBlur stdDeviation="2.2" result="b" />
                  <feMerge>
                    <feMergeNode in="b" />
                    <feMergeNode in="SourceGraphic" />
                  </feMerge>
                </filter>
              </defs>
              <g>{ruler}</g>
              <line className="track" x1={xA} x2={xA} y1={0} y2={height} style={{ stroke: "var(--phos-a-dim)" }} />
              <line className="track" x1={xB} x2={xB} y1={0} y2={height} style={{ stroke: "var(--phos-b-dim)" }} />

              {trace.timers.map((timer, i) => {
                const host = timer.host;
                const x = xOf(host) + outward(host) * 7;
                const end = Math.min(timer.to, time);
                if (end < timer.from) {
                  return null;
                }
                const y0 = yOf(axis, timer.from);
                const y1 = yOf(axis, end);
                const fired = timer.outcome === "fired" && timer.to <= time;
                return (
                  <g key={`timer-${i}`} className={`trace-timer ${timer.name === "2msl" ? "msl" : "rto"}`}>
                    <line x1={x} x2={x} y1={y0} y2={y1} />
                    <line x1={x - 3} x2={x + 3} y1={y0} y2={y0} />
                    {fired ? (
                      <text x={x + outward(host) * 4} y={y1 + 3} textAnchor={host === "B" ? "start" : "end"}>
                        ⏱ {timer.name.toUpperCase()}
                      </text>
                    ) : timer.outcome === "stopped" ? (
                      <line x1={x - 3} x2={x + 3} y1={y1} y2={y1} />
                    ) : null}
                  </g>
                );
              })}

              {trace.states
                .filter((r) => !COMPONENTS.has(r.protocol ?? ""))
                .map((r) => {
                  const host = r.host ?? "A";
                  const y = yOf(axis, r.at);
                  const x = xOf(host) + outward(host) * 16;
                  return (
                    <g key={`st-${r.seq}`} className="trace-state" style={{ color: hostColor(host) }}>
                      <circle cx={xOf(host)} cy={y} r={2.6} />
                      <text x={x} y={y + 3.5} textAnchor={host === "B" ? "start" : "end"}>
                        {r.to}
                      </text>
                    </g>
                  );
                })}

              {flights.map((g) => (
                <g
                  key={`f-${g.flight.id}`}
                  className={`trace-flight${g.flying ? " flying" : ""}${g.lost ? " lost" : ""}`}
                  style={{ color: g.color }}
                  onClick={() => act((api) => api.SeekFrame(lane, g.record.step))}
                  filter={g.flying ? `url(#trace-glow-${lane})` : undefined}
                >
                  <title>{tooltip(g.flight)}</title>
                  {g.flight.corrupted ? (
                    <path d={zigzag(g.x0, g.y0, g.hx, g.hy)} className="wire" />
                  ) : g.flight.duplicate ? (
                    <>
                      <line x1={g.x0 + g.ox} y1={g.y0 + g.oy} x2={g.hx + g.ox} y2={g.hy + g.oy} className="wire" />
                      <line x1={g.x0 - g.ox} y1={g.y0 - g.oy} x2={g.hx - g.ox} y2={g.hy - g.oy} className="wire" />
                    </>
                  ) : (
                    <line x1={g.x0} y1={g.y0} x2={g.hx} y2={g.hy} className={g.flight.reordered ? "wire reordered" : "wire"} />
                  )}
                  <line x1={g.x0} y1={g.y0} x2={g.hx} y2={g.hy} className="hit" />
                  {g.flying ? (
                    <circle cx={g.hx} cy={g.hy} r={3} className="head" />
                  ) : g.lost ? null : (
                    <path
                      d={`M ${g.x1} ${g.y1} l ${-Math.cos(((g.angle - 24) * Math.PI) / 180) * 9} ${-Math.sin(((g.angle - 24) * Math.PI) / 180) * 9} M ${g.x1} ${g.y1} l ${-Math.cos(((g.angle + 24) * Math.PI) / 180) * 9} ${-Math.sin(((g.angle + 24) * Math.PI) / 180) * 9}`}
                      className="wire arrowhead"
                    />
                  )}
                </g>
              ))}

              {flights
                .filter((g) => g.lost && !g.flying)
                .map((g) => (
                  <g key={`x-${g.flight.id}`} className="loss-mark">
                    <title>{`perda ${g.flight.cause ?? ""}: ${g.flight.header.label}`}</title>
                    <circle cx={g.x1} cy={g.y1} r={9} />
                    <line x1={g.x1 - 6} y1={g.y1 - 6} x2={g.x1 + 6} y2={g.y1 + 6} />
                    <line x1={g.x1 - 6} y1={g.y1 + 6} x2={g.x1 + 6} y2={g.y1 - 6} />
                  </g>
                ))}

              <g className="flight-labels">
                {flights
                  .filter((g) => g.progress > g.t || !g.flying)
                  .map((g) => (
                    <text
                      key={`l-${g.flight.id}`}
                      x={g.lx}
                      y={g.ly}
                      transform={`rotate(${g.readable} ${g.lx} ${g.ly})`}
                      textAnchor="middle"
                      dy={g.dy}
                      className="flight-label"
                      style={{ color: g.color }}
                    >
                      {flightLabel(g.flight)}
                    </text>
                  ))}
              </g>

              {trace.discards.map((r: SimRecord) => {
                const host = r.host ?? "B";
                const y = yOf(axis, r.at);
                const x = xOf(host) - outward(host) * 12;
                return (
                  <g key={`d-${r.seq}`} className="trace-discard">
                    <title>{r.reason}</title>
                    <circle cx={x} cy={y} r={4.5} />
                    <line x1={x - 4} y1={y + 4} x2={x + 4} y2={y - 4} />
                  </g>
                );
              })}

              {trace.deliveries.map((r: SimRecord) => {
                const host = r.host ?? "B";
                const y = yOf(axis, r.at);
                const x = xOf(host) - outward(host) * 5;
                return <circle key={`v-${r.seq}`} cx={x} cy={y} r={2.2} className={`trace-delivery ${r.verdict === "nova" ? "fresh" : r.verdict === "duplicada" ? "dup" : "late"}`}><title>{`mensagem #${r.message} · ${r.verdict}`}</title></circle>;
              })}

              {trace.rejections.map((r: SimRecord) => {
                const host = r.host ?? "A";
                const y = yOf(axis, r.at);
                return (
                  <text key={`r-${r.seq}`} x={xOf(host) - outward(host) * 10} y={y + 4} className="trace-reject" textAnchor="middle">
                    <title>{`${r.event} rejeitado em ${r.from}: ${r.reason}`}</title>!
                  </text>
                );
              })}

              <g className="trace-cursor">
                <line x1={0} x2={width} y1={yNow} y2={yNow} />
                <path d={`M 0 ${yNow - 5} L 7 ${yNow} L 0 ${yNow + 5} Z`} />
                <path d={`M ${width} ${yNow - 5} L ${width - 7} ${yNow} L ${width} ${yNow + 5} Z`} />
              </g>
            </svg>
          </div>
        </div>
      </div>
    </section>
  );
}
