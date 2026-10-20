import { act, setState, useStore } from "../store";

const verbs: Record<string, Record<string, string[]>> = {
  tcp: { A: ["connect", "send", "close"], B: ["listen", "send", "close"] },
  udp: { A: ["send"], B: ["send"] },
  rdt: { A: ["send"], B: [] },
};

const verbLabel: Record<string, string> = {
  connect: "connect()",
  listen: "listen()",
  send: "send ×N",
  close: "close()",
};

export function HostConsole() {
  const info = useStore((s) => s.info);
  const burst = useStore((s) => s.burst);
  const families = [...new Set((info?.lanes ?? []).map((l) => l.family))];
  const allowed = (host: string) => [...new Set(families.flatMap((f) => verbs[f]?.[host] ?? []))];

  return (
    <section className="panel console-panel">
      <header className="panel-head">
        <span className="label">Aplicação</span>
        <span className="rule" />
        <div className="segmented">
          {[1, 5, 10, 30].map((n) => (
            <button key={n} className={burst === n ? "key on" : "key"} onClick={() => setState((s) => ({ ...s, burst: n }))}>
              {n}
            </button>
          ))}
        </div>
      </header>
      <div className="panel-body console-body">
        {["A", "B"].map((host) => (
          <div key={host} className="console-row">
            <span className={host === "A" ? "console-host host-a" : "console-host host-b"}>{host}</span>
            {["listen", "connect", "send", "close"].map((verb) =>
              allowed(host).includes(verb) ? (
                <button key={verb} className="key" onClick={() => act((api) => api.AppCommand(host, verb, burst))}>
                  {verbLabel[verb]?.replace("N", String(burst))}
                </button>
              ) : null,
            )}
          </div>
        ))}
        <button className="key transfer" onClick={() => act((api) => api.Transfer(burst, 0))} disabled={!info}>
          transferir ×{burst} (carga do protocolo)
        </button>
      </div>
    </section>
  );
}
