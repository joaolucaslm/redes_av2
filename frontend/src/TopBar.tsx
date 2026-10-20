import { useState } from "react";
import { protocolLabel } from "./format";
import { load, reloadDefinitions, setState, useStore, type Selection } from "./store";

function Logo() {
  return (
    <svg viewBox="0 0 44 28" className="logo" aria-hidden="true">
      <rect x="0.5" y="0.5" width="43" height="27" rx="3" />
      <path d="M4 18 H10 L13 7 L17 22 L20 12 H26 L29 18 H40" />
      <circle cx="40" cy="18" r="1.6" />
    </svg>
  );
}

export function TopBar() {
  const catalog = useStore((s) => s.catalog);
  const selection = useStore((s) => s.selection);
  const info = useStore((s) => s.info);
  const panel = useStore((s) => s.panel);
  const connected = useStore((s) => s.connected);
  const [seedText, setSeedText] = useState<string | null>(null);
  const protocols = catalog?.protocols ?? [];
  const scenario = catalog?.scenarios.find((s) => s.id === selection.scenario);
  const canPair = !selection.scenario || (scenario?.runs.length ?? 0) > 1;

  const choose = (patch: Partial<Selection>) => void load({ ...selection, ...patch });
  const commitSeed = () => {
    if (seedText === null) {
      return;
    }
    const seed = Math.max(0, Math.min(2 ** 32, Number.parseInt(seedText, 10) || 0));
    setSeedText(null);
    void load({ ...selection, scenario: "", seed, protocols: [info?.lanes[0]?.protocol ?? selection.protocols[0], selection.protocols[1]] });
  };

  return (
    <header className="topbar panel">
      <div className="brand">
        <Logo />
        <div>
          <div className="brand-name">BANCADA</div>
          <div className="label brand-sub">analisador de FSMs · transporte</div>
        </div>
      </div>

      <div className="topbar-group">
        <span className="label">cenário</span>
        <select className="select" value={selection.scenario} onChange={(e) => choose({ scenario: e.target.value })}>
          <option value="">— bancada livre —</option>
          {catalog?.scenarios.map((s) => (
            <option key={s.id} value={s.id}>
              {s.number.padStart(2, " ")} · {s.title}
            </option>
          ))}
        </select>
      </div>

      <div className="topbar-group">
        <span className="label">protocolo</span>
        <div className="segmented">
          {protocols.map((p) => (
            <button
              key={p.id}
              className={!selection.scenario && selection.protocols[0] === p.id ? "key on" : "key"}
              onClick={() => choose({ scenario: "", protocols: [p.id, selection.protocols[1]] })}
              title={p.summary}
            >
              {protocolLabel(p.id)}
            </button>
          ))}
        </div>
      </div>

      <div className="topbar-group">
        <button
          className={selection.sideBySide ? "key on" : "key"}
          disabled={!canPair}
          onClick={() => choose({ sideBySide: !selection.sideBySide })}
          title="Mesma seed e mesmo canal em duas faixas sincronizadas"
        >
          lado a lado
        </button>
        {selection.sideBySide && !selection.scenario && (
          <select className="select" value={selection.protocols[1]} onChange={(e) => choose({ protocols: [selection.protocols[0], e.target.value] })}>
            {protocols.map((p) => (
              <option key={p.id} value={p.id}>
                vs {protocolLabel(p.id)}
              </option>
            ))}
          </select>
        )}
      </div>

      <div className="topbar-group">
        <span className="label">seed</span>
        <input
          className="lcd seed"
          value={seedText ?? String(info?.seed ?? selection.seed)}
          onChange={(e) => setSeedText(e.target.value.replace(/\D/g, ""))}
          onBlur={commitSeed}
          onKeyDown={(e) => e.key === "Enter" && commitSeed()}
          title="Mudar a seed abre a bancada livre com o protocolo atual"
        />
      </div>

      <div className="topbar-spacer" />

      <button className="key" onClick={() => void reloadDefinitions()} title="Relê fsm/*.json e scenarios/*.json do disco, sem recompilar">
        ⟳ FSMs
      </button>
      <button className={panel === "suite" ? "key on" : "key"} onClick={() => setState((s) => ({ ...s, panel: s.panel === "suite" ? "bench" : "suite" }))}>
        suíte
      </button>
      <span className="link-status" title={connected ? "núcleo Go conectado" : "sem núcleo: abra pelo wails dev"}>
        <span className={connected ? "led green on" : "led on"} />
      </span>
    </header>
  );
}
