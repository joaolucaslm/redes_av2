export type Piece = { t0: number; t1: number; y0: number; y1: number; compressed: boolean };

export type Axis = { pieces: Piece[]; height: number; pxPerMicro: number };

export function buildAxis(spans: [number, number][], pxPerMs: number, top: number, idleMicros: number, gapPx: number): Axis {
  const pxPerMicro = pxPerMs / 1000;
  const sorted = spans.map(([a, b]): [number, number] => [a, Math.max(a, b)]).sort((x, y) => x[0] - y[0]);
  const merged: [number, number][] = [];
  for (const [a, b] of sorted) {
    const last = merged[merged.length - 1];
    if (last && a <= last[1] + idleMicros) {
      last[1] = Math.max(last[1], b);
    } else {
      merged.push([a, b]);
    }
  }
  const pieces: Piece[] = [];
  let y = top;
  merged.forEach(([a, b], i) => {
    const previous = merged[i - 1];
    if (previous) {
      pieces.push({ t0: previous[1], t1: a, y0: y, y1: y + gapPx, compressed: true });
      y += gapPx;
    }
    const height = (b - a) * pxPerMicro;
    pieces.push({ t0: a, t1: b, y0: y, y1: y + height, compressed: false });
    y += height;
  });
  return { pieces, height: y, pxPerMicro };
}

export function yOf(axis: Axis, t: number): number {
  const { pieces } = axis;
  const first = pieces[0];
  if (!first) {
    return 0;
  }
  if (t <= first.t0) {
    return first.y0;
  }
  let lo = 0;
  let hi = pieces.length - 1;
  while (lo < hi) {
    const mid = (lo + hi + 1) >> 1;
    if ((pieces[mid]?.t0 ?? 0) <= t) {
      lo = mid;
    } else {
      hi = mid - 1;
    }
  }
  const piece = pieces[lo] ?? first;
  if (t >= piece.t1) {
    return piece.y1 + (lo === pieces.length - 1 ? (t - piece.t1) * axis.pxPerMicro : 0);
  }
  const span = piece.t1 - piece.t0 || 1;
  return piece.y0 + ((t - piece.t0) / span) * (piece.y1 - piece.y0);
}

const tickChoices = [1, 2, 5, 10, 20, 50, 100, 200, 500, 1000, 2000, 5000];

export function tickStepMicros(pxPerMs: number, minPx: number): number {
  const ms = tickChoices.find((step) => step * pxPerMs >= minPx) ?? 10000;
  return ms * 1000;
}
