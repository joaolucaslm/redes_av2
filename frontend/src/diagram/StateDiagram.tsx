import { useId, useMemo } from "react";
import type { Edge, MachineDTO, Point, SimRecord } from "../model";
import { hostColor } from "../format";
import { edgeGeometry, loopGeometry, makeBox, textWidth, wrap, type Box, type Geometry } from "./geometry";

export type Probe = { host: string; state: string; last: SimRecord | null };

type Props = { machine: MachineDTO; probes: Probe[]; scale: number };

type Layout = {
  boxes: Map<string, Box>;
  rules: { edge: Edge; geometry: Geometry }[];
  loops: { edge: Edge; geometry: Geometry }[];
  groups: { key: string; rules: number[]; lines: string[]; side: string; geometry: Geometry }[];
  byEdge: Map<number, Geometry>;
};

const clamp = (v: number, lo: number, hi: number) => Math.min(hi, Math.max(lo, v));

function buildLayout(machine: MachineDTO, font: number, edgeFont: number): Layout {
  const boxes = new Map(machine.states.map((s) => [s.id, makeBox(s, font)]));
  const sideOf = (e: Edge) => e.loop || machine.states.find((s) => s.id === e.from)?.loop || "top";
  const edges = machine.edges ?? [];
  const firstOfRule = new Map<number, Edge>();
  for (const edge of edges) {
    if (!firstOfRule.has(edge.rule)) {
      firstOfRule.set(edge.rule, edge);
    }
  }
  const rules: Layout["rules"] = [];
  const loops: Layout["loops"] = [];
  const ruleGeometry = new Map<number, Geometry>();
  const stacks = new Map<string, number>();
  const groups = new Map<string, Layout["groups"][number]>();
  for (const edge of firstOfRule.values()) {
    const from = boxes.get(edge.from);
    const to = boxes.get(edge.to);
    if (!from || !to || edge.quiet) {
      continue;
    }
    if (edge.from === edge.to) {
      const key = `${edge.from}|${sideOf(edge)}`;
      const stack = stacks.get(key) ?? 0;
      stacks.set(key, stack + 1);
      const geometry = loopGeometry(from, sideOf(edge), stack, edgeFont);
      loops.push({ edge, geometry });
      ruleGeometry.set(edge.rule, geometry);
      const group = groups.get(key);
      if (group) {
        group.rules.push(edge.rule);
        group.lines.push(edge.label.event);
        group.geometry = geometry;
      } else {
        groups.set(key, { key, rules: [edge.rule], lines: [edge.label.event], side: sideOf(edge), geometry });
      }
    } else {
      const geometry = edgeGeometry(from, to, edge);
      rules.push({ edge, geometry });
      ruleGeometry.set(edge.rule, geometry);
    }
  }
  const byEdge = new Map<number, Geometry>();
  for (const edge of edges) {
    const known = ruleGeometry.get(edge.rule);
    if (known) {
      byEdge.set(edge.index, known);
      continue;
    }
    const from = boxes.get(edge.from);
    const to = boxes.get(edge.to);
    if (!from || !to) {
      continue;
    }
    if (edge.from === edge.to) {
      const key = `${edge.from}|${sideOf(edge)}`;
      byEdge.set(edge.index, loopGeometry(from, sideOf(edge), stacks.get(key) ?? 0, edgeFont));
    } else {
      byEdge.set(edge.index, edgeGeometry(from, to, edge, 0.16));
    }
  }
  return { boxes, rules, loops, groups: [...groups.values()], byEdge };
}

type LabelProps = {
  at: Point;
  anchor: "start" | "middle" | "end";
  event: string;
  action: string;
  font: number;
  className: string;
  color?: string;
};

