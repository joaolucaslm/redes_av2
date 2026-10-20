import type { Flight, Frame, SimRecord } from "./model";

type Cache = { frames: Frame[]; records: SimRecord[] };

const caches = new Map<number, Cache>();

export function laneRecords(lane: number, frames: Frame[]): SimRecord[] {
  const cached = caches.get(lane);
  if (cached && cached.frames === frames) {
    return cached.records;
  }
  let records: SimRecord[];
  const previous = cached?.frames;
  const shared =
    previous !== undefined &&
    previous.length > 0 &&
    frames.length >= previous.length &&
    frames[previous.length - 1] === previous[previous.length - 1];
  if (shared && cached) {
    records = cached.records.slice();
    for (let i = previous.length; i < frames.length; i++) {
      records.push(...(frames[i]?.records ?? []));
    }
  } else {
    records = frames.flatMap((f) => f.records);
  }
  caches.set(lane, { frames, records });
  return records;
}

export function visibleCount(records: SimRecord[], cursor: number): number {
  let lo = 0;
  let hi = records.length;
  while (lo < hi) {
    const mid = (lo + hi) >> 1;
    if ((records[mid]?.step ?? 0) <= cursor) {
      lo = mid + 1;
    } else {
      hi = mid;
    }
  }
  return lo;
}

export type Trace = {
  flights: { record: SimRecord; flight: Flight }[];
  states: SimRecord[];
  timeouts: SimRecord[];
  discards: SimRecord[];
  deliveries: SimRecord[];
  rejections: SimRecord[];
  timers: { host: string; name: string; from: number; to: number; outcome: "fired" | "stopped" | "open"; step: number }[];
};

export function buildTrace(records: SimRecord[], count: number): Trace {
  const trace: Trace = { flights: [], states: [], timeouts: [], discards: [], deliveries: [], rejections: [], timers: [] };
  const open = new Map<string, number>();
  for (let i = 0; i < count; i++) {
    const r = records[i];
    if (!r) {
      continue;
    }
    const key = `${r.host ?? ""}/${r.timer ?? ""}`;
    switch (r.kind) {
      case "send":
        if (r.flight) {
          trace.flights.push({ record: r, flight: r.flight });
        }
        break;
      case "transition":
        if (r.from !== r.to) {
          trace.states.push(r);
        }
        break;
      case "timeout": {
        trace.timeouts.push(r);
        const at = open.get(key);
        if (at !== undefined) {
          const timer = trace.timers[at];
          if (timer) {
            timer.to = r.at;
            timer.outcome = "fired";
          }
          open.delete(key);
        }
        break;
      }
      case "timer-arm":
        open.set(key, trace.timers.length);
        trace.timers.push({ host: r.host ?? "", name: r.timer ?? "", from: r.at, to: r.deadline ?? r.at, outcome: "open", step: r.step });
        break;
      case "timer-stop": {
        const at = open.get(key);
        if (at !== undefined) {
          const timer = trace.timers[at];
          if (timer) {
            timer.to = r.at;
            timer.outcome = "stopped";
          }
          open.delete(key);
        }
        break;
      }
      case "discard":
        trace.discards.push(r);
        break;
      case "delivery":
        trace.deliveries.push(r);
        break;
      case "rejection":
        trace.rejections.push(r);
        break;
      default:
        break;
    }
  }
  return trace;
}

export function lastTransition(records: SimRecord[], count: number, host: string, machine: string): SimRecord | null {
  for (let i = count - 1; i >= 0; i--) {
    const r = records[i];
    if (r && r.kind === "transition" && r.host === host && r.machine === machine) {
      return r;
    }
  }
  return null;
}

export function currentFrame(frames: Frame[], cursor: number): Frame | null {
  return frames[Math.min(cursor, frames.length - 1)] ?? null;
}
