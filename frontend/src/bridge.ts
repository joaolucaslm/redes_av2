import type { Catalog, ChannelParams, Info, LoadRequest, ScenarioResult, Timeline } from "./model";

export type BenchAPI = {
  Catalog(): Promise<Catalog>;
  Load(req: LoadRequest): Promise<Info>;
  Reset(): Promise<Info>;
  Timeline(): Promise<Timeline>;
  Play(): Promise<void>;
  Pause(): Promise<void>;
  StepForward(): Promise<void>;
  StepBack(): Promise<void>;
  SeekFrame(lane: number, frame: number): Promise<void>;
  SeekTime(micros: number): Promise<void>;
  SetSpeed(speed: number): Promise<void>;
  SetChannel(params: ChannelParams): Promise<void>;
  DropNext(from: string): Promise<void>;
  AppCommand(host: string, verb: string, count: number): Promise<void>;
  Transfer(messages: number, size: number): Promise<void>;
  RunSuite(): Promise<ScenarioResult[] | null>;
  ReloadDefinitions(): Promise<Catalog>;
};

type WailsRuntime = {
  EventsOn(name: string, callback: (...data: unknown[]) => void): () => void;
};

declare global {
  interface Window {
    go?: { app?: { Bench?: BenchAPI } };
    runtime?: WailsRuntime;
  }
}

export const Topics = {
  session: "bench:session",
  patch: "bench:patch",
  clock: "bench:clock",
  fault: "bench:fault",
} as const;

export function bench(): BenchAPI | null {
  return window.go?.app?.Bench ?? null;
}

export function subscribe<T>(topic: string, handler: (payload: T) => void): () => void {
  const runtime = window.runtime;
  if (!runtime) {
    return () => undefined;
  }
  return runtime.EventsOn(topic, (payload) => handler(payload as T));
}
