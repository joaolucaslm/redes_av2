import type { Edge, Point, StateSpec } from "../model";

export type Box = { id: string; cx: number; cy: number; w: number; h: number; lines: string[] };

export type Geometry = { d: string; label: Point; anchor: "start" | "middle" | "end"; loop: boolean };

const CHAR = 0.61;

export function wrap(text: string, max: number): string[] {
  if (text.length <= max) {
    return [text];
  }
  const lines: string[] = [];
  let current = "";
  for (const word of text.split(" ")) {
    const candidate = current ? `${current} ${word}` : word;
    if (candidate.length > max && current) {
      lines.push(current);
      current = word;
    } else {
      current = candidate;
    }
  }
  if (current) {
    lines.push(current);
  }
  return lines;
}

export function textWidth(text: string, font: number): number {
  return text.length * font * CHAR;
}

export function makeBox(state: StateSpec, font: number): Box {
  const lines = wrap(state.label || state.id, 16);
  const longest = Math.max(...lines.map((line) => line.length));
  return {
    id: state.id,
    cx: state.x,
    cy: state.y,
    w: Math.max(font * 6, longest * font * CHAR + font * 1.9),
    h: font * 2.15 + (lines.length - 1) * font * 1.2,
    lines,
  };
}

export function clip(box: Box, toward: Point, margin = 3): Point {
  const dx = toward.x - box.cx;
  const dy = toward.y - box.cy;
  if (dx === 0 && dy === 0) {
    return { x: box.cx, y: box.cy };
  }
  const scale = 1 / Math.max(Math.abs(dx) / (box.w / 2 + margin), Math.abs(dy) / (box.h / 2 + margin));
  return { x: box.cx + dx * scale, y: box.cy + dy * scale };
}

function lerp(a: Point, b: Point, t: number): Point {
  return { x: a.x + (b.x - a.x) * t, y: a.y + (b.y - a.y) * t };
}

function distance(a: Point, b: Point): number {
  return Math.hypot(b.x - a.x, b.y - a.y);
}

function fmt(p: Point): string {
  return `${p.x.toFixed(1)} ${p.y.toFixed(1)}`;
}

function roundedPolyline(points: Point[], radius: number): string {
  const first = points[0];
  if (!first) {
    return "";
  }
  let d = `M ${fmt(first)}`;
  for (let i = 1; i < points.length - 1; i++) {
    const prev = points[i - 1];
    const corner = points[i];
    const next = points[i + 1];
    if (!prev || !corner || !next) {
      continue;
    }
    const r = Math.min(radius, distance(prev, corner) / 2, distance(corner, next) / 2);
    const entry = lerp(corner, prev, r / distance(prev, corner));
    const exit = lerp(corner, next, r / distance(corner, next));
    d += ` L ${fmt(entry)} Q ${fmt(corner)} ${fmt(exit)}`;
  }
  const last = points[points.length - 1];
  if (last) {
    d += ` L ${fmt(last)}`;
  }
  return d;
}

function polylineMidpoint(points: Point[]): Point {
  const lengths = points.slice(1).map((p, i) => distance(points[i] ?? p, p));
  const total = lengths.reduce((a, b) => a + b, 0);
  let walked = 0;
  for (let i = 0; i < lengths.length; i++) {
    const len = lengths[i] ?? 0;
    const a = points[i];
    const b = points[i + 1];
    if (a && b && walked + len >= total / 2) {
      return lerp(a, b, (total / 2 - walked) / (len || 1));
    }
    walked += len;
  }
  return points[0] ?? { x: 0, y: 0 };
}

export function edgeGeometry(from: Box, to: Box, edge: Edge, fallbackBend = 0): Geometry {
  const via = edge.via ?? [];
  if (via.length > 0) {
    const head = via[0] as Point;
    const tail = via[via.length - 1] as Point;
    const points = [clip(from, head), ...via, clip(to, tail)];
    return { d: roundedPolyline(points, 18), label: edge.labelAt ?? polylineMidpoint(points), anchor: "middle", loop: false };
  }
  const p0 = { x: from.cx, y: from.cy };
  const p1 = { x: to.cx, y: to.cy };
  const len = distance(p0, p1) || 1;
  const bend = edge.bend ?? fallbackBend;
  const normal = { x: -(p1.y - p0.y) / len, y: (p1.x - p0.x) / len };
  const mid = lerp(p0, p1, 0.5);
  const control = { x: mid.x + normal.x * bend * len, y: mid.y + normal.y * bend * len };
  const start = clip(from, bend ? control : p1);
  const end = clip(to, bend ? control : p0);
  const label = {
    x: 0.25 * start.x + 0.5 * control.x + 0.25 * end.x,
    y: 0.25 * start.y + 0.5 * control.y + 0.25 * end.y,
  };
  return { d: `M ${fmt(start)} Q ${fmt(control)} ${fmt(end)}`, label: edge.labelAt ?? label, anchor: "middle", loop: false };
}

export function loopGeometry(box: Box, side: string, stack: number, font: number): Geometry {
  const ext = font * 1.7 + stack * font * 0.95;
  const spread = font * 0.7;
  const { cx, cy, w, h } = box;
  switch (side) {
    case "left": {
      const sx = cx - w / 2;
      return {
        d: `M ${fmt({ x: sx, y: cy - spread })} C ${fmt({ x: sx - ext, y: cy - spread - ext * 0.8 })} ${fmt({ x: sx - ext, y: cy + spread + ext * 0.8 })} ${fmt({ x: sx, y: cy + spread })}`,
        label: { x: sx - ext * 0.78 - font * 0.4, y: cy - ext * 0.15 },
        anchor: "end",
        loop: true,
      };
    }
    case "right": {
      const sx = cx + w / 2;
      return {
        d: `M ${fmt({ x: sx, y: cy - spread })} C ${fmt({ x: sx + ext, y: cy - spread - ext * 0.8 })} ${fmt({ x: sx + ext, y: cy + spread + ext * 0.8 })} ${fmt({ x: sx, y: cy + spread })}`,
        label: { x: sx + ext * 0.78 + font * 0.4, y: cy - ext * 0.15 },
        anchor: "start",
        loop: true,
      };
    }
    case "bottom": {
      const sy = cy + h / 2;
      return {
        d: `M ${fmt({ x: cx - spread, y: sy })} C ${fmt({ x: cx - spread - ext * 0.8, y: sy + ext })} ${fmt({ x: cx + spread + ext * 0.8, y: sy + ext })} ${fmt({ x: cx + spread, y: sy })}`,
        label: { x: cx, y: sy + ext * 0.8 + font * 1.1 },
        anchor: "middle",
        loop: true,
      };
    }
    default: {
      const sy = cy - h / 2;
      return {
        d: `M ${fmt({ x: cx - spread, y: sy })} C ${fmt({ x: cx - spread - ext * 0.8, y: sy - ext })} ${fmt({ x: cx + spread + ext * 0.8, y: sy - ext })} ${fmt({ x: cx + spread, y: sy })}`,
        label: { x: cx, y: sy - ext * 0.8 - font * 0.5 },
        anchor: "middle",
        loop: true,
      };
    }
  }
}