function BookLabel({ at, anchor, event, action, font, className, color }: LabelProps) {
  const eventLines = wrap(event, 30);
  const actionLines = action.split("\n").flatMap((line) => wrap(line, 34));
  const all = [...eventLines, ...actionLines];
  const width = Math.max(...all.map((line) => textWidth(line, font))) + font * 1.1;
  const gap = font * 1.2;
  const x0 = anchor === "middle" ? at.x - width / 2 : anchor === "start" ? at.x : at.x - width;
  const tx = anchor === "middle" ? at.x : anchor === "start" ? x0 + font * 0.55 : x0 + width - font * 0.55;
  const top = at.y - gap * eventLines.length - font * 0.25;
  const height = gap * all.length + font * 0.6;
  return (
    <g className={className} style={color ? { color } : undefined}>
      <rect x={x0} y={top} width={width} height={height} rx={2} className="label-bg" />
      {eventLines.map((line, i) => (
        <text key={`e${i}`} x={tx} y={at.y - gap * (eventLines.length - 1 - i) - font * 0.42} textAnchor={anchor} fontSize={font}>
          {line}
        </text>
      ))}
      <line x1={x0 + font * 0.4} x2={x0 + width - font * 0.4} y1={at.y} y2={at.y} className="label-rule" />
      {actionLines.map((line, i) => (
        <text key={`a${i}`} x={tx} y={at.y + gap * (i + 1) - font * 0.3} textAnchor={anchor} fontSize={font} className="label-action">
          {line}
        </text>
      ))}
    </g>
  );
}

