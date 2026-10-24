# Bancada — analisador de FSMs de transporte (UDP × TCP)

**Vídeo explicação:** https://youtu.be/fmuEUq56PDY

Simulador desktop que **executa, visualiza e testa** as máquinas de estados do UDP e do TCP (capítulo 3 de Kurose & Ross) sobre um canal com falhas configuráveis. Ele verifica automaticamente se cada FSM se comporta como o livro prevê. Trabalho Prático 2 de Redes Convergentes (Unifor).

- **Núcleo em Go puro**, determinístico e sem relógio real. Roda sozinho em `go test` e numa CLI.
- **FSMs 100% dirigidas por JSON**: TCP com os 11 estados (figs. 3.41/3.42 e diagrama completo da RFC 793), controle de congestionamento Reno (fig. 3.51), UDP de estado único, rdt 2.1 e rdt 3.0.
- **GUI Wails v2 + React + SVG desenhado à mão**, com cara de instrumento de bancada: diagrama de estados animado, diagrama de sequência no estilo de analisador lógico, rack de knobs para o canal, transporte de DAW com passo para trás, log filtrável e modo lado a lado.
- **14 cenários declarados como dados** (os 10 do enunciado e 4 extras). O mesmo runner atende a CLI, o `go test` e o botão "suíte" da GUI.

```
go test ./...            todos os testes (unidade, propriedade, cenários, sessão concorrente)
go run ./cmd/scenarios   tabela APROVADO/REPROVADO + report/scenarios.{json,md}
wails dev                GUI com hot reload
```

---

## Sumário

