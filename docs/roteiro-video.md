# Roteiro do vídeo (até 5 minutos)

**Preparação, antes de gravar:**

- `wails dev` na raiz do projeto, com a janela maximizada.
- Um terminal pronto em segundo plano.
- Velocidade inicial 1×. O campo "seed" não precisa ser tocado: cada cenário traz a sua.

Os tempos de tela entre colchetes são aproximados. As falas são sugestões.

---

### 0:00–0:25 · Apresentação

1. Mostre a janela inteira, que abre no cenário **1 · Handshake sem falhas**.
2. Fala: "Esta é a Bancada. À esquerda está o diagrama de estados desenhado a partir do JSON da FSM, no centro o diagrama de sequência como num analisador lógico, à direita o canal simulado e embaixo o transporte e o log. Tudo roda num núcleo Go determinístico, em tempo virtual."

### 0:25–1:05 · Cenário 1: handshake

1. Clique em **▶** (ou espaço).
2. Aponte para o diagrama de estados: as sondas **A** (verde) e **B** (ciano) pulsam, a partícula percorre `CLOSED → SYN_SENT` e o rótulo aparece no formato do livro ("connect()" acima da linha, "snd SYN" abaixo).
3. No diagrama de sequência, mostre `SYN seq=0`, `SYNACK seq=0 ACK=1` e `ACK=1`, com as etiquetas de estado nas trilhas.
4. Pressione **←** duas vezes (passo para trás) e **→** duas vezes. Fala: "Cada passo é um snapshot imutável; voltar é só mover o cursor."

### 1:05–1:50 · Cenário 3: perda do SYN-ACK

1. No seletor **cenário**, escolha **3 · Perda do SYN-ACK** (seed 3).
2. Velocidade **2×**, depois **▶**.
3. Mostre o SYN-ACK terminando no **×** vermelho no meio do canal e a quebra **"≈ +927 ms ocioso"** (o tempo ocioso é comprimido).
4. Quando o RTO de A expirar (barra âmbar e "⏱ RTO"), mostre o **SYN reenviado em âmbar** e, no diagrama, o auto-laço de B em SYN_RCVD: "SYN repetido / reenvia SYN,ACK".
5. No **log**, clique na linha `1.050,282 ms · B · SYN_RCVD · rcv.SYN`: a simulação volta exatamente para aquele instante.

### 1:50–2:50 · Cenário 6: retransmissão rápida

1. Escolha **6 · Perda de um segmento de dados no meio de 10** (seed 6) e clique na aba **CONGESTIONAMENTO** do diagrama.
2. Velocidade **1×**, **▶**. Pause (espaço) quando A entrar em **FAST_RECOVERY** (cerca de 507 ms).
3. Mostre no diagrama a transição `dupACKcount == 3 / ssthresh = cwnd/2; cwnd = ssthresh + 3·MSS; retransmite segmento perdido`. No trace, mostre os três `ACK=4001` e a retransmissão `↻ seq=4001` em âmbar.
4. No painel **registradores**, mostre `cwnd`, `ssthresh`, `dupACKcount` e `TimeoutInterval`, e o temporizador RTO com a contagem regressiva.
5. Ative **lado a lado**: aparecem as execuções `com-fr` e `sem-fr`. **▶** a 4× e abra a aba **comparar**. Fala: "Com a mesma seed e o mesmo canal, a retransmissão rápida chegou 103,7 ms antes do timeout e terminou a transferência 205 ms mais cedo."

### 2:50–3:20 · Cenário 5: fechamento simultâneo

1. Desative **lado a lado** e escolha **5 · Fechamento simultâneo** (seed 5). **▶** a 2×.
2. Mostre os FINs se cruzando em X, os dois lados em **CLOSING**, os ACKs cruzados e os dois em **TIME_WAIT**. Na trilha, a barra violeta mostra o 2·MSL.

### 3:20–3:55 · Bancada livre e falhas manuais

1. Clique em **TCP** na barra de protocolos (bancada livre). Digite **42** no campo seed e tecle Enter.
2. Clique em **transferir ×10** no painel Aplicação e depois **▶**.
3. Assim que os primeiros dados cruzarem o canal, pressione **D** (descartar o próximo): o próximo segmento termina num ×.
4. Gire o knob **perda** até 20%. Fala: "O comando entra no diário; se eu voltar no tempo e mexer de novo, o futuro é reescrito de forma determinística."

### 3:55–4:25 · UDP × TCP (cenário 10)

1. Escolha **10 · Mesma carga, mesmo canal ruidoso** e ative **lado a lado**. Velocidade **8×**, **▶**.
2. À esquerda, o UDP: perdas (×), corrompidos serrilhados descartados em B e duplicatas em traço duplo. À direita, o TCP: retransmissões em âmbar.
3. Abra a aba **comparar**: UDP com 25 de 30 entregues, 2 duplicadas e 3 fora de ordem em 362 ms; TCP com 30 de 30, em ordem, em 2,29 s com 46 segmentos.

### 4:25–5:00 · Extensibilidade e testes

1. No editor, mostre `fsm/rdt-3.0.json` (sem nenhuma linha de Go). Na GUI, clique em **RDT 3.0** e **▶** a 4×.
2. Clique em **suíte** e em **executar suíte**: **14/14 aprovados**. Expanda o cenário 6 para mostrar as verificações.
3. No terminal, rode `go run ./cmd/scenarios` e mostre a tabela APROVADO/REPROVADO e o `report/scenarios.md` gerado.
4. Encerramento: "FSMs como dados, tempo virtual determinístico e a mesma suíte na CLI, no go test e na GUI."