export function StateDiagram({ machine, probes, scale }: Props) {
  const uid = useId().replace(/[^a-zA-Z0-9]/g, "");
  const font = clamp(12.5 / scale, 10, 26);
  const edgeFont = clamp(10.4 / scale, 8.5, 21);
  const layout = useMemo(() => buildLayout(machine, font, edgeFont), [machine, font, edgeFont]);
  const edges = machine.edges ?? [];
  const occupied = new Map<string, string[]>();
  for (const probe of probes) {
    occupied.set(probe.state, [...(occupied.get(probe.state) ?? []), probe.host]);
  }
  const hot = probes.flatMap((probe) => {
    const r = probe.last;
    if (!r || r.protocol !== machine.protocol || r.machine !== machine.machine) {
      return [];
    }
    const edge = edges[r.edge];
    const geometry = layout.byEdge.get(r.edge);
    return edge && geometry ? [{ probe, record: r, edge, geometry }] : [];
  });
  const hotRules = new Set(hot.map((h) => h.edge.rule));
  const initial = layout.boxes.get(machine.initial);
  const arrow = (name: string) => `url(#${uid}-${name})`;

  return (
    <svg className="fsm" viewBox={`0 0 ${machine.width} ${machine.height}`} preserveAspectRatio="xMidYMid meet">
      <defs>
        {[
          ["dim", "var(--fsm-edge)"],
          ["A", "var(--phos-a)"],
          ["B", "var(--phos-b)"],
        ].map(([name, color]) => (
          <marker key={name} id={`${uid}-${name}`} viewBox="0 0 10 10" refX="9" refY="5" markerWidth={font * 0.62} markerHeight={font * 0.62} markerUnits="userSpaceOnUse" orient="auto-start-reverse">
            <path d="M 0 0 L 10 5 L 0 10 z" fill={color} />
          </marker>
        ))}
        <filter id={`${uid}-glow`} x="-50%" y="-50%" width="200%" height="200%">
          <feGaussianBlur stdDeviation={font * 0.22} result="blur" />
          <feMerge>
            <feMergeNode in="blur" />
            <feMergeNode in="SourceGraphic" />
          </feMerge>
        </filter>
      </defs>

      <g className="fsm-edges">
        {layout.rules.map(({ edge, geometry }) => (
          <path key={edge.rule} d={geometry.d} className={hotRules.has(edge.rule) ? "edge was-hot" : "edge"} markerEnd={arrow("dim")}>
            <title>{`${edge.from} --${edge.event}${edge.guard ? ` [${edge.guard}]` : ""}--> ${edge.to}\n${(edge.actions ?? []).join("; ") || "Λ"}`}</title>
          </path>
        ))}
        {layout.loops.map(({ edge, geometry }) => (
          <path key={edge.rule} d={geometry.d} className="edge loop" markerEnd={arrow("dim")}>
            <title>{`${edge.event}${edge.guard ? ` [${edge.guard}]` : ""} / ${(edge.actions ?? []).join("; ") || "Λ"}`}</title>
          </path>
        ))}
      </g>

      <g className="fsm-labels">
        {layout.rules.map(({ edge, geometry }) =>
          hotRules.has(edge.rule) ? null : (
            <BookLabel key={edge.rule} at={geometry.label} anchor={geometry.anchor} event={edge.label.event} action={edge.label.action} font={edgeFont} className="edge-label" />
          ),
        )}
        {layout.groups.map(({ key, rules, lines, side, geometry }) => {
          if (rules.some((rule) => hotRules.has(rule))) {
            return null;
          }
          const step = edgeFont * 1.15;
          const first = side === "top" ? geometry.label.y - step * (lines.length - 1) : side === "bottom" ? geometry.label.y : geometry.label.y - (step * (lines.length - 1)) / 2;
          return (
            <text key={key} x={geometry.label.x} textAnchor={geometry.anchor} fontSize={edgeFont} className="loop-label">
              {lines.map((line, i) => (
                <tspan key={i} x={geometry.label.x} y={first + i * step}>
                  {line}
                </tspan>
              ))}
            </text>
          );
        })}
      </g>

      {initial && (
        <g className="initial">
          <circle cx={initial.cx - initial.w / 2 - font * 2.2} cy={initial.cy - initial.h / 2 - font * 1.6} r={font * 0.32} />
          <line
            x1={initial.cx - initial.w / 2 - font * 2.2}
            y1={initial.cy - initial.h / 2 - font * 1.6}
            x2={initial.cx - initial.w / 2 + font * 0.2}
            y2={initial.cy - initial.h / 2 + font * 0.1}
            markerEnd={arrow("dim")}
          />
        </g>
      )}

      <g className="fsm-states">
        {[...layout.boxes.values()].map((box) => {
          const hosts = occupied.get(box.id) ?? [];
          const spec = machine.states.find((s) => s.id === box.id);
          return (
            <g key={box.id} className={hosts.length > 0 ? "state lit" : "state"}>
              <title>{spec?.note ? `${box.id} — ${spec.note}` : box.id}</title>
              <rect x={box.cx - box.w / 2} y={box.cy - box.h / 2} width={box.w} height={box.h} rx={font * 0.45} />
              {box.lines.map((line, i) => (
                <text key={i} x={box.cx} y={box.cy + (i - (box.lines.length - 1) / 2) * font * 1.2 + font * 0.36} textAnchor="middle" fontSize={font}>
                  {line}
                </text>
              ))}
              {hosts.map((host, i) => (
                <g key={host} style={{ color: hostColor(host) }}>
                  <rect
                    className="probe"
                    x={box.cx - box.w / 2 - 4 - i * 4.5}
                    y={box.cy - box.h / 2 - 4 - i * 4.5}
                    width={box.w + 8 + i * 9}
                    height={box.h + 8 + i * 9}
                    rx={font * 0.6}
                  />
                  <g className="probe-tag" transform={`translate(${box.cx + box.w / 2 - font * 0.2 + i * font * 1.25}, ${box.cy - box.h / 2 - font * 0.55})`}>
                    <rect x={-font * 0.55} y={-font * 0.62} width={font * 1.1} height={font * 1.15} rx={2} />
                    <text textAnchor="middle" y={font * 0.32} fontSize={font * 0.82}>
                      {host}
                    </text>
                  </g>
                </g>
              ))}
            </g>
          );
        })}
      </g>

      <g className="fsm-hot">
        {hot.map(({ probe, record, edge, geometry }) => (
          <g key={`${probe.host}-${record.seq}`} style={{ color: hostColor(probe.host) }}>
            <path d={geometry.d} className="edge hot" markerEnd={arrow(probe.host === "B" ? "B" : "A")} filter={`url(#${uid}-glow)`} />
            <circle r={font * 0.36} className="particle" filter={`url(#${uid}-glow)`}>
              <animateMotion dur="0.9s" fill="freeze" path={geometry.d} />
            </circle>
            <BookLabel at={geometry.label} anchor={geometry.anchor} event={edge.label.event} action={edge.label.action} font={edgeFont * 1.06} className="edge-label hot" color={hostColor(probe.host)} />
          </g>
        ))}
      </g>
    </svg>
  );
}
