import { useState } from "react";
import { currentFrame } from "../derive";
import { hostClass, protocolLabel, timeShort } from "../format";
import type { HostView, Stats } from "../model";
import { setState, useStore } from "../store";

const comparison: { label: string; pick: (s: Stats) => string }[] = [
  { label: "oferecidas", pick: (s) => String(s.offered) },
  { label: "entregues", pick: (s) => String(s.delivered) },
  { label: "perdidas p/ app", pick: (s) => String(s.missing) },
  { label: "duplicadas", pick: (s) => String(s.duplicates) },
  { label: "em ordem", pick: (s) => String(s.inOrder) },
  { label: "fora de ordem", pick: (s) => String(s.outOfOrder) },
  { label: "segmentos", pick: (s) => String(s.sent) },
  { label: "retransmissões", pick: (s) => String(s.retransmissions) },
  { label: "timeouts", pick: (s) => String(s.timeouts) },
  { label: "perdidos no canal", pick: (s) => String(s.lost) },
  { label: "descartados", pick: (s) => String(s.discarded) },
  { label: "tempo total", pick: (s) => (s.delivered > 0 ? timeShort(s.lastDeliveryAt) : "—") },
  { label: "economia FR", pick: (s) => (s.fastRetransmitSaved > 0 ? timeShort(s.fastRetransmitSaved) : "—") },
];

function HostColumn({ view, now }: { view: HostView; now: number }) {
  const groups = new Map<string, HostView["registers"]>();
  for (const r of view.registers ?? []) {
    groups.set(r.group, [...(groups.get(r.group) ?? []), r]);
  }
  return (
    <div className="host-column">
      <div className={`host-title ${hostClass(view.name)}`}>
        {view.name} <span className="faint">{view.addr}</span> <span className="label">{view.role}</span>
      </div>
      {[...groups.entries()].map(([group, registers]) => (
        <div key={group} className="reg-group">
          <div className="label reg-group-title">{group}</div>
          {registers.map((r) => (
            <div key={r.name} className="reg">
              <span className="dim">{r.name}</span>
              <span>{r.value}</span>
            </div>
          ))}
        </div>
      ))}
      <div className="reg-group">
        <div className="label reg-group-title">temporizadores</div>
        {(view.timers ?? []).length === 0 ? <div className="faint reg">nenhum armado</div> : null}
        {(view.timers ?? []).map((t) => (
          <div key={`${t.name}-${t.deadline}`} className="reg">
            <span className="dim">{t.name}</span>
            <span className="lcd amber">−{timeShort(Math.max(0, t.deadline - now))}</span>
          </div>
        ))}
      </div>
      <div className="reg-group">
        <div className="label reg-group-title">aplicação</div>
        <div className="reg">
          <span className="dim">oferecidas / entregues</span>
          <span>
            {view.app.offered} / {view.app.delivered}
          </span>
        </div>
        <div className="reg">
          <span className="dim">duplicadas · fora de ordem</span>
          <span>
            {view.app.duplicates} · {view.app.outOfOrder}
          </span>
        </div>
      </div>
    </div>
  );
}

export function Inspector() {
  const [tab, setTab] = useState<"regs" | "compare">("regs");
  const info = useStore((s) => s.info);
  const lanes = useStore((s) => s.lanes);
  const clock = useStore((s) => s.clock);
  const focus = useStore((s) => s.focusLane);
  const frame = currentFrame(lanes[focus] ?? [], clock?.cursors?.[focus] ?? 0);
  const dual = (info?.lanes.length ?? 0) > 1;

  return (
    <section className="panel inspector-panel">
      <header className="panel-head">
        <div className="segmented">
          <button className={tab === "regs" ? "key on" : "key"} onClick={() => setTab("regs")}>
            registradores
          </button>
          <button className={tab === "compare" ? "key on" : "key"} onClick={() => setTab("compare")}>
            comparar
          </button>
        </div>
        <span className="rule" />
        {dual && (
          <div className="segmented">
            {info?.lanes.map((lane, i) => (
              <button key={lane.name} className={focus === i ? "key on" : "key"} onClick={() => setState((s) => ({ ...s, focusLane: i }))}>
                {protocolLabel(lane.name)}
              </button>
            ))}
          </div>
        )}
      </header>
      <div className="panel-body scroll inspector-body">
        {tab === "regs" ? (
          <div className="host-columns">{frame?.snapshot.hosts.map((h) => <HostColumn key={h.name} view={h} now={clock?.time ?? 0} />)}</div>
        ) : (
          <table className="compare">
            <thead>
              <tr>
                <th />
                {info?.lanes.map((lane) => (
                  <th key={lane.name} className="label">
                    {protocolLabel(lane.name)}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {comparison.map((row) => (
                <tr key={row.label}>
                  <td className="dim">{row.label}</td>
                  {info?.lanes.map((lane, i) => {
                    const f = currentFrame(lanes[i] ?? [], clock?.cursors?.[i] ?? 0);
                    return <td key={lane.name}>{f ? row.pick(f.snapshot.stats) : "—"}</td>;
                  })}
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </section>
  );
}
