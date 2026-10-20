import { useEffect, useRef, useState } from "react";
import { currentFrame } from "../derive";
import { percent, timeShort } from "../format";
import { useDebounced } from "../hooks";
import type { ChannelParams } from "../model";
import { act, getState, load, NO_FRAMES, setState, useStore } from "../store";
import { Knob } from "./Knob";

const MS = 1000;

export function ChannelRack() {
  const info = useStore((s) => s.info);
  const focus = useStore((s) => s.focusLane);
  const frames = useStore((s) => s.lanes[s.focusLane] ?? NO_FRAMES);
  const cursor = useStore((s) => s.clock?.cursors?.[s.focusLane] ?? 0);
  const dropFrom = useStore((s) => s.dropFrom);
  const defaults = useStore((s) => s.catalog?.defaults);
  const frame = currentFrame(frames, cursor);
  const live = frame?.snapshot.channel.params ?? info?.channel;
  const [draft, setDraft] = useState<ChannelParams | null>(null);
  const release = useRef<number | undefined>(undefined);
  const commit = useDebounced<ChannelParams>((p) => void act((api) => api.SetChannel(p)), 140);

  useEffect(() => () => window.clearTimeout(release.current), []);

  if (!live) {
    return (
      <section className="panel rack-panel">
        <div className="empty">canal</div>
      </section>
    );
  }
  const params = draft ?? live;
  const change = (patch: Partial<ChannelParams>) => {
    const next = { ...params, ...patch };
    if (patch.delayMin !== undefined && next.delayMin > next.delayMax) {
      next.delayMax = next.delayMin;
    }
    if (patch.delayMax !== undefined && next.delayMax < next.delayMin) {
      next.delayMin = next.delayMax;
    }
    setDraft(next);
    commit(next);
    window.clearTimeout(release.current);
    release.current = window.setTimeout(() => setDraft(null), 900);
  };
  const armed = frame?.snapshot.channel.armed ?? [];
  const inTransit = frame?.snapshot.channel.inTransit ?? 0;
  const rules = frame?.snapshot.channel.rules ?? 0;
  const base = defaults?.channel;
  const hasTCP = (info?.lanes ?? []).some((l) => l.family === "tcp");

  return (
    <section className="panel rack-panel">
      <header className="panel-head">
        <span className="label">Canal simulado</span>
        <span className="rule" />
        <span className="faint">{info && info.lanes.length > 1 ? `aplica às ${info.lanes.length} faixas` : `faixa ${focus + 1}`}</span>
      </header>
      <div className="panel-body">
        <div className="knob-grid">
          <Knob label="perda" value={params.loss} min={0} max={1} step={0.01} display={percent} onChange={(v) => change({ loss: v })} reset={base?.loss} accent="var(--red)" />
          <Knob label="corrupção" value={params.corrupt} min={0} max={1} step={0.01} display={percent} onChange={(v) => change({ corrupt: v })} reset={base?.corrupt} accent="var(--magenta)" />
          <Knob label="duplicação" value={params.duplicate} min={0} max={1} step={0.01} display={percent} onChange={(v) => change({ duplicate: v })} reset={base?.duplicate} accent="var(--violet)" />
          <Knob label="reordem" value={params.reorder} min={0} max={1} step={0.01} display={percent} onChange={(v) => change({ reorder: v })} reset={base?.reorder} accent="var(--amber)" />
          <Knob label="atraso mín" value={params.delayMin} min={0} max={400 * MS} step={MS} display={timeShort} onChange={(v) => change({ delayMin: v })} reset={base?.delayMin} accent="var(--phos-b)" />
          <Knob label="atraso máx" value={params.delayMax} min={0} max={400 * MS} step={MS} display={timeShort} onChange={(v) => change({ delayMax: v })} reset={base?.delayMax} accent="var(--phos-b)" />
          <Knob label="atraso reordem" value={params.reorderLag} min={0} max={400 * MS} step={MS} display={timeShort} onChange={(v) => change({ reorderLag: v })} reset={base?.reorderLag} accent="var(--amber)" />
          <Knob label="bits trocados" value={params.corruptBits} min={1} max={8} step={1} display={(v) => `${v} bit${v > 1 ? "s" : ""}`} onChange={(v) => change({ corruptBits: v })} reset={1} accent="var(--magenta)" />
        </div>
        <div className="drop-row">
          <button className={armed.length > 0 ? "key danger lit" : "key danger"} onClick={() => act((api) => api.DropNext(getState().dropFrom))} title="Atalho: D">
            <span className={armed.length > 0 ? "led on" : "led"} />
            descartar próximo
          </button>
          <div className="segmented">
            {[
              ["", "qualquer"],
              ["A", "de A"],
              ["B", "de B"],
            ].map(([value, text]) => (
              <button key={value} className={dropFrom === value ? "key on" : "key"} onClick={() => setState((s) => ({ ...s, dropFrom: value ?? "" }))}>
                {text}
              </button>
            ))}
          </div>
        </div>
        <div className="rack-status">
          <span>
            <span className={inTransit > 0 ? "led green on" : "led green"} /> trânsito <b>{inTransit}</b>
          </span>
          <span>
            <span className={rules > 0 ? "led amber on" : "led amber"} /> regras <b>{rules}</b>
          </span>
          <span>
            armados <b>{armed.length}</b>
          </span>
        </div>
        {hasTCP && info && <TcpUnit msl={info.msl} fastRetransmit={info.fastRetransmit} fallback={defaults?.msl} />}
      </div>
    </section>
  );
}

function TcpUnit({ msl, fastRetransmit, fallback }: { msl: number; fastRetransmit: boolean; fallback: number | undefined }) {
  const [draft, setDraft] = useState<number | null>(null);
  const reload = useDebounced<number>((value) => {
    void load(getState().selection, { msl: value, fastRetransmit }).then(() => setDraft(null));
  }, 450);
  const shown = draft ?? msl;
  return (
    <div className="subrack">
      <span className="label">unidade TCP · reinicia a sessão</span>
      <div className="subrack-row">
        <Knob
          label="MSL"
          value={shown}
          min={100 * MS}
          max={5000 * MS}
          step={100 * MS}
          display={(v) => `${timeShort(v)} · 2MSL ${timeShort(2 * v)}`}
          onChange={(v) => {
            setDraft(v);
            reload(v);
          }}
          reset={fallback}
          accent="var(--violet)"
        />
        <button className={fastRetransmit ? "key toggle on" : "key toggle"} onClick={() => void load(getState().selection, { msl, fastRetransmit: !fastRetransmit })}>
          <span className={fastRetransmit ? "led amber on" : "led amber"} />
          retransmissão rápida
        </button>
      </div>
    </div>
  );
}
