import { useMemo } from "react";
import { currentFrame, lastTransition, laneRecords, visibleCount } from "../derive";
import { hostClass, protocolLabel } from "../format";
import { useSize } from "../hooks";
import type { MachineDTO, SimRecord } from "../model";
import { setState, useStore } from "../store";
import { StateDiagram, type Probe } from "./StateDiagram";

type Entry = { key: string; lane: number; machine: MachineDTO; label: string };

function Readout({ record }: { record: SimRecord }) {
  const actions = (record.actions ?? []).join("; ") || "Λ";
  return (
    <div className="readout">
      <span className={`readout-host ${hostClass(record.host)}`}>{record.host}</span>
      <span className="readout-states">
        {record.from}
        <span className="arrow">⟶</span>
        {record.to}
      </span>
      <span className="readout-book">
        <span className="readout-event">
          {record.event}
          {record.guard ? <span className="dim"> [{record.guard}]</span> : null}
        </span>
        <span className="readout-action">{actions}</span>
      </span>
    </div>
  );
}

export function DiagramPanel() {
  const catalog = useStore((s) => s.catalog);
  const info = useStore((s) => s.info);
  const lanes = useStore((s) => s.lanes);
  const clock = useStore((s) => s.clock);
  const tab = useStore((s) => s.diagramTab);
  const [screenRef, size] = useSize<HTMLDivElement>();

  const entries = useMemo<Entry[]>(() => {
    if (!catalog || !info) {
      return [];
    }
    return info.lanes.flatMap((lane, index) => {
      const protocol = catalog.protocols.find((p) => p.id === lane.protocol);
      return (protocol?.machines ?? []).map((machine) => ({
        key: `${index}:${machine.protocol}:${machine.machine}`,
        lane: index,
        machine,
        label: info.lanes.length > 1 ? `${protocolLabel(lane.name)} · ${machine.title}` : machine.title,
      }));
    });
  }, [catalog, info]);

  const selected = entries.find((e) => e.key === tab) ?? entries[0];
  const frames = selected ? (lanes[selected.lane] ?? []) : [];
  const cursor = selected ? (clock?.cursors?.[selected.lane] ?? 0) : 0;
  const frame = currentFrame(frames, cursor);
  const records = selected ? laneRecords(selected.lane, frames) : [];
  const count = visibleCount(records, cursor);

  const probes: Probe[] = [];
  if (selected && frame) {
    for (const hostView of frame.snapshot.hosts) {
      const view = hostView.machines.find((m) => m.protocol === selected.machine.protocol && m.machine === selected.machine.machine);
      if (view) {
        probes.push({ host: hostView.name, state: view.state, last: lastTransition(records, count, hostView.name, view.machine) });
      }
    }
  }

  const scale = selected && size.width > 0 ? Math.min(size.width / selected.machine.width, size.height / selected.machine.height) : 1;

  return (
    <section className="panel diagram-panel">
      <header className="panel-head">
        <span className="label">Diagrama de estados</span>
        <span className="rule" />
        <div className="tabs">
          {entries.map((entry) => (
            <button key={entry.key} className={entry === selected ? "key on" : "key"} onClick={() => setState((s) => ({ ...s, diagramTab: entry.key }))}>
              {entry.label}
            </button>
          ))}
        </div>
      </header>
      <div className="panel-body">
        <div className="screen diagram-screen" ref={screenRef}>
          {selected ? <StateDiagram machine={selected.machine} probes={probes} scale={scale} /> : <div className="empty">sem sessão</div>}
        </div>
        {selected && (
          <div className="diagram-caption">
            <span className="label">{selected.machine.protocol}</span>
            <span className="faint">{catalog?.protocols.find((p) => p.machines.includes(selected.machine))?.source}</span>
          </div>
        )}
        <div className="readouts">
          {probes.map((probe) =>
            probe.last ? (
              <Readout key={probe.host} record={probe.last} />
            ) : (
              <div key={probe.host} className="readout">
                <span className={`readout-host ${hostClass(probe.host)}`}>{probe.host}</span>
                <span className="readout-states">{probe.state}</span>
                <span className="faint">nenhuma transição ainda</span>
              </div>
            ),
          )}
        </div>
      </div>
    </section>
  );
}
