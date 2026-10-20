import { useRef, type PointerEvent } from "react";
import { protocolLabel, timeLong } from "../format";
import { useSize } from "../hooks";
import { act, DEFAULT_SPEEDS, useStore } from "../store";
import { transport } from "./actions";

const icons = {
  reset: "M3 3h2v10H3zM13 3v10L6 8z",
  back: "M3 3h2v10H3zM14 3v10L7 8z",
  play: "M4 2.5v11l9-5.5z",
  pause: "M4 3h3v10H4zM9 3h3v10H9z",
  step: "M11 3h2v10h-2zM2 3v10l7-5z",
};

function Icon({ path }: { path: string }) {
  return (
    <svg viewBox="0 0 16 16" aria-hidden="true">
      <path d={path} fill="currentColor" />
    </svg>
  );
}


function Scrubber() {
  const clock = useStore((s) => s.clock);
  const lanes = useStore((s) => s.lanes);
  const [ref, size] = useSize<HTMLDivElement>();
  const dragging = useRef(false);
  const lastSeek = useRef(0);
  const end = Math.max(clock?.end ?? 0, clock?.time ?? 0, 1);
  const width = Math.max(1, size.width);
  const columns = Math.max(1, Math.floor(width / 3));
  const density = lanes.map((frames) => {
    const buckets = new Array<number>(columns).fill(0);
    for (const f of frames) {
      const i = Math.min(columns - 1, Math.floor((f.at / end) * columns));
      buckets[i] = (buckets[i] ?? 0) + 1;
    }
    return buckets;
  });
  const peak = Math.max(1, ...density.flat());

  const seek = (e: PointerEvent<HTMLDivElement>, force: boolean) => {
    const rect = e.currentTarget.getBoundingClientRect();
    const t = Math.min(1, Math.max(0, (e.clientX - rect.left) / rect.width)) * end;
    const now = performance.now();
    if (force || now - lastSeek.current > 50) {
      lastSeek.current = now;
      void act((api) => api.SeekTime(Math.round(t)));
    }
  };

  const needle = ((clock?.time ?? 0) / end) * width;
  return (
    <div
      className="scrubber"
      ref={ref}
      onPointerDown={(e) => {
        dragging.current = true;
        e.currentTarget.setPointerCapture(e.pointerId);
        seek(e, true);
      }}
      onPointerMove={(e) => dragging.current && seek(e, false)}
      onPointerUp={(e) => {
        dragging.current = false;
        seek(e, true);
      }}
    >
      <svg width={width} height="100%" preserveAspectRatio="none" viewBox={`0 0 ${width} 30`}>
        {density.map((buckets, lane) =>
          buckets.map((n, i) =>
            n > 0 ? (
              <rect
                key={`${lane}-${i}`}
                x={i * 3}
                width={2}
                y={lane === 0 ? 15 - (Math.log1p(n) / Math.log1p(peak)) * 13 : 15}
                height={(Math.log1p(n) / Math.log1p(peak)) * 13}
                className={lane === 0 ? "density a" : "density b"}
              />
            ) : null,
          ),
        )}
        <line x1={0} x2={width} y1={15} y2={15} className="scrub-axis" />
        <rect x={0} y={0} width={Math.max(0, needle)} height={30} className="scrub-played" />
        <line x1={needle} x2={needle} y1={0} y2={30} className="scrub-needle" />
      </svg>
    </div>
  );
}

export function Transport() {
  const clock = useStore((s) => s.clock);
  const info = useStore((s) => s.info);
  const speeds = useStore((s) => s.catalog?.defaults.speeds ?? DEFAULT_SPEEDS);
  const slowMotion = useStore((s) => s.catalog?.defaults.slowMotion ?? 20);
  const playing = clock?.playing ?? false;
  const speed = clock?.speed ?? 1;

  return (
    <section className="panel transport">
      <div className="transport-keys">
        <button className="key" onClick={transport.reset} title="Reset (R)">
          <Icon path={icons.reset} />
        </button>
        <button className="key" onClick={transport.back} title="Voltar um passo (←)">
          <Icon path={icons.back} />
        </button>
        <button className={playing ? "key play on" : "key play"} onClick={transport.toggle} title="Tocar / pausar (espaço)">
          <Icon path={playing ? icons.pause : icons.play} />
        </button>
        <button className="key" onClick={transport.step} title="Avançar um passo (→)">
          <Icon path={icons.step} />
        </button>
      </div>
      <div className="transport-speed">
        <span className="label">velocidade</span>
        <div className="segmented">
          {speeds.map((s) => (
            <button key={s} className={speed === s ? "key on" : "key"} onClick={() => act((api) => api.SetSpeed(s))}>
              {s < 1 ? s.toString().replace(".", ",") : s}×
            </button>
          ))}
        </div>
        <span className="faint rate">1× = 1 ms virtual em {slowMotion} ms reais</span>
      </div>
      <div className="transport-scrub">
        <Scrubber />
      </div>
      <div className="transport-readouts">
        <span className="lcd time-lcd">t = {timeLong(clock?.time ?? 0)}</span>
        <span className="steps">
          {(info?.lanes ?? []).map((lane, i) => (
            <span key={lane.name} className="dim">
              {info && info.lanes.length > 1 ? `${protocolLabel(lane.name)} ` : ""}passo{" "}
              <b className="lcd amber">
                {clock?.cursors?.[i] ?? 0}/{Math.max(0, (clock?.frontier?.[i] ?? 1) - 1)}
              </b>
            </span>
          ))}
          {clock?.done ? <span className="label done">fim · sem eventos pendentes</span> : null}
        </span>
      </div>
    </section>
  );
}
