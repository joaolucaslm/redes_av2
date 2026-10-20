export type VirtualTime = number;

export type Header = {
  proto: string;
  label: string;
  flags?: string;
  srcPort?: number;
  dstPort?: number;
  seq?: number;
  ack?: number;
  relSeq?: number;
  relAck?: number;
  length?: number;
  payload?: number;
  window?: number;
  checksum?: number;
  message?: number;
  retransmission?: boolean;
  reason?: string;
};

export type Flight = {
  id: number;
  src: string;
  dst: string;
  sentAt: VirtualTime;
  endAt: VirtualTime;
  fate: "delivered" | "lost";
  cause?: string;
  corrupted?: boolean;
  flipped?: number[];
  duplicate?: boolean;
  reordered?: boolean;
  header: Header;
};

export type RecordKind =
  | "transition"
  | "rejection"
  | "send"
  | "arrival"
  | "loss"
  | "discard"
  | "timer-arm"
  | "timer-stop"
  | "timeout"
  | "delivery"
  | "app"
  | "operator"
  | "metric";

export type SimRecord = {
  seq: number;
  step: number;
  at: VirtualTime;
  kind: RecordKind;
  host?: string;
  protocol?: string;
  machine?: string;
  from?: string;
  event?: string;
  guard?: string;
  actions?: string[];
  to?: string;
  rule: number;
  edge: number;
  reason?: string;
  timer?: string;
  deadline?: VirtualTime;
  flight?: Flight;
  message?: number;
  verdict?: string;
  metric?: string;
  value?: number;
  detail?: string;
};

export type Fired = {
  machine: string;
  from: string;
  event: string;
  guard?: string;
  actions: string[] | null;
  to: string;
  rule: number;
  edge: number;
};

export type MachineView = {
  protocol: string;
  machine: string;
  state: string;
  last: Fired;
  hasLast: boolean;
};

export type Register = { group: string; name: string; value: string };

export type TimerView = { name: string; deadline: VirtualTime };

export type Tally = {
  offered: number;
  delivered: number;
  inOrder: number;
  outOfOrder: number;
  duplicates: number;
  last: number;
};

export type HostView = {
  name: string;
  addr: string;
  role: string;
  machines: MachineView[];
  registers: Register[];
  timers: TimerView[];
  app: Tally;
};

export type ChannelParams = {
  loss: number;
  delayMin: VirtualTime;
  delayMax: VirtualTime;
  corrupt: number;
  corruptBits: number;
  duplicate: number;
  reorder: number;
  reorderLag: VirtualTime;
};

export type ChannelView = {
  params: ChannelParams;
  inTransit: number;
  armed: string[] | null;
  rules: number;
};

export type Stats = {
  offered: number;
  delivered: number;
  inOrder: number;
  outOfOrder: number;
  duplicates: number;
  missing: number;
  sent: number;
  bytesOnWire: number;
  retransmissions: number;
  fastRetransmits: number;
  timeouts: number;
  lost: number;
  corrupted: number;
  duplicated: number;
  reordered: number;
  discarded: number;
  rejected: number;
  transitions: number;
  rttSamples: number;
  firstOfferAt: VirtualTime;
  lastDeliveryAt: VirtualTime;
  totalTime: VirtualTime;
  fastRetransmitSaved: VirtualTime;
};

export type Snapshot = {
  step: number;
  now: VirtualTime;
  protocol: string;
  family: string;
  hosts: HostView[];
  channel: ChannelView;
  stats: Stats;
  pending: number;
  nextAt: VirtualTime;
  idle: boolean;
};

export type Frame = {
  index: number;
  at: VirtualTime;
  cause: string;
  snapshot: Snapshot;
  records: SimRecord[];
};

export type LaneInfo = { name: string; protocol: string; family: string };

export type Info = {
  scenario: string;
  scenarioTitle: string;
  lanes: LaneInfo[];
  seed: number;
  channel: ChannelParams;
  msl: VirtualTime;
  fastRetransmit: boolean;
  revision: number;
};

export type Clock = {
  time: VirtualTime;
  cursors: number[];
  frontier: number[];
  playing: boolean;
  speed: number;
  done: boolean;
  end: VirtualTime;
  revision: number;
};

export type Patch = { lane: number; base: number; frames: Frame[] | null; revision: number };

export type Timeline = { info: Info; lanes: Frame[][] | null; clock: Clock };

export type StateSpec = { id: string; label: string; x: number; y: number; loop: string; note: string };

export type Point = { x: number; y: number };

export type EdgeLabel = { event: string; action: string };

export type Edge = {
  index: number;
  rule: number;
  from: string;
  event: string;
  guard?: string;
  actions: string[] | null;
  to: string;
  label: EdgeLabel;
  bend?: number;
  via?: Point[];
  labelAt?: Point;
  loop?: string;
  quiet?: boolean;
};

export type MachineDTO = {
  protocol: string;
  machine: string;
  title: string;
  initial: string;
  width: number;
  height: number;
  states: StateSpec[];
  edges: Edge[] | null;
};

export type ProtocolDTO = {
  id: string;
  family: string;
  title: string;
  source: string;
  summary: string;
  machines: MachineDTO[];
};

export type ScenarioDTO = {
  id: string;
  number: string;
  title: string;
  description: string;
  expected: string;
  runs: LaneInfo[];
  seed: number;
  screenshot: boolean;
};

export type Defaults = {
  seed: number;
  channel: ChannelParams;
  msl: VirtualTime;
  fastRetransmit: boolean;
  slowMotion: number;
  speeds: number[];
};

export type Catalog = {
  protocols: ProtocolDTO[];
  scenarios: ScenarioDTO[];
  defaults: Defaults;
  warnings: string[] | null;
};

export type LoadRequest = {
  scenario?: string;
  protocols?: string[];
  sideBySide?: boolean;
  seed?: number;
  channel?: ChannelParams;
  msl?: VirtualTime;
  fastRetransmit?: boolean;
};

export type Check = { title: string; run?: string; passed: boolean; detail: string };

export type PathDTO = { host: string; machine: string; states: string[] };

export type RunResult = {
  name: string;
  protocol: string;
  seed: number;
  stats: Stats;
  steps: number;
  end: VirtualTime;
  paths: PathDTO[];
  halted?: string;
};

export type ScenarioResult = {
  id: string;
  number: string;
  title: string;
  description?: string;
  expected?: string;
  protocols: string[];
  seed: number;
  passed: boolean;
  checks: Check[] | null;
  runs: RunResult[] | null;
  error?: string;
};

export type Fault = { message: string };
