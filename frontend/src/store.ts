import { useSyncExternalStore } from "react";
import { bench, subscribe, Topics, type BenchAPI } from "./bridge";
import type { Catalog, Clock, Fault, Frame, Info, LoadRequest, Patch, ScenarioResult, Timeline } from "./model";

export type LogMode = "transitions" | "all";

export type Selection = {
  scenario: string;
  protocols: [string, string];
  sideBySide: boolean;
  seed: number;
};

export type State = {
  connected: boolean;
  catalog: Catalog | null;
  info: Info | null;
  lanes: Frame[][];
  clock: Clock | null;
  faults: { id: number; message: string; level: "error" | "info" }[];
  panel: "bench" | "suite";
  selection: Selection;
  focusLane: number;
  diagramTab: string;
  log: { host: string; text: string; mode: LogMode };
  follow: boolean;
  zoom: number;
  dropFrom: string;
  burst: number;
  suite: { running: boolean; results: ScenarioResult[] | null; open: string | null };
};

const initial: State = {
  connected: false,
  catalog: null,
  info: null,
  lanes: [],
  clock: null,
  faults: [],
  panel: "bench",
  selection: { scenario: "01-handshake", protocols: ["tcp", "udp"], sideBySide: false, seed: 1 },
  focusLane: 0,
  diagramTab: "",
  log: { host: "", text: "", mode: "transitions" },
  follow: true,
  zoom: 2.4,
  dropFrom: "",
  burst: 10,
  suite: { running: false, results: null, open: null },
};

export const NO_FRAMES: Frame[] = [];

export const DEFAULT_SPEEDS = [0.25, 0.5, 1, 2, 4, 8, 16, 32, 64];

let state = initial;
const listeners = new Set<() => void>();
let faultSeq = 0;

export function getState(): State {
  return state;
}

export function setState(update: (s: State) => State): void {
  const next = update(state);
  if (next === state) {
    return;
  }
  state = next;
  listeners.forEach((listener) => listener());
}

function listen(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function useStore<T>(selector: (s: State) => T): T {
  return useSyncExternalStore(listen, () => selector(state));
}

export function pushFault(message: string, level: "error" | "info" = "error"): void {
  const id = ++faultSeq;
  setState((s) => ({ ...s, faults: [...s.faults.slice(-3), { id, message, level }] }));
  window.setTimeout(() => setState((s) => ({ ...s, faults: s.faults.filter((f) => f.id !== id) })), 6000);
}

export async function act<T>(run: (api: BenchAPI) => Promise<T>): Promise<T | undefined> {
  const api = bench();
  if (!api) {
    pushFault("bindings do Wails indisponíveis: abra pelo wails dev ou pelo binário");
    return undefined;
  }
  try {
    return await run(api);
  } catch (err) {
    pushFault(String(err));
    return undefined;
  }
}

function normalizeFrame(frame: Frame): Frame {
  return { ...frame, records: frame.records ?? [] };
}

function applyPatch(patch: Patch): void {
  let stale = false;
  setState((s) => {
    if (!s.info || patch.revision !== s.info.revision) {
      return s;
    }
    const lanes = s.lanes.slice();
    const current = lanes[patch.lane] ?? [];
    if (patch.base > current.length) {
      stale = true;
      return s;
    }
    lanes[patch.lane] = current.slice(0, patch.base).concat((patch.frames ?? []).map(normalizeFrame));
    return { ...s, lanes };
  });
  if (stale) {
    void resync();
  }
}

function adoptTimeline(timeline: Timeline): void {
  setState((s) => ({
    ...s,
    info: timeline.info,
    lanes: (timeline.lanes ?? []).map((frames) => frames.map(normalizeFrame)),
    clock: timeline.clock,
    focusLane: Math.min(s.focusLane, Math.max(0, (timeline.info.lanes?.length ?? 1) - 1)),
  }));
}

export async function resync(): Promise<void> {
  const timeline = await act((api) => api.Timeline());
  if (timeline) {
    adoptTimeline(timeline);
  }
}

export function requestFor(selection: Selection, extra: Partial<LoadRequest> = {}): LoadRequest {
  if (selection.scenario) {
    return { scenario: selection.scenario, sideBySide: selection.sideBySide, ...extra };
  }
  const protocols = selection.sideBySide ? [...selection.protocols] : [selection.protocols[0]];
  return { protocols, seed: selection.seed, ...extra };
}

export async function load(selection: Selection, extra: Partial<LoadRequest> = {}): Promise<void> {
  setState((s) => ({ ...s, selection, diagramTab: "" }));
  const info = await act((api) => api.Load(requestFor(selection, extra)));
  if (info) {
    setState((s) => ({ ...s, selection: { ...s.selection, seed: info.seed }, focusLane: 0 }));
    await resync();
  }
}

export async function boot(): Promise<void> {
  const api = bench();
  if (!api) {
    setState((s) => ({ ...s, connected: false }));
    return;
  }
  subscribe<Info>(Topics.session, (info) =>
    setState((s) => ({ ...s, info, lanes: info.lanes.map(() => []), clock: null })),
  );
  subscribe<Patch>(Topics.patch, applyPatch);
  subscribe<Clock>(Topics.clock, (clock) =>
    setState((s) => (s.info && clock.revision === s.info.revision ? { ...s, clock } : s)),
  );
  subscribe<Fault>(Topics.fault, (fault) => pushFault(fault.message));
  const catalog = await act((a) => a.Catalog());
  setState((s) => ({ ...s, connected: true, catalog: catalog ?? null }));
  (catalog?.warnings ?? []).forEach((w) => pushFault(w));
  await load(getState().selection);
}

export async function reloadDefinitions(): Promise<void> {
  const catalog = await act((api) => api.ReloadDefinitions());
  if (!catalog) {
    return;
  }
  setState((s) => ({ ...s, catalog }));
  (catalog.warnings ?? []).forEach((w) => pushFault(w));
  const { selection } = getState();
  const known = !selection.scenario || catalog.scenarios.some((sc) => sc.id === selection.scenario);
  await load(known ? selection : { ...selection, scenario: "" });
  pushFault(`definições recarregadas do disco: ${catalog.protocols.length} protocolos, ${catalog.scenarios.length} cenários`, "info");
}
