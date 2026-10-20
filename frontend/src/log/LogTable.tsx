import { useLayoutEffect, useMemo, useRef, useState } from "react";
import { laneRecords, visibleCount } from "../derive";
import { hostClass, protocolLabel, timeLong } from "../format";
import type { SimRecord } from "../model";
import { act, NO_FRAMES, setState, useStore, type LogMode } from "../store";

const ROW = 21;

function describe(r: SimRecord): string {
  const f = r.flight;
  switch (r.kind) {
    case "send":
      return `envia #${f?.id} ${f?.header.label ?? ""}${f?.header.retransmission ? " (retransmissão)" : ""}${f?.duplicate ? " (cópia do canal)" : ""}`;
    case "arrival":
      return `chega #${f?.id} ${f?.header.label ?? ""}${f?.corrupted ? " (corrompido)" : ""}`;
    case "loss":
      return `perde #${f?.id} ${f?.header.label ?? ""} (${f?.cause ?? ""})`;
    case "discard":
      return `descarta: ${r.reason ?? ""}`;
    case "timer-arm":
      return `arma ${r.timer} até ${timeLong(r.deadline ?? 0)}`;
    case "timer-stop":
      return `desarma ${r.timer}`;
    case "timeout":
      return `expira ${r.timer}`;
    case "delivery":
      return `entrega mensagem #${r.message} (${r.verdict})`;
    case "metric":
      return r.detail ?? `${r.metric} = ${r.value}`;
    case "app":
      return `app ${r.event}${r.detail ? ` · ${r.detail}` : ""}`;
    case "operator":
      return r.detail ?? r.reason ?? `operador ${r.event}`;
    default:
      return r.reason ?? "";
  }
}

function matches(r: SimRecord, mode: LogMode, host: string, text: string): boolean {
  if (mode === "transitions" && r.kind !== "transition" && r.kind !== "rejection") {
    return false;
  }
  if (host && r.host !== host) {
    return false;
  }
  if (!text) {
    return true;
  }
  const haystack = [r.from, r.to, r.event, r.guard, r.machine, ...(r.actions ?? []), r.reason, r.kind, r.flight?.header.label]
    .join(" ")
    .toLowerCase();
  return text
    .toLowerCase()
    .split(/\s+/)
    .every((term) => haystack.includes(term));
}

export function LogTable() {
  const focus = useStore((s) => s.focusLane);
  const info = useStore((s) => s.info);
  const frames = useStore((s) => s.lanes[s.focusLane] ?? NO_FRAMES);
  const cursor = useStore((s) => s.clock?.cursors?.[s.focusLane] ?? 0);
  const filter = useStore((s) => s.log);
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const [scrollTop, setScrollTop] = useState(0);
  const [viewport, setViewport] = useState(150);
  const [pinned, setPinned] = useState(true);
  const records = laneRecords(focus, frames);
  const count = visibleCount(records, cursor);
  const rows = useMemo(
    () => records.slice(0, count).filter((r) => matches(r, filter.mode, filter.host, filter.text.trim())),
    [records, count, filter],
  );

  useLayoutEffect(() => {
    const el = scrollRef.current;
    if (!el) {
      return;
    }
    setViewport(el.clientHeight);
    if (pinned) {
      el.scrollTop = el.scrollHeight;
      setScrollTop(el.scrollTop);
    }
  }, [rows.length, pinned]);

  const first = Math.max(0, Math.floor(scrollTop / ROW) - 6);
  const last = Math.min(rows.length, Math.ceil((scrollTop + viewport) / ROW) + 6);
  const patchFilter = (patch: Partial<typeof filter>) => setState((s) => ({ ...s, log: { ...s.log, ...patch } }));

  return (
    <section className="panel log-panel">
      <header className="panel-head">
        <span className="label">Log de transições</span>
        {info && info.lanes.length > 1 && (
          <div className="segmented">
            {info.lanes.map((lane, i) => (
              <button key={lane.name} className={focus === i ? "key on" : "key"} onClick={() => setState((s) => ({ ...s, focusLane: i }))}>
                {protocolLabel(lane.name)}
              </button>
            ))}
          </div>
        )}
        <span className="rule" />
        <div className="segmented">
          {[
            ["", "todos"],
            ["A", "A"],
            ["B", "B"],
          ].map(([value, text]) => (
            <button key={value} className={filter.host === value ? "key on" : "key"} onClick={() => patchFilter({ host: value ?? "" })}>
              {text}
            </button>
          ))}
        </div>
        <input className="log-filter" placeholder="filtrar estado, evento, ação…" value={filter.text} onChange={(e) => patchFilter({ text: e.target.value })} />
        <div className="segmented">
          <button className={filter.mode === "transitions" ? "key on" : "key"} onClick={() => patchFilter({ mode: "transitions" })}>
            transições
          </button>
          <button className={filter.mode === "all" ? "key on" : "key"} onClick={() => patchFilter({ mode: "all" })}>
            tudo
          </button>
        </div>
        <button className={pinned ? "key on" : "key"} onClick={() => setPinned(!pinned)} title="Manter o fim da tabela visível">
          fim
        </button>
        <span className="faint">{rows.length} linhas</span>
      </header>
      <div className="log-head">
        <span>tempo</span>
        <span>host</span>
        <span>máquina</span>
        <span>estado anterior</span>
        <span>evento [guarda]</span>
        <span>ações</span>
        <span>novo estado</span>
      </div>
      <div
        className="log-body scroll"
        ref={scrollRef}
        onScroll={(e) => {
          const el = e.currentTarget;
          setScrollTop(el.scrollTop);
          const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < ROW;
          if (atBottom !== pinned) {
            setPinned(atBottom);
          }
        }}
      >
        <div style={{ height: rows.length * ROW, position: "relative" }}>
          {rows.slice(first, last).map((r, i) => {
            const rejected = r.kind === "rejection";
            const transition = r.kind === "transition";
            return (
              <div
                key={r.seq}
                className={`log-row${r.step === cursor ? " current" : ""}${rejected ? " rejected" : ""}${transition && r.from !== r.to ? " changes" : ""}`}
                style={{ top: (first + i) * ROW }}
                onClick={() => act((api) => api.SeekFrame(focus, r.step))}
              >
                <span className="dim">{timeLong(r.at)}</span>
                <span className={hostClass(r.host)}>{r.host ?? "·"}</span>
                <span className="dim">{r.machine ?? r.kind}</span>
                <span>{transition || rejected ? r.from : ""}</span>
                <span>
                  {transition || rejected ? r.event : describe(r)}
                  {r.guard ? <span className="dim"> [{r.guard}]</span> : null}
                </span>
                <span className={rejected ? "reject-reason" : "dim"}>{rejected ? `rejeitado: ${r.reason}` : (r.actions ?? []).join("; ")}</span>
                <span className={transition && r.from !== r.to ? hostClass(r.host) : ""}>{transition ? r.to : ""}</span>
              </div>
            );
          })}
        </div>
      </div>
    </section>
  );
}
