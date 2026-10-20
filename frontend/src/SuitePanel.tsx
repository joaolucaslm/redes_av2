import { protocolLabel, timeShort } from "./format";
import type { ScenarioResult } from "./model";
import { act, getState, load, setState, useStore } from "./store";

async function runSuite() {
  setState((s) => ({ ...s, suite: { ...s.suite, running: true } }));
  const results = await act((api) => api.RunSuite());
  setState((s) => ({ ...s, suite: { ...s.suite, running: false, results: results ?? s.suite.results } }));
}

function openOnBench(result: ScenarioResult) {
  setState((s) => ({ ...s, panel: "bench" }));
  void load({ ...getState().selection, scenario: result.id, sideBySide: (result.runs?.length ?? 1) > 1 });
}

export function SuitePanel() {
  const suite = useStore((s) => s.suite);
  const results = suite.results ?? [];
  const passed = results.filter((r) => r.passed).length;

  return (
    <div className="overlay" onClick={() => setState((s) => ({ ...s, panel: "bench" }))}>
      <section className="panel suite" onClick={(e) => e.stopPropagation()}>
        <header className="panel-head">
          <span className="label">Suíte de testes automatizados · mesmos cenários do go test e da CLI</span>
          <span className="rule" />
          {results.length > 0 && (
            <span className={passed === results.length ? "stamp ok" : "stamp fail"}>
              {passed}/{results.length} aprovados
            </span>
          )}
          <button className="key on" onClick={() => void runSuite()} disabled={suite.running}>
            {suite.running ? "executando…" : "executar suíte"}
          </button>
          <button className="key" onClick={() => setState((s) => ({ ...s, panel: "bench" }))}>
            fechar
          </button>
        </header>
        <div className="panel-body scroll suite-body">
          {results.length === 0 ? (
            <div className="empty">execute a suíte para ver aprovado ou reprovado por cenário</div>
          ) : (
            <table className="suite-table">
              <thead>
                <tr>
                  <th>#</th>
                  <th>protocolo</th>
                  <th>cenário</th>
                  <th>resultado</th>
                  <th>verificações</th>
                  <th>seed</th>
                  <th>tempo virtual</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {results.map((r) => {
                  const checks = r.checks ?? [];
                  const open = suite.open === r.id;
                  const span = Math.max(0, ...(r.runs ?? []).map((run) => run.end));
                  return [
                    <tr key={r.id} className={open ? "open" : ""} onClick={() => setState((s) => ({ ...s, suite: { ...s.suite, open: open ? null : r.id } }))}>
                      <td className="num">{r.number}</td>
                      <td>{[...new Set(r.protocols.map(protocolLabel))].join(" vs ")}</td>
                      <td>{r.title}</td>
                      <td>
                        <span className={r.passed ? "stamp ok" : "stamp fail"}>{r.passed ? "APROVADO" : "REPROVADO"}</span>
                      </td>
                      <td>
                        {checks.filter((c) => c.passed).length}/{checks.length}
                      </td>
                      <td>{r.seed}</td>
                      <td>{timeShort(span)}</td>
                      <td>
                        <button
                          className="key"
                          onClick={(e) => {
                            e.stopPropagation();
                            openOnBench(r);
                          }}
                        >
                          abrir na bancada
                        </button>
                      </td>
                    </tr>,
                    open ? (
                      <tr key={`${r.id}-detail`} className="detail">
                        <td />
                        <td colSpan={7}>
                          {r.expected && (
                            <p className="dim">
                              <span className="label">esperado</span> {r.expected}
                            </p>
                          )}
                          {r.error && <p className="reject-reason">{r.error}</p>}
                          <ul className="checks">
                            {checks.map((c, i) => (
                              <li key={i} className={c.passed ? "ok" : "fail"}>
                                <span>{c.passed ? "✔" : "✘"}</span> {c.title}
                                {c.run ? <span className="label"> {c.run}</span> : null}
                                <span className="dim"> — {c.detail}</span>
                              </li>
                            ))}
                          </ul>
                          {(r.runs ?? []).map((run) => (
                            <div key={run.name} className="paths">
                              {run.paths.map((p) => (
                                <div key={`${run.name}-${p.host}-${p.machine}`}>
                                  <span className="label">
                                    {run.name} · {p.host}/{p.machine}
                                  </span>{" "}
                                  {p.states.join(" → ")}
                                </div>
                              ))}
                            </div>
                          ))}
                        </td>
                      </tr>
                    ) : null,
                  ];
                })}
              </tbody>
            </table>
          )}
        </div>
      </section>
    </div>
  );
}
