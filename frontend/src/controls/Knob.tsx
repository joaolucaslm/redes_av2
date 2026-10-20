import { useId, useRef, type KeyboardEvent, type PointerEvent, type WheelEvent } from "react";

type Props = {
  label: string;
  value: number;
  min: number;
  max: number;
  step: number;
  display: (value: number) => string;
  onChange: (value: number) => void;
  reset?: number;
  accent?: string;
  disabled?: boolean;
};

const START = -135;
const SWEEP = 270;

function polar(r: number, deg: number): [number, number] {
  const rad = (deg * Math.PI) / 180;
  return [30 + r * Math.sin(rad), 30 - r * Math.cos(rad)];
}

function arc(r: number, from: number, to: number): string {
  const [x0, y0] = polar(r, from);
  const [x1, y1] = polar(r, to);
  return `M ${x0.toFixed(2)} ${y0.toFixed(2)} A ${r} ${r} 0 ${to - from > 180 ? 1 : 0} 1 ${x1.toFixed(2)} ${y1.toFixed(2)}`;
}

export function Knob({ label, value, min, max, step, display, onChange, reset, accent = "var(--phos-a)", disabled }: Props) {
  const uid = useId().replace(/[^a-zA-Z0-9]/g, "");
  const drag = useRef<{ y: number; value: number } | null>(null);
  const fraction = Math.min(1, Math.max(0, (value - min) / (max - min || 1)));
  const angle = START + fraction * SWEEP;

  const emit = (raw: number) => {
    const snapped = Math.round((Math.min(max, Math.max(min, raw)) - min) / step) * step + min;
    const fixed = Number(snapped.toFixed(6));
    if (fixed !== value) {
      onChange(fixed);
    }
  };

  const onPointerDown = (e: PointerEvent<SVGSVGElement>) => {
    if (disabled) {
      return;
    }
    e.currentTarget.setPointerCapture(e.pointerId);
    drag.current = { y: e.clientY, value };
  };

  const onPointerMove = (e: PointerEvent<SVGSVGElement>) => {
    if (!drag.current) {
      return;
    }
    const sensitivity = e.shiftKey ? 900 : 170;
    emit(drag.current.value + ((drag.current.y - e.clientY) / sensitivity) * (max - min));
  };

  const onWheel = (e: WheelEvent<SVGSVGElement>) => {
    if (!disabled) {
      emit(value + (e.deltaY < 0 ? step : -step));
    }
  };

  const onKeyDown = (e: KeyboardEvent<SVGSVGElement>) => {
    if (e.key === "ArrowUp" || e.key === "ArrowRight") {
      e.preventDefault();
      e.stopPropagation();
      emit(value + step);
    }
    if (e.key === "ArrowDown" || e.key === "ArrowLeft") {
      e.preventDefault();
      e.stopPropagation();
      emit(value - step);
    }
  };

  return (
    <div className={disabled ? "knob disabled" : "knob"}>
      <svg
        viewBox="0 0 60 60"
        role="slider"
        aria-label={label}
        aria-valuenow={value}
        aria-valuemin={min}
        aria-valuemax={max}
        tabIndex={0}
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={() => (drag.current = null)}
        onPointerCancel={() => (drag.current = null)}
        onWheel={onWheel}
        onKeyDown={onKeyDown}
        onDoubleClick={() => reset !== undefined && !disabled && onChange(reset)}
      >
        <defs>
          <radialGradient id={`${uid}-body`} cx="38%" cy="32%" r="75%">
            <stop offset="0%" stopColor="#4a3f35" />
            <stop offset="55%" stopColor="#241e19" />
            <stop offset="100%" stopColor="#0f0c0a" />
          </radialGradient>
        </defs>
        {Array.from({ length: 11 }, (_, i) => {
          const a = START + (i * SWEEP) / 10;
          const [x0, y0] = polar(25.5, a);
          const [x1, y1] = polar(28.5, a);
          return <line key={i} x1={x0} y1={y0} x2={x1} y2={y1} className={i % 5 === 0 ? "knob-tick major" : "knob-tick"} />;
        })}
        <path d={arc(22, START, START + SWEEP)} className="knob-track" />
        {fraction > 0.001 && <path d={arc(22, START, angle)} className="knob-value" style={{ stroke: accent }} />}
        <circle cx={30} cy={30} r={16.5} fill={`url(#${uid}-body)`} className="knob-body" />
        <circle cx={30} cy={30} r={12} className="knob-cap" />
        {(() => {
          const [x0, y0] = polar(5, angle);
          const [x1, y1] = polar(15, angle);
          return <line x1={x0} y1={y0} x2={x1} y2={y1} className="knob-pointer" />;
        })()}
      </svg>
      <div className="lcd knob-lcd">{display(value)}</div>
      <div className="label knob-label">{label}</div>
    </div>
  );
}
