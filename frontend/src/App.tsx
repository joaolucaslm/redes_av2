import { useEffect } from "react";
import { ChannelRack } from "./controls/ChannelRack";
import { HostConsole } from "./controls/HostConsole";
import { Inspector } from "./controls/Inspector";
import { DiagramPanel } from "./diagram/DiagramPanel";
import { LogTable } from "./log/LogTable";
import { boot, useStore } from "./store";
import { SuitePanel } from "./SuitePanel";
import { TopBar } from "./TopBar";
import { SequenceTrace } from "./trace/SequenceTrace";
import { transport } from "./transport/actions";
import { Transport } from "./transport/Transport";

function useShortcuts() {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement | null;
      if (target && (target.tagName === "INPUT" || target.tagName === "SELECT" || target.tagName === "TEXTAREA")) {
        return;
      }
      const actions: Record<string, () => unknown> = {
        " ": transport.toggle,
        ArrowRight: transport.step,
        ArrowLeft: transport.back,
        d: transport.drop,
        D: transport.drop,
        r: transport.reset,
        R: transport.reset,
        Home: transport.home,
        End: transport.end,
      };
      const action = actions[e.key];
      if (action && !e.ctrlKey && !e.metaKey && !e.altKey) {
        e.preventDefault();
        void action();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);
}

function Faults() {
  const faults = useStore((s) => s.faults);
  return (
    <div className="faults">
      {faults.map((f) => (
        <div key={f.id} className={`fault ${f.level}`}>
          <span className={f.level === "info" ? "led green on" : "led on"} /> {f.message}
        </div>
      ))}
    </div>
  );
}

function Offline() {
  return (
    <div className="offline">
      <div className="label">sem núcleo Go</div>
      <p>
        Esta interface só renderiza snapshots do núcleo. Rode <code>wails dev</code> na raiz do projeto, ou abra o endereço que o
        Wails imprime (por padrão <code>http://localhost:34115</code>) para usar o navegador com os bindings ativos.
      </p>
    </div>
  );
}

export function App() {
  const connected = useStore((s) => s.connected);
  const laneCount = useStore((s) => s.info?.lanes.length ?? 1);
  const panel = useStore((s) => s.panel);

  useEffect(() => {
    void boot();
  }, []);
  useShortcuts();

  return (
    <div className="chassis">
      <TopBar />
      {connected ? (
        <div className={laneCount > 1 ? "bay dual" : "bay"}>
          <DiagramPanel />
          {Array.from({ length: laneCount }, (_, lane) => (
            <SequenceTrace key={lane} lane={lane} />
          ))}
          <aside className="rack-column">
            <ChannelRack />
            <HostConsole />
            <Inspector />
          </aside>
        </div>
      ) : (
        <Offline />
      )}
      <Transport />
      <LogTable />
      {panel === "suite" && <SuitePanel />}
      <Faults />
    </div>
  );
}