1. [Pré-requisitos](#pré-requisitos)
2. [Instalação e execução](#instalação-e-execução)
3. [Usando a interface](#usando-a-interface)
4. [Cenários e testes automatizados](#cenários-e-testes-automatizados)
5. [Decisões de arquitetura (e por que fogem do convencional)](#decisões-de-arquitetura)
6. [Formato das FSMs em JSON](#formato-das-fsms-em-json)
7. [Como adicionar uma nova FSM editando só o JSON](#como-adicionar-uma-nova-fsm-editando-só-o-json)
8. [DSL de cenários](#dsl-de-cenários)
9. [Interface entre o núcleo Go e o front](#interface-entre-o-núcleo-go-e-o-front)
10. [Estrutura de pastas](#estrutura-de-pastas)
11. [Simplificações conscientes](#simplificações-conscientes)

---

## Pré-requisitos

| Ferramenta | Versão usada | Observação |
|---|---|---|
| Go | 1.27 (mínimo exigido pelo `go.mod`) | https://go.dev/dl |
| Node.js | 24 (o Vite 8 exige ≥ 20.19) | inclui o `npm` |
| Wails CLI | v2.17.0 | `go install github.com/wailsapp/wails/v2/cmd/wails@latest` |

O binário `wails` fica em `$(go env GOPATH)/bin`, que precisa estar no `PATH`. No Windows o instalador do Go já faz isso: `%USERPROFILE%\go\bin`.

Dependências de sistema da GUI (o núcleo, os testes e a CLI não precisam delas):

| SO | Dependências | Comando |
|---|---|---|
| Windows 10/11 | WebView2 Runtime (já vem no Windows 11) | — |
| macOS | Xcode Command Line Tools | `xcode-select --install` |
| Ubuntu/Debian | gcc, pkg-config, GTK 3, WebKitGTK | `sudo apt install build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev` (em distros com webkit 4.1, use `wails dev -tags webkit2_41`) |
| Fedora | gcc, GTK 3, WebKitGTK | `sudo dnf install gcc pkgconf-pkg-config gtk3-devel webkit2gtk4.1-devel` |

`wails doctor` confere tudo de uma vez.

**Detector de corrida (`go test -race`).** Ele exige cgo e, portanto, um compilador C. Linux e macOS já têm. No Windows, instale o MinGW-w64, por exemplo pelo MSYS2 (`pacman -S mingw-w64-ucrt-x86_64-gcc`) ou pelo WinLibs (`winget install BrechtSanders.WinLibs.POSIX.UCRT`). Depois rode:

```
set CGO_ENABLED=1
go test -race ./internal/session/ ./internal/...
```

## Instalação e execução

```bash
git clone <este repositório> bancada && cd bancada

go test ./...                 # não precisa de Node nem de Wails
go run ./cmd/scenarios        # idem

wails dev                     # instala o front (npm install), sobe o Vite e abre a janela
wails build                   # gera build/bin/bancada(.exe) com o front embutido
```

O `go test ./...` funciona num clone limpo, antes de qualquer build do front, porque `frontend/dist/.gitkeep` está versionado. É ele que satisfaz o `//go:embed all:frontend/dist` do `main.go`, e um plugin do `vite.config.ts` recria o arquivo depois de cada build.

Durante o `wails dev` o app também fica disponível em `http://localhost:34115` para usar no navegador, com os bindings Go ativos. O HMR do Vite vai direto para a porta 5173 (`server.hmr` no `vite.config.ts`). Assim, a proteção contra DNS rebinding do dev server do Wails 2.17 não rejeita o websocket da janela desktop, e não é preciso configurar `WAILS_DEV_ALLOWED_HOSTS`.

Opções da CLI:

```
go run ./cmd/scenarios -only 06-fast-retransmit,10-udp-vs-tcp
go run ./cmd/scenarios -trace 03-synack-loss           # log completo de um cenário
go run ./cmd/scenarios -trace 06-fast-retransmit:sem-fr
go run ./cmd/scenarios -fsm ./fsm -scenarios ./scenarios -out report
```

O código de saída é 1 se algum cenário reprovar, para integrar em CI.

## Usando a interface

| Área | O que faz |
|---|---|
| **Barra superior** | escolhe um cenário pronto ou a bancada livre; protocolo (UDP, TCP, TCP sem TIME_WAIT, rdt 2.1, rdt 3.0); **lado a lado** (duas faixas sincronizadas, mesma seed e mesmo canal); seed; **⟳ FSMs** (relê `fsm/` e `scenarios/` do disco sem recompilar); **suíte** (roda os 14 cenários e mostra APROVADO/REPROVADO) |
| **Diagrama de estados** | SVG desenhado a partir do JSON. Estado atual pulsando, com uma sonda por host (A verde, B ciano). A última transição é percorrida por uma partícula e mostra o rótulo no formato do livro: **evento acima da linha, ações abaixo**. Abas por máquina (conexão TCP, congestionamento, remetente e receptor rdt…). Embaixo, a leitura da última transição de cada host |
| **Diagrama de sequência** | trilhas A e B; setas inclinadas pelo atraso real; perdas terminam num **×** no meio do canal; retransmissões em **âmbar**; duplicatas com **traço duplo**; corrompidos **serrilhados** em magenta; reordenados tracejados. Temporizadores aparecem como barras ao lado da trilha (RTO âmbar, 2·MSL violeta). Tempo ocioso longo é **comprimido** com marca de quebra ("≈ +1,95 s ocioso"). Clicar numa seta leva a simulação ao instante do envio |
| **Canal simulado** | rack de knobs (arraste na vertical, roda do mouse, setas do teclado; duplo clique restaura): perda, corrupção, bits trocados, duplicação, reordenação, atraso mín./máx. e atraso de reordenação. **Descartar próximo** (de qualquer host, de A ou de B) para testes determinísticos. A unidade TCP ajusta MSL e liga ou desliga a retransmissão rápida (reinicia a sessão) |
| **Aplicação** | `listen()`, `connect()`, `send ×N`, `close()` por host e "transferir ×N", que instala a carga típica de cada protocolo |
| **Registradores / comparar** | variáveis com nomes do livro (SendBase, NextSeqNum, EstimatedRTT, DevRTT, TimeoutInterval, cwnd, ssthresh, dupACKcount, rwnd…), temporizadores com contagem regressiva e a tabela comparativa entre as faixas |
| **Transporte** | ⏮ reset, ◀\| passo para trás, ▶/❚❚, \|▶ passo, velocidade de 0,25× a 64× (1× = 1 ms virtual em 20 ms reais) e scrubber com a densidade de eventos de cada faixa |
| **Log** | tabela densa e virtualizada (tempo, host, máquina, estado anterior, evento [guarda], ações, novo estado). Filtra por host, texto e tipo (só transições ou tudo). Clicar numa linha leva a simulação àquele passo. Eventos rejeitados aparecem em vermelho com o motivo |

Atalhos: **espaço** tocar/pausar · **→** passo · **←** passo para trás · **D** descartar o próximo · **R** reset · **Home/End** início e fim.

Aplicar um comando no passado (mexer num knob depois de voltar alguns passos, por exemplo) **reescreve o futuro**: a sessão refaz a simulação até aquele passo, aplica o comando e segue daí. É o comportamento esperado de event sourcing.

## Cenários e testes automatizados

| # | Protocolo | Cenário | O que é verificado |
|---|---|---|---|
| 1 | TCP | Handshake sem falhas | A: CLOSED→SYN_SENT→ESTABLISHED (exato); B: LISTEN→SYN_RCVD→ESTABLISHED |
| 2 | TCP | Perda do SYN | 1 timeout de RTO, SYN reenviado depois do timeout, nenhuma amostra de RTT do SYN retransmitido (Karn) |
| 3 | TCP | Perda do SYN-ACK | cliente reenvia o SYN; servidor, em SYN_RCVD, recebe o SYN repetido e reenvia o SYN-ACK |
| 4 | TCP | Encerramento normal | FIN_WAIT_1→FIN_WAIT_2→TIME_WAIT→CLOSED e CLOSE_WAIT→LAST_ACK→CLOSED; TIME_WAIT dura exatamente 2·MSL |
| 5 | TCP | Fechamento simultâneo | ambos FIN_WAIT_1→CLOSING→TIME_WAIT; ninguém passa por FIN_WAIT_2 |
| 6 | TCP | 1 segmento perdido em 10 | ≥3 ACKs duplicados, 1 retransmissão rápida, 0 timeouts, economia medida; a execução contrafactual `sem-fr` (guarda desligada) precisa de timeout e termina depois |
| 7 | TCP | Porta fechada | RST,ACK de um TCB transitório em CLOSED; A vai de SYN_SENT a CLOSED e a aplicação é avisada |
| 8 | UDP | 20 datagramas, 30% de perda | 20 enviados, 0 retransmissões, mensagens perdidas = perdas no canal, B nunca envia nada |
| 8b | UDP | Duplicação e reordenação | duplicatas e fora de ordem chegam à aplicação sem correção |
| 9 | UDP | Checksum corrompido | descarte silencioso por checksum, 4 de 5 entregues, nenhuma notificação ao remetente |
| 10 | UDP × TCP | Mesma carga e mesmo canal ruidoso | comparação de entregues, perdidos, duplicados, em ordem e tempo total |
| 11 | TCP × TCP sem TIME_WAIT | Último ACK perdido | com TIME_WAIT, A reconfirma o FIN repetido; sem ele, A responde RST e B aborta (pergunta 1) |
| 12 | rdt 2.1 | 30% de corrupção | entrega exata via NAK e reenvio, só com JSON |
| 13 | rdt 3.0 | 20% de perda + 10% de corrupção | entrega exata via temporizador, só com JSON |

`go test ./...` também cobre:

- o motor de FSM: validação, guardas, rejeição tipada e reentrância;
- o checksum: respostas conhecidas da seção 3.3 e da RFC 1071, a **propriedade** "toda troca de 1 bit é detectada" com `pgregory.net/rapid` em UDP, TCP e rdt, o ponto cego de duas trocas compensadas e o fuzzing nativo `FuzzChecksum`;
- o canal: mesma seed gera as mesmas falhas, e mudar a perda não embaralha os atrasos;
- o TCP: aritmética de sequência modular e o estimador de RTT conferido com valores calculados à mão;
- a determinística dos 14 cenários;
- a sessão da GUI: voltar no tempo restaura o snapshot exato, um comando no passado reescreve o futuro igual a uma execução direta, e 8 goroutines chamam a API ao mesmo tempo.

```
go test -run FuzzChecksum -fuzz FuzzChecksum -fuzztime 30s ./internal/wire/
```

## Decisões de arquitetura

**1. Simulação por eventos discretos em tempo virtual.** `sim.VirtualTime` é um `int64` em microssegundos. O escalonador é um heap (`container/heap`) ordenado por (tempo, ordem de inserção), o que torna o desempate determinístico. Temporizadores são eventos agendados e cancelados no próprio heap. O núcleo não usa `time.Sleep`, `time.After` nem goroutines: os 14 cenários rodam em cerca de 10 ms de CPU (o resto do tempo de `go test` é compilação).

**2. Determinismo por fluxos aleatórios independentes.** Cada componente aleatório (perda, atraso, corrupção, duplicação, reordenação e o ISN de cada host) tem seu próprio PCG (`math/rand/v2`), semeado com a seed e um hash do nome do fluxo. O canal sorteia **todos** os valores para todo pacote, mesmo os não usados. Assim, aumentar a perda não altera o atraso que o n-ésimo pacote teria; isso tem teste.

**3. Event sourcing com snapshots imutáveis.** Todo efeito vira um `sim.Record` publicado num barramento: transição, rejeição, envio, chegada, perda, descarte, temporizador, entrega, métrica. A cada passo nasce um `testbed.Snapshot` montado por valor. A sessão da GUI guarda a linha do tempo de frames (snapshot + registros do passo) e um **diário de comandos** com o passo em que cada um foi aplicado. Voltar um passo é mover o cursor. Aplicar um comando no passado é reconstruir o mundo a partir da seed, reaplicar o diário até o cursor e descartar o futuro. As estatísticas também são uma dobra dos registros.

**4. FSMs como dados, com vocabulário tipado.** O JSON descreve `(estado, evento, guarda?) → (ações[], novo estado)`. Guardas são expressões booleanas (`in_order && !fin_acked`) sobre nomes resolvidos num `fsm.Registry[C]` genérico. Ações são `nome` ou `nome(arg)`, com a lista de argumentos permitidos declarada no registro. Ao carregar, o motor rejeita estados inexistentes, guardas ou ações não registradas, argumentos não permitidos, transições duplicadas e **transições inalcançáveis** (sem guarda, antes de outra com o mesmo par estado/evento), e reporta todos os problemas de uma vez com `errors.Join`. Um evento inválido no estado atual vira `*fsm.RejectionError`, que embrulha `ErrNotAccepted`, `ErrGuardsFailed` ou `ErrUnknownEvent`, e aparece no log com o motivo.

**5. TCP como duas FSMs cooperando, não uma struct gigante.** O host TCP roda a máquina de **conexão** (11 estados) e a de **congestionamento** (fig. 3.51: SLOW_START, CONGESTION_AVOIDANCE, FAST_RECOVERY). A retransmissão rápida é literalmente a transição `ack.dup [third_dup_ack && fast_retransmit]` da segunda máquina. O TCB (`tcp.Endpoint`) só guarda variáveis e expõe operações pequenas. Quem decide é o JSON. O único "switch" do TCP é a tabela que classifica um segmento recebido em evento (`rcv.SYN`, `rcv.SYNACK`, `rcv.FIN`…), o equivalente ao "evento acima da linha".

**6. Bytes reais no fio.** UDP, TCP e rdt são codificados em bytes com cabeçalhos reais. O checksum é a soma em complemento de 1 de 16 bits sobre pseudo-cabeçalho, cabeçalho e dados (seção 3.3), e o canal corrompe trocando **bits** de verdade. Quem descobre a corrupção é o receptor, recalculando a soma, nunca uma flag mágica.

**7. Uma única porta de entrada concorrente.** Só a camada do Wails é concorrente. `session.Controller` tem uma goroutine dona de todo o estado, que recebe pedidos por um canal e roda o relógio de reprodução (ticker de 33 ms). Os métodos ligados ao front enviam uma closure e esperam a resposta. Não há mutex no núcleo, e o teste de concorrência martela a API com 8 goroutines.

**8. Front que só renderiza.** O React recebe snapshots e registros, desenha e manda comandos. Nenhuma regra de protocolo mora no TypeScript. Os bindings não usam o `wailsjs` gerado, cheio de comentários: uma ponte tipada à mão (`frontend/src/bridge.ts`) chama `window.go.app.Bench.*` e `window.runtime.EventsOn`. As posições dos estados, curvas e rótulos vêm do próprio JSON da FSM.

**9. Contrafactuais como cidadãos de primeira classe.** Um cenário pode ter várias execuções (`runs`) com a mesma seed e o mesmo canal. Assim se medem coisas como "quanto a retransmissão rápida economizou" (cenário 6, com e sem a guarda) e "o que acontece sem TIME_WAIT" (cenário 11, com outra definição JSON). A GUI mostra as execuções lado a lado.

## Formato das FSMs em JSON

Cada arquivo em `fsm/` descreve um protocolo e suas máquinas:

```json
{
  "protocol": "tcp",
  "family": "tcp",
  "title": "TCP",
  "source": "Kurose & Ross, figuras 3.41 e 3.42",
  "uses": ["tcp-congestion"],
  "machines": [{
    "name": "connection",
    "initial": "CLOSED",
    "width": 1120, "height": 840,
    "states": [ { "id": "FIN_WAIT_1", "x": 350, "y": 540, "loop": "left" } ],
    "transitions": [
      {
        "from": "FIN_WAIT_1", "event": "rcv.FIN", "guard": "in_order && !fin_acked",
        "actions": ["ack.ingest", "fin.accept", "send(ACK)"],
        "to": "CLOSING",
        "label": { "event": "rcv FIN", "action": "snd ACK" },
        "labelAt": { "x": 474, "y": 592 }
      }
    ]
  }]
}
```

| Campo | Significado |
|---|---|
| `family` | qual vocabulário (registro de guardas e ações) dá vida ao arquivo: `tcp`, `udp` ou `rdt` |
| `component` | `true` para máquinas que só rodam acopladas a outra (o congestionamento TCP) |
| `uses` | componentes de que a máquina depende |
| `init` | ações executadas ao iniciar a máquina (ex.: `make_pkt(ACK1)` no receptor do rdt 3.0) |
| `from`, `event` | aceitam string ou lista; listas viram uma aresta por combinação |
| `guard` | expressão com `&&`, `\|\|`, `!` e parênteses sobre guardas registradas |
| `actions` | lista ordenada de `nome` ou `nome(arg)` |
| `label` | texto do livro: evento acima da linha, ação abaixo (`\n` quebra linha) |
| `x`, `y`, `loop`, `bend`, `via`, `labelAt`, `quiet` | layout do SVG: posição, lado do auto-laço, curvatura, rota com cotovelos, posição do rótulo e arestas raras desenhadas só quando disparam |

Vocabulário disponível por família:

| Família | Guardas | Ações |
|---|---|---|
| `tcp` | `ack_ok`, `fin_acked`, `in_order`, `passive`, `third_dup_ack`, `fast_retransmit`, `reaches_ssthresh` | `isn.choose`, `passive.open`, `syn.accept`, `ack.ingest`, `data.enqueue`, `data.accept`, `fin.accept`, `tcb.release`, `send(SYN\|SYN\|ACK\|ACK\|FIN\|RST)`, `retransmit(timeout\|fast\|duplicate)`, `transmit(new)`, `timer.start(2msl)`, `timer.restart(2msl)`, `cc(timeout)`, `app.notify(...)`, `dupacks.inc`, `dupacks.reset`, `cwnd.add(MSS\|MSS*MSS/cwnd)`, `cwnd.set(1MSS\|ssthresh\|ssthresh+3MSS)`, `ssthresh.set(cwnd/2)` |
| `udp` | `intact`, `bound` | `make_datagram`, `udt_send`, `deliver_data`, `discard(checksum\|port)` |
| `rdt` | `corrupt`, `notcorrupt`, `isACK`, `isNAK`, `isACK0`, `isACK1`, `has_seq0`, `has_seq1` | `make_pkt(0\|1\|ACK\|NAK\|ACK0\|ACK1)`, `udt_send`, `extract`, `deliver_data`, `start_timer`, `stop_timer` |

## Como adicionar uma nova FSM editando só o JSON

Exemplo: criar o **rdt 2.2** (ACK numerado, sem NAK) sem tocar em Go.

1. Copie `fsm/rdt-2.1.json` para `fsm/rdt-2.2.json` e troque `"protocol"` para `"rdt-2.2"`.
2. No remetente, troque as guardas `corrupt || isNAK` por `corrupt || isACK1` (em `WAIT_ACK_0`) e `notcorrupt && isACK` por `notcorrupt && isACK0`. Faça o simétrico em `WAIT_ACK_1`.
3. No receptor, use `make_pkt(ACK0)`/`make_pkt(ACK1)` no lugar de `ACK`/`NAK`, reenvie `udt_send` para corrompidos ou duplicados e adicione `"init": ["make_pkt(ACK1)"]`. O `rdt-3.0.json` já usa exatamente esse receptor.
4. Na GUI, clique em **⟳ FSMs**. O novo protocolo aparece no seletor, com diagrama, e roda sobre o mesmo canal. Para um teste automatizado, crie `scenarios/14-rdt22.json` com `"protocol": "rdt-2.2"`.

Se o JSON tiver erro (estado inexistente, ação fora do vocabulário, transição inalcançável…), o carregamento falha com uma mensagem que aponta arquivo, máquina e número da transição. Na GUI, a mensagem aparece como alerta e as definições embutidas continuam em uso.

O `rdt-3.0.json` e o `tcp-sem-time-wait.json` foram criados assim. A variante do TCP é a mesma máquina sem o estado TIME_WAIT, e é ela que o cenário 11 usa para responder à pergunta 1.

Na GUI e na CLI, `fsm/` e `scenarios/` são lidos do disco quando existem no diretório corrente (ou em `BANCADA_FSM_DIR` / `BANCADA_SCENARIOS_DIR`). Caso contrário, valem as cópias embutidas no binário com `//go:embed`.

## DSL de cenários

```json
{
  "id": "06-fast-retransmit",
  "number": "6",
  "title": "Perda de um segmento de dados no meio de 10",
  "seed": 6,
  "channel": { "delayMin": "45ms", "delayMax": "55ms" },
  "runs": [
    { "name": "com-fr", "protocol": "tcp" },
    { "name": "sem-fr", "protocol": "tcp", "tcp": { "fastRetransmit": false } }
  ],
  "workload": { "messages": 10, "size": 1000 },
  "faults": [ { "action": "drop", "match": { "from": "A", "data": true }, "nth": 5 } ],
  "expect": [
    { "run": "com-fr", "count": { "kind": "transition", "host": "A", "machine": "congestion", "event": "ack.dup" }, "atLeast": 3 },
    { "run": "com-fr", "count": { "kind": "timeout", "host": "A", "timer": "rto" }, "eq": 0 },
    { "run": "com-fr", "metric": { "name": "totalTimeMs", "below": "sem-fr:totalTimeMs" } }
  ]
}
```

| Bloco | Conteúdo |
|---|---|
| `protocol` ou `runs` | uma execução ou várias com a mesma seed e o mesmo canal (comparações e contrafactuais) |
| `channel`, `tcp`, `udp`, `rdt` | sobrescrevem os padrões; durações aceitam `"40ms"`, `"1.5s"` ou microssegundos |
| `workload` | carga típica do protocolo (TCP: listen, connect, send ao estabelecer, close ao drenar; UDP: rajada espaçada; rdt: envio pela FSM) |
| `script` | passos `{ "at": "10ms" }` ou `{ "on": "B:CLOSE_WAIT", "after": "30ms" }` (também `"A:drained"`) com `host`, `do` (`listen`, `connect`, `send`, `close`), `count`, `size`, `interval`, `port` |
| `faults` | `drop`, `corrupt` ou `duplicate` do n-ésimo segmento que casar com `match` (`from`, `flags`, `exactFlags`, `data`, `retransmission`, `message`) |
| `expect` | `visits` (subcaminho contíguo ou `exact`), `count` de registros com `eq`/`atLeast`/`atMost`, `metric` (`eq`, `min`, `max`, `gt`, `lt`, `sameAs`, `above`, `below`, inclusive contra outra execução: `"sem-fr:totalTimeMs"`), `order` (um registro antes de outro) e `dwell` (tempo de permanência num estado) |

## Interface entre o núcleo Go e o front

Métodos expostos pelo Wails (`app.Bench`):

| Método | Efeito |
|---|---|
| `Catalog()` | protocolos, máquinas com layout, cenários e padrões |
| `Load(req)` / `Reset()` | abre um cenário ou a bancada livre (1 ou 2 faixas) e recomeça |
| `Timeline()` | todos os frames de todas as faixas (ressincronização) |
| `Play()`, `Pause()`, `StepForward()`, `StepBack()` | transporte |
| `SeekFrame(lane, frame)`, `SeekTime(µs)`, `SetSpeed(x)` | navegação |
| `SetChannel(params)`, `DropNext(from)` | canal (registrados no diário) |
| `AppCommand(host, verb, n)`, `Transfer(n, size)` | aplicação (agendados no tempo de exibição) |
| `RunSuite()` | roda os cenários e devolve APROVADO/REPROVADO |
| `ReloadDefinitions()` | relê `fsm/` e `scenarios/` do disco |

Eventos emitidos com `runtime.EventsEmit`:

| Tópico | Conteúdo |
|---|---|
| `bench:session` | `Info` da sessão (faixas, seed, canal, revisão) |
| `bench:patch` | `{lane, base, frames, revision}`: a faixa passa a ser `frames[:base] + frames`, o que cobre tanto o crescimento quanto o futuro reescrito |
| `bench:clock` | tempo virtual de exibição, cursores, fronteiras, tocando/pausado, velocidade, fim |
| `bench:fault` | mensagem de erro |

## Estrutura de pastas

```
main.go                 entrada Wails (embute frontend/dist)
wails.json              configuração do Wails
app/                    bindings: Bench (API), Runtime (ciclo de vida), carga das definições
cmd/scenarios/          CLI dos cenários
fsm/                    definições JSON das FSMs (+ embed)
scenarios/              cenários JSON (+ embed)
internal/sim            VirtualTime, escalonador, barramento de registros, fluxos aleatórios
internal/fsm            motor genérico: parse, validação, registro tipado, guardas, máquinas
internal/wire           checksum em complemento de 1; UDP, TCP e rdt em bytes
internal/channel        perda, atraso, corrupção, duplicação, reordenação, descarte manual, regras
internal/host           contrato entre testbed e pilhas (Env, Stack, views)
internal/udp            pilha UDP (contexto + vocabulário)
internal/tcp            pilha TCP: TCB, RTT, Karn, retransmissões, vocabulário das duas máquinas
internal/rdt            família rdt (2.x/3.0) dirigida por JSON
internal/workload       mensagens numeradas, remontagem e veredito (nova/duplicada/fora de ordem)
internal/testbed        monta A–canal–B, Step(), Snapshot, gatilhos, estatísticas
internal/scenario       DSL, runner, asserções, relatórios
internal/session        controlador da GUI: frames, cursor, diário, replay, lado a lado
frontend/               React + TypeScript estrito + SVG (Vite)
docs/                   relatório, roteiro do vídeo, capturas
report/                 saída de go run ./cmd/scenarios
```

## Simplificações conscientes

- Um par de hosts (A cliente, B servidor) e uma conexão por host. Portas sem socket são atendidas por um TCB transitório em CLOSED.
- O receptor TCP confirma cada segmento na hora (sem ACK atrasado) e guarda segmentos fora de ordem.
- A detecção de ACK duplicado segue a RFC 5681, exceto a exigência de janela inalterada, porque a rwnd anunciada encolhe com dados fora de ordem guardados.
- O estimador de RTT segue a RFC 6298: DevRTT é atualizado com o EstimatedRTT anterior e depois o EstimatedRTT, com α=0,125 e β=0,25. Há uma amostra por vez, e segmentos retransmitidos não são amostrados (Karn). TimeoutInterval é limitado a [200 ms, 60 s] e dobra a cada expiração, voltando ao valor estimado no próximo ACK novo (seção 3.5.4).
- `ssthresh = max(cwnd/2, 2·MSS)`. Depois de uma retransmissão (rápida ou por SYN repetido) o RTO é reiniciado, como no BSD.
- O UDP exige checksum (como no IPv6). Por isso, um datagrama com checksum zerado também é descartado.
- Sem sockets reais: o canal é em memória, como o enunciado permite.
