# Relatório automático dos cenários

Gerado por `go run ./cmd/scenarios`. Todos os tempos são virtuais. **14 de 14 cenários aprovados.**

| # | Protocolo | Cenário | Resultado | Verificações | Seed | Tempo virtual |
|---|---|---|---|---|---|---|
| 1 | TCP | Handshake sem falhas | **APROVADO** | 6/6 | 1 | 160ms |
| 2 | TCP | Perda do SYN | **APROVADO** | 8/8 | 2 | 1160ms |
| 3 | TCP | Perda do SYN-ACK | **APROVADO** | 6/6 | 3 | 1135.352ms |
| 4 | TCP | Encerramento normal | **APROVADO** | 6/6 | 4 | 1440ms |
| 5 | TCP | Fechamento simultâneo | **APROVADO** | 6/6 | 5 | 1500ms |
| 6 | TCP | Perda de um segmento de dados no meio de 10 | **APROVADO** | 9/9 | 6 | 2921.543ms |
| 7 | TCP | Conexão a uma porta fechada | **APROVADO** | 5/5 | 7 | 110ms |
| 8 | UDP | 20 datagramas com 30% de perda | **APROVADO** | 6/6 | 8 | 237.982ms |
| 8b | UDP | Duplicação e reordenação chegam à aplicação | **APROVADO** | 4/4 | 9 | 279.159ms |
| 9 | UDP | Datagrama com checksum corrompido | **APROVADO** | 5/5 | 10 | 86.97ms |
| 10 | UDP vs TCP | Mesma carga, mesmo canal ruidoso | **APROVADO** | 9/9 | 17 | 5361.75ms |
| 11 | TCP vs TCP-SEM-TIME-WAIT | Sem TIME_WAIT, o último ACK perdido vira RST | **APROVADO** | 8/8 | 11 | 1740ms |
| 12 | RDT-2.1 | rdt 2.1 sobre canal com erros de bits | **APROVADO** | 4/4 | 12 | 899.703ms |
| 13 | RDT-3.0 | rdt 3.0 sobre canal com perdas | **APROVADO** | 4/4 | 13 | 760.024ms |

## 1 · Handshake sem falhas — APROVADO

**Esperado (enunciado):** A: CLOSED→SYN_SENT→ESTABLISHED; B: LISTEN→SYN_RCVD→ESTABLISHED

Canal limpo com atraso fixo de 50 ms. B abre a porta 80 em t=0 e A conecta em t=10 ms.

| Verificação | Execução | Resultado | Detalhe |
|---|---|---|---|
| A percorre exatamente CLOSED→SYN_SENT→ESTABLISHED |  | ✔ | observado: CLOSED→SYN_SENT→ESTABLISHED |
| B visita LISTEN→SYN_RCVD→ESTABLISHED |  | ✔ | observado: CLOSED→LISTEN→SYN_RCVD→ESTABLISHED |
| um SYN de A e um SYN-ACK de B |  | ✔ | observado: 2 |
| A fecha o handshake com um ACK puro |  | ✔ | observado: 1 |
| retransmissions = 0 |  | ✔ | observado: 0 |
| rejected = 0 |  | ✔ | observado: 0 |

**Caminhos de estados:**

- `A/connection`: CLOSED → SYN_SENT → ESTABLISHED
- `A/congestion`: SLOW_START
- `B/connection`: CLOSED → LISTEN → SYN_RCVD → ESTABLISHED
- `B/congestion`: SLOW_START

| Métrica | tcp |
|---|---|
| mensagens oferecidas | 0 |
| entregues (únicas) | 0 |
| perdidas para a aplicação | 0 |
| duplicadas entregues à aplicação | 0 |
| entregues em ordem | 0 |
| entregues fora de ordem | 0 |
| segmentos enviados | 3 |
| retransmissões | 0 |
| retransmissões rápidas | 0 |
| timeouts | 0 |
| perdidos no canal | 0 |
| corrompidos no canal | 0 |
| duplicados pelo canal | 0 |
| descartados pelo receptor | 0 |
| bytes no fio | 60 |
| tempo de transferência: 1ª oferta → última entrega (ms) | 0 |
| tempo total desde t=0, com handshake (ms) | 0 |
| economia da retransmissão rápida (ms) | 0 |

## 2 · Perda do SYN — APROVADO

**Esperado (enunciado):** Timeout e retransmissão do SYN

Uma regra do canal descarta o primeiro SYN de A. O temporizador de retransmissão (1 s inicial) expira e A reenvia o SYN com o mesmo número de sequência.

| Verificação | Execução | Resultado | Detalhe |
|---|---|---|---|
| o SYN original se perde no canal |  | ✔ | observado: 1 |
| o RTO de A expira uma vez |  | ✔ | observado: 1 |
| A reenvia o SYN |  | ✔ | observado: 1 |
| timeout[host=A timer=rto] antes de send[host=A flags exatas=SYN retransmissão=true] |  | ✔ | primeiro em 1010ms, segundo em 1010ms |
| #transition[host=A de=SYN_SENT evento=timeout.rto para=SYN_SENT] = 1 |  | ✔ | observado: 1 |
| A percorre exatamente CLOSED→SYN_SENT→ESTABLISHED |  | ✔ | observado: CLOSED→SYN_SENT→ESTABLISHED |
| B visita LISTEN→SYN_RCVD→ESTABLISHED |  | ✔ | observado: CLOSED→LISTEN→SYN_RCVD→ESTABLISHED |
| A não amostra RTT do SYN retransmitido (Karn) |  | ✔ | observado: 0 |

**Caminhos de estados:**

- `A/connection`: CLOSED → SYN_SENT → ESTABLISHED
- `A/congestion`: SLOW_START
- `B/connection`: CLOSED → LISTEN → SYN_RCVD → ESTABLISHED
- `B/congestion`: SLOW_START

| Métrica | tcp |
|---|---|
| mensagens oferecidas | 0 |
| entregues (únicas) | 0 |
| perdidas para a aplicação | 0 |
| duplicadas entregues à aplicação | 0 |
| entregues em ordem | 0 |
| entregues fora de ordem | 0 |
| segmentos enviados | 4 |
| retransmissões | 1 |
| retransmissões rápidas | 0 |
| timeouts | 1 |
| perdidos no canal | 1 |
| corrompidos no canal | 0 |
| duplicados pelo canal | 0 |
| descartados pelo receptor | 0 |
| bytes no fio | 80 |
| tempo de transferência: 1ª oferta → última entrega (ms) | 0 |
| tempo total desde t=0, com handshake (ms) | 0 |
| economia da retransmissão rápida (ms) | 0 |

## 3 · Perda do SYN-ACK — APROVADO

**Esperado (enunciado):** Cliente retransmite o SYN; servidor reenvia o SYN-ACK

O primeiro SYN-ACK de B se perde. A continua em SYN_SENT até o RTO expirar e reenviar o SYN; B, em SYN_RCVD, recebe o SYN repetido e reenvia o SYN-ACK.

| Verificação | Execução | Resultado | Detalhe |
|---|---|---|---|
| o SYN-ACK original se perde |  | ✔ | observado: 1 |
| cliente retransmite o SYN |  | ✔ | observado: 1 |
| servidor reenvia o SYN-ACK |  | ✔ | observado: 1 |
| B reage ao SYN repetido em SYN_RCVD |  | ✔ | observado: 1 |
| A percorre exatamente CLOSED→SYN_SENT→ESTABLISHED |  | ✔ | observado: CLOSED→SYN_SENT→ESTABLISHED |
| B visita LISTEN→SYN_RCVD→ESTABLISHED |  | ✔ | observado: CLOSED→LISTEN→SYN_RCVD→ESTABLISHED |

**Caminhos de estados:**

- `A/connection`: CLOSED → SYN_SENT → ESTABLISHED
- `A/congestion`: SLOW_START
- `B/connection`: CLOSED → LISTEN → SYN_RCVD → ESTABLISHED
- `B/congestion`: SLOW_START

| Métrica | tcp |
|---|---|
| mensagens oferecidas | 0 |
| entregues (únicas) | 0 |
| perdidas para a aplicação | 0 |
| duplicadas entregues à aplicação | 0 |
| entregues em ordem | 0 |
| entregues fora de ordem | 0 |
| segmentos enviados | 5 |
| retransmissões | 2 |
| retransmissões rápidas | 0 |
| timeouts | 1 |
| perdidos no canal | 1 |
| corrompidos no canal | 0 |
| duplicados pelo canal | 0 |
| descartados pelo receptor | 0 |
| bytes no fio | 100 |
| tempo de transferência: 1ª oferta → última entrega (ms) | 0 |
| tempo total desde t=0, com handshake (ms) | 0 |
| economia da retransmissão rápida (ms) | 0 |

## 4 · Encerramento normal — APROVADO

**Esperado (enunciado):** A passa por FIN_WAIT_1, FIN_WAIT_2 e TIME_WAIT; B por CLOSE_WAIT e LAST_ACK

Depois do handshake A fecha a conexão. B confirma o FIN, entra em CLOSE_WAIT e fecha 30 ms depois. A espera 2·MSL em TIME_WAIT (MSL = 500 ms nesta simulação).

| Verificação | Execução | Resultado | Detalhe |
|---|---|---|---|
| A visita ESTABLISHED→FIN_WAIT_1→FIN_WAIT_2→TIME_WAIT→CLOSED |  | ✔ | observado: CLOSED→SYN_SENT→ESTABLISHED→FIN_WAIT_1→FIN_WAIT_2→TIME_WAIT→CLOSED |
| B visita ESTABLISHED→CLOSE_WAIT→LAST_ACK→CLOSED |  | ✔ | observado: CLOSED→LISTEN→SYN_RCVD→ESTABLISHED→CLOSE_WAIT→LAST_ACK→CLOSED |
| TIME_WAIT dura exatamente 2·MSL |  | ✔ | observado: 1000ms (de 440ms a 1440ms) |
| o temporizador 2·MSL de A expira |  | ✔ | observado: 1 |
| um FIN de cada lado |  | ✔ | observado: 2 |
| rejected = 0 |  | ✔ | observado: 0 |

**Caminhos de estados:**

- `A/connection`: CLOSED → SYN_SENT → ESTABLISHED → FIN_WAIT_1 → FIN_WAIT_2 → TIME_WAIT → CLOSED
- `A/congestion`: SLOW_START
- `B/connection`: CLOSED → LISTEN → SYN_RCVD → ESTABLISHED → CLOSE_WAIT → LAST_ACK → CLOSED
- `B/congestion`: SLOW_START

| Métrica | tcp |
|---|---|
| mensagens oferecidas | 0 |
| entregues (únicas) | 0 |
| perdidas para a aplicação | 0 |
| duplicadas entregues à aplicação | 0 |
| entregues em ordem | 0 |
| entregues fora de ordem | 0 |
| segmentos enviados | 7 |
| retransmissões | 0 |
| retransmissões rápidas | 0 |
| timeouts | 0 |
| perdidos no canal | 0 |
| corrompidos no canal | 0 |
| duplicados pelo canal | 0 |
| descartados pelo receptor | 0 |
| bytes no fio | 140 |
| tempo de transferência: 1ª oferta → última entrega (ms) | 0 |
| tempo total desde t=0, com handshake (ms) | 0 |
| economia da retransmissão rápida (ms) | 0 |

## 5 · Fechamento simultâneo — APROVADO

**Esperado (enunciado):** Ambos passam por FIN_WAIT_1→CLOSING→TIME_WAIT

A e B chamam close() no mesmo instante virtual (t=400 ms). Os FINs se cruzam no canal: cada lado recebe um FIN que ainda não confirma o seu próprio FIN e vai para CLOSING.

| Verificação | Execução | Resultado | Detalhe |
|---|---|---|---|
| A visita FIN_WAIT_1→CLOSING→TIME_WAIT→CLOSED |  | ✔ | observado: CLOSED→SYN_SENT→ESTABLISHED→FIN_WAIT_1→CLOSING→TIME_WAIT→CLOSED |
| B visita FIN_WAIT_1→CLOSING→TIME_WAIT→CLOSED |  | ✔ | observado: CLOSED→LISTEN→SYN_RCVD→ESTABLISHED→FIN_WAIT_1→CLOSING→TIME_WAIT→CLOSED |
| ninguém passa por FIN_WAIT_2 |  | ✔ | observado: 0 |
| A permanece em TIME_WAIT = 1000ms |  | ✔ | observado: 1000ms (de 500ms a 1500ms) |
| B permanece em TIME_WAIT = 1000ms |  | ✔ | observado: 1000ms (de 500ms a 1500ms) |
| rejected = 0 |  | ✔ | observado: 0 |

**Caminhos de estados:**

- `A/connection`: CLOSED → SYN_SENT → ESTABLISHED → FIN_WAIT_1 → CLOSING → TIME_WAIT → CLOSED
- `A/congestion`: SLOW_START
- `B/connection`: CLOSED → LISTEN → SYN_RCVD → ESTABLISHED → FIN_WAIT_1 → CLOSING → TIME_WAIT → CLOSED
- `B/congestion`: SLOW_START

| Métrica | tcp |
|---|---|
| mensagens oferecidas | 0 |
| entregues (únicas) | 0 |
| perdidas para a aplicação | 0 |
| duplicadas entregues à aplicação | 0 |
| entregues em ordem | 0 |
| entregues fora de ordem | 0 |
| segmentos enviados | 7 |
| retransmissões | 0 |
| retransmissões rápidas | 0 |
| timeouts | 0 |
| perdidos no canal | 0 |
| corrompidos no canal | 0 |
| duplicados pelo canal | 0 |
| descartados pelo receptor | 0 |
| bytes no fio | 140 |
| tempo de transferência: 1ª oferta → última entrega (ms) | 0 |
| tempo total desde t=0, com handshake (ms) | 0 |
| economia da retransmissão rápida (ms) | 0 |

## 6 · Perda de um segmento de dados no meio de 10 — APROVADO

**Esperado (enunciado):** 3 ACKs duplicados e retransmissão rápida, sem esperar o timeout

A envia 10 mensagens de 1 MSS (1000 B) e o 5º segmento de dados se perde. Os segmentos seguintes geram 3 ACKs duplicados e a máquina de congestionamento dispara a retransmissão rápida. A execução sem-fr repete tudo com a guarda fast_retransmit desligada e serve de contrafactual para medir a economia de ponta a ponta.

| Verificação | Execução | Resultado | Detalhe |
|---|---|---|---|
| pelo menos 3 ACKs duplicados chegam a A | com-fr | ✔ | observado: 4 |
| exatamente uma retransmissão rápida | com-fr | ✔ | observado: 1 |
| nenhum timeout de retransmissão | com-fr | ✔ | observado: 0 |
| A visita SLOW_START→FAST_RECOVERY→CONGESTION_AVOIDANCE | com-fr | ✔ | observado: SLOW_START→FAST_RECOVERY→CONGESTION_AVOIDANCE |
| a retransmissão rápida acontece antes do prazo do RTO | com-fr | ✔ | observado: 103.684 |
| inOrder = 10 | com-fr | ✔ | observado: 10 |
| sem a guarda, só o timeout recupera a perda | sem-fr | ✔ | observado: 1 |
| inOrder = 10 | sem-fr | ✔ | observado: 10 |
| com retransmissão rápida a transferência termina antes | com-fr | ✔ | observado: 460.48 |

**Caminhos de estados:**

- `com-fr · A/connection`: CLOSED → SYN_SENT → ESTABLISHED → FIN_WAIT_1 → FIN_WAIT_2 → TIME_WAIT → CLOSED
- `com-fr · A/congestion`: SLOW_START → FAST_RECOVERY → CONGESTION_AVOIDANCE
- `com-fr · B/connection`: CLOSED → LISTEN → SYN_RCVD → ESTABLISHED → CLOSE_WAIT → LAST_ACK → CLOSED
- `com-fr · B/congestion`: SLOW_START
- `sem-fr · A/connection`: CLOSED → SYN_SENT → ESTABLISHED → FIN_WAIT_1 → FIN_WAIT_2 → TIME_WAIT → CLOSED
- `sem-fr · A/congestion`: SLOW_START → CONGESTION_AVOIDANCE
- `sem-fr · B/connection`: CLOSED → LISTEN → SYN_RCVD → ESTABLISHED → CLOSE_WAIT → LAST_ACK → CLOSED
- `sem-fr · B/congestion`: SLOW_START

| Métrica | com-fr | sem-fr |
|---|---|---|
| mensagens oferecidas | 10 | 10 |
| entregues (únicas) | 10 | 10 |
| perdidas para a aplicação | 0 | 0 |
| duplicadas entregues à aplicação | 0 | 0 |
| entregues em ordem | 10 | 10 |
| entregues fora de ordem | 0 | 0 |
| segmentos enviados | 28 | 28 |
| retransmissões | 1 | 1 |
| retransmissões rápidas | 1 | 0 |
| timeouts | 0 | 1 |
| perdidos no canal | 1 | 1 |
| corrompidos no canal | 0 | 0 |
| duplicados pelo canal | 0 | 0 |
| descartados pelo receptor | 0 | 0 |
| bytes no fio | 11560 | 11560 |
| tempo de transferência: 1ª oferta → última entrega (ms) | 460.48 | 665.989 |
| tempo total desde t=0, com handshake (ms) | 555.754 | 761.263 |
| economia da retransmissão rápida (ms) | 103.684 | 0 |

## 7 · Conexão a uma porta fechada — APROVADO

**Esperado (enunciado):** Resposta RST

B não chama listen(). O SYN de A chega a uma porta sem socket: o demultiplexador cria um TCB transitório em CLOSED, que responde RST,ACK. A sai de SYN_SENT direto para CLOSED (conexão recusada).

| Verificação | Execução | Resultado | Detalhe |
|---|---|---|---|
| B responde com RST |  | ✔ | observado: 1 |
| #transition[host=B de=CLOSED evento=rcv.SYN ação=send(RST)] = 1 |  | ✔ | observado: 1 |
| A percorre exatamente CLOSED→SYN_SENT→CLOSED |  | ✔ | observado: CLOSED→SYN_SENT→CLOSED |
| a aplicação de A é avisada da recusa |  | ✔ | observado: 1 |
| nenhuma retransmissão depois do RST |  | ✔ | observado: 0 |

**Caminhos de estados:**

- `A/connection`: CLOSED → SYN_SENT → CLOSED
- `A/congestion`: SLOW_START
- `B/connection`: CLOSED
- `B/congestion`: SLOW_START

| Métrica | tcp |
|---|---|
| mensagens oferecidas | 0 |
| entregues (únicas) | 0 |
| perdidas para a aplicação | 0 |
| duplicadas entregues à aplicação | 0 |
| entregues em ordem | 0 |
| entregues fora de ordem | 0 |
| segmentos enviados | 2 |
| retransmissões | 0 |
| retransmissões rápidas | 0 |
| timeouts | 0 |
| perdidos no canal | 0 |
| corrompidos no canal | 0 |
| duplicados pelo canal | 0 |
| descartados pelo receptor | 0 |
| bytes no fio | 40 |
| tempo de transferência: 1ª oferta → última entrega (ms) | 0 |
| tempo total desde t=0, com handshake (ms) | 0 |
| economia da retransmissão rápida (ms) | 0 |

## 8 · 20 datagramas com 30% de perda — APROVADO

**Esperado (enunciado):** Perdidos não são recuperados e nenhuma retransmissão ocorre

A envia 20 datagramas espaçados de 10 ms por um canal com 30% de perda. O UDP não tem ACK nem temporizador: o que se perde fica perdido.

| Verificação | Execução | Resultado | Detalhe |
|---|---|---|---|
| sent = 20 |  | ✔ | observado: 20 |
| lost ≥ 1 |  | ✔ | observado: 4 |
| cada perda vira uma mensagem a menos na aplicação |  | ✔ | observado: 4 |
| retransmissions = 0 |  | ✔ | observado: 0 |
| timeouts = 0 |  | ✔ | observado: 0 |
| B nunca envia nada de volta |  | ✔ | observado: 0 |

**Caminhos de estados:**

- `A/socket`: READY
- `B/socket`: READY

| Métrica | udp |
|---|---|
| mensagens oferecidas | 20 |
| entregues (únicas) | 16 |
| perdidas para a aplicação | 4 |
| duplicadas entregues à aplicação | 0 |
| entregues em ordem | 16 |
| entregues fora de ordem | 0 |
| segmentos enviados | 20 |
| retransmissões | 0 |
| retransmissões rápidas | 0 |
| timeouts | 0 |
| perdidos no canal | 4 |
| corrompidos no canal | 0 |
| duplicados pelo canal | 0 |
| descartados pelo receptor | 0 |
| bytes no fio | 4160 |
| tempo de transferência: 1ª oferta → última entrega (ms) | 237.982 |
| tempo total desde t=0, com handshake (ms) | 237.982 |
| economia da retransmissão rápida (ms) | 0 |

## 8b · Duplicação e reordenação chegam à aplicação — APROVADO

**Esperado (enunciado):** Duplicados e fora de ordem são entregues como chegaram

Complemento do item 4.2: com duplicação e reordenação no canal, o receptor UDP entrega à aplicação cópias repetidas e mensagens fora de ordem, sem qualquer correção.

| Verificação | Execução | Resultado | Detalhe |
|---|---|---|---|
| duplicates ≥ 1 |  | ✔ | observado: 4 |
| outOfOrder ≥ 1 |  | ✔ | observado: 2 |
| retransmissions = 0 |  | ✔ | observado: 0 |
| toda duplicata do canal chega à aplicação |  | ✔ | observado: 4 |

**Caminhos de estados:**

- `A/socket`: READY
- `B/socket`: READY

| Métrica | udp |
|---|---|
| mensagens oferecidas | 20 |
| entregues (únicas) | 17 |
| perdidas para a aplicação | 3 |
| duplicadas entregues à aplicação | 4 |
| entregues em ordem | 15 |
| entregues fora de ordem | 2 |
| segmentos enviados | 20 |
| retransmissões | 0 |
| retransmissões rápidas | 0 |
| timeouts | 0 |
| perdidos no canal | 3 |
| corrompidos no canal | 0 |
| duplicados pelo canal | 4 |
| descartados pelo receptor | 0 |
| bytes no fio | 4160 |
| tempo de transferência: 1ª oferta → última entrega (ms) | 279.159 |
| tempo total desde t=0, com handshake (ms) | 279.159 |
| economia da retransmissão rápida (ms) | 0 |

## 9 · Datagrama com checksum corrompido — APROVADO

**Esperado (enunciado):** Descartado pelo receptor, sem notificação

O canal troca um bit do 3º datagrama de A. O receptor recalcula a soma em complemento de 1 sobre pseudo-cabeçalho, cabeçalho e dados, percebe a divergência e descarta o datagrama em silêncio.

| Verificação | Execução | Resultado | Detalhe |
|---|---|---|---|
| corrupted = 1 |  | ✔ | observado: 1 |
| B descarta por checksum |  | ✔ | observado: 1 |
| #transition[host=B evento=rcv ação=discard(checksum)] = 1 |  | ✔ | observado: 1 |
| delivered = 4 |  | ✔ | observado: 4 |
| o remetente não é notificado |  | ✔ | observado: 0 |

**Caminhos de estados:**

- `A/socket`: READY
- `B/socket`: READY

| Métrica | udp |
|---|---|
| mensagens oferecidas | 5 |
| entregues (únicas) | 4 |
| perdidas para a aplicação | 1 |
| duplicadas entregues à aplicação | 0 |
| entregues em ordem | 4 |
| entregues fora de ordem | 0 |
| segmentos enviados | 5 |
| retransmissões | 0 |
| retransmissões rápidas | 0 |
| timeouts | 0 |
| perdidos no canal | 0 |
| corrompidos no canal | 1 |
| duplicados pelo canal | 0 |
| descartados pelo receptor | 1 |
| bytes no fio | 1040 |
| tempo de transferência: 1ª oferta → última entrega (ms) | 86.97 |
| tempo total desde t=0, com handshake (ms) | 86.97 |
| economia da retransmissão rápida (ms) | 0 |

## 10 · Mesma carga, mesmo canal ruidoso — APROVADO

**Esperado (enunciado):** Relatório comparando entregues, perdidos, duplicados, em ordem e tempo total

30 mensagens de 500 B pelo mesmo canal (15% de perda, 5% de corrupção, 5% de duplicação, 10% de reordenação, atraso de 30 a 70 ms) e a mesma seed. Cada componente aleatório do canal tem seu próprio gerador derivado da seed.

| Verificação | Execução | Resultado | Detalhe |
|---|---|---|---|
| delivered = 30 | tcp | ✔ | observado: 30 |
| duplicates = 0 | tcp | ✔ | observado: 0 |
| outOfOrder = 0 | tcp | ✔ | observado: 0 |
| missing ≥ 1 | udp | ✔ | observado: 5 |
| retransmissions = 0 | udp | ✔ | observado: 0 |
| duplicatas do canal chegam à aplicação UDP | udp | ✔ | observado: 2 |
| a aplicação UDP vê mensagens fora de ordem | udp | ✔ | observado: 3 |
| UDP entrega menos que TCP | udp | ✔ | observado: 25 |
| TCP paga com mais segmentos no fio | tcp | ✔ | observado: 46 |

**Caminhos de estados:**

- `udp · A/socket`: READY
- `udp · B/socket`: READY
- `tcp · A/connection`: CLOSED → SYN_SENT → ESTABLISHED → FIN_WAIT_1 → FIN_WAIT_2 → TIME_WAIT → CLOSED
- `tcp · A/congestion`: SLOW_START → CONGESTION_AVOIDANCE → SLOW_START → CONGESTION_AVOIDANCE → FAST_RECOVERY → CONGESTION_AVOIDANCE → SLOW_START → CONGESTION_AVOIDANCE → SLOW_START
- `tcp · B/connection`: CLOSED → LISTEN → SYN_RCVD → ESTABLISHED → CLOSE_WAIT → LAST_ACK → CLOSED
- `tcp · B/congestion`: SLOW_START

| Métrica | udp | tcp |
|---|---|---|
| mensagens oferecidas | 30 | 30 |
| entregues (únicas) | 25 | 30 |
| perdidas para a aplicação | 5 | 0 |
| duplicadas entregues à aplicação | 2 | 0 |
| entregues em ordem | 22 | 30 |
| entregues fora de ordem | 3 | 0 |
| segmentos enviados | 30 | 46 |
| retransmissões | 0 | 7 |
| retransmissões rápidas | 0 | 1 |
| timeouts | 0 | 6 |
| perdidos no canal | 2 | 5 |
| corrompidos no canal | 3 | 3 |
| duplicados pelo canal | 2 | 2 |
| descartados pelo receptor | 3 | 5 |
| bytes no fio | 15240 | 21920 |
| tempo de transferência: 1ª oferta → última entrega (ms) | 361.561 | 2178.751 |
| tempo total desde t=0, com handshake (ms) | 361.561 | 2294.07 |
| economia da retransmissão rápida (ms) | 0 | 22.966 |

## 11 · Sem TIME_WAIT, o último ACK perdido vira RST — APROVADO

**Esperado (enunciado):** Com TIME_WAIT, A reconfirma o FIN repetido; sem TIME_WAIT, A responde RST e B aborta

Cenário extra para a pergunta 1. O encerramento do cenário 4 é repetido, mas o ACK final de A (2º ACK puro) se perde. B, em LAST_ACK, reenvia o FIN após o RTO. A execução sem-tw usa a variante tcp-sem-time-wait.json, idêntica exceto pela remoção do estado TIME_WAIT.

| Verificação | Execução | Resultado | Detalhe |
|---|---|---|---|
| B reenvia o FIN | com-tw | ✔ | observado: 1 |
| A, ainda em TIME_WAIT, confirma de novo | com-tw | ✔ | observado: 1 |
| B visita CLOSE_WAIT→LAST_ACK→CLOSED | com-tw | ✔ | observado: CLOSED→LISTEN→SYN_RCVD→ESTABLISHED→CLOSE_WAIT→LAST_ACK→CLOSED |
| B fecha de forma ordenada | com-tw | ✔ | observado: 1 |
| #send[flags=RST] = 0 | com-tw | ✔ | observado: 0 |
| A já está em CLOSED e responde RST | sem-tw | ✔ | observado: 1 |
| B aborta em vez de fechar | sem-tw | ✔ | observado: 1 |
| #app[host=B evento=reset] = 1 | sem-tw | ✔ | observado: 1 |

**Caminhos de estados:**

- `com-tw · A/connection`: CLOSED → SYN_SENT → ESTABLISHED → FIN_WAIT_1 → FIN_WAIT_2 → TIME_WAIT → CLOSED
- `com-tw · A/congestion`: SLOW_START
- `com-tw · B/connection`: CLOSED → LISTEN → SYN_RCVD → ESTABLISHED → CLOSE_WAIT → LAST_ACK → CLOSED
- `com-tw · B/congestion`: SLOW_START
- `sem-tw · A/connection`: CLOSED → SYN_SENT → ESTABLISHED → FIN_WAIT_1 → FIN_WAIT_2 → CLOSED
- `sem-tw · A/congestion`: SLOW_START
- `sem-tw · B/connection`: CLOSED → LISTEN → SYN_RCVD → ESTABLISHED → CLOSE_WAIT → LAST_ACK → CLOSED
- `sem-tw · B/congestion`: SLOW_START

| Métrica | com-tw | sem-tw |
|---|---|---|
| mensagens oferecidas | 0 | 0 |
| entregues (únicas) | 0 | 0 |
| perdidas para a aplicação | 0 | 0 |
| duplicadas entregues à aplicação | 0 | 0 |
| entregues em ordem | 0 | 0 |
| entregues fora de ordem | 0 | 0 |
| segmentos enviados | 9 | 9 |
| retransmissões | 1 | 1 |
| retransmissões rápidas | 0 | 0 |
| timeouts | 1 | 1 |
| perdidos no canal | 1 | 1 |
| corrompidos no canal | 0 | 0 |
| duplicados pelo canal | 0 | 0 |
| descartados pelo receptor | 0 | 0 |
| bytes no fio | 180 | 180 |
| tempo de transferência: 1ª oferta → última entrega (ms) | 0 | 0 |
| tempo total desde t=0, com handshake (ms) | 0 | 0 |
| economia da retransmissão rápida (ms) | 0 | 0 |

## 12 · rdt 2.1 sobre canal com erros de bits — APROVADO

**Esperado (enunciado):** Todas as mensagens entregues uma vez, em ordem, só com NAK e reenvio

Prova de extensibilidade: rdt-2.1.json roda com o mesmo motor e o mesmo canal, sem código específico da versão. 30% dos pacotes chegam corrompidos.

| Verificação | Execução | Resultado | Detalhe |
|---|---|---|---|
| inOrder = 10 |  | ✔ | observado: 10 |
| duplicates = 0 |  | ✔ | observado: 0 |
| #send[host=B flags exatas=NAK] ≥ 1 |  | ✔ | observado: 4 |
| retransmissions ≥ 1 |  | ✔ | observado: 8 |

**Caminhos de estados:**

- `A/sender`: WAIT_CALL_0 → WAIT_ACK_0 → WAIT_CALL_1 → WAIT_ACK_1 → WAIT_CALL_0 → WAIT_ACK_0 → WAIT_CALL_1 → WAIT_ACK_1 → WAIT_CALL_0 → WAIT_ACK_0 → WAIT_CALL_1 → WAIT_ACK_1 → WAIT_CALL_0 → WAIT_ACK_0 → WAIT_CALL_1 → WAIT_ACK_1 → WAIT_CALL_0 → WAIT_ACK_0 → WAIT_CALL_1 → WAIT_ACK_1 → WAIT_CALL_0
- `B/receiver`: WAIT_0 → WAIT_1 → WAIT_0 → WAIT_1 → WAIT_0 → WAIT_1 → WAIT_0 → WAIT_1 → WAIT_0 → WAIT_1 → WAIT_0

| Métrica | rdt-2.1 |
|---|---|
| mensagens oferecidas | 10 |
| entregues (únicas) | 10 |
| perdidas para a aplicação | 0 |
| duplicadas entregues à aplicação | 0 |
| entregues em ordem | 10 |
| entregues fora de ordem | 0 |
| segmentos enviados | 36 |
| retransmissões | 8 |
| retransmissões rápidas | 0 |
| timeouts | 0 |
| perdidos no canal | 0 |
| corrompidos no canal | 10 |
| duplicados pelo canal | 0 |
| descartados pelo receptor | 0 |
| bytes no fio | 1296 |
| tempo de transferência: 1ª oferta → última entrega (ms) | 831.985 |
| tempo total desde t=0, com handshake (ms) | 833.985 |
| economia da retransmissão rápida (ms) | 0 |

## 13 · rdt 3.0 sobre canal com perdas — APROVADO

**Esperado (enunciado):** Temporizador recupera perdas sem NAK

rdt-3.0.json foi escrito só em JSON, reaproveitando o vocabulário do rdt 2.1 (make_pkt, udt_send, start_timer…). 20% de perda e 10% de corrupção.

| Verificação | Execução | Resultado | Detalhe |
|---|---|---|---|
| inOrder = 10 |  | ✔ | observado: 10 |
| duplicates = 0 |  | ✔ | observado: 0 |
| timeouts ≥ 1 |  | ✔ | observado: 1 |
| #send[flags exatas=NAK] = 0 |  | ✔ | observado: 0 |

**Caminhos de estados:**

- `A/sender`: WAIT_CALL_0 → WAIT_ACK_0 → WAIT_CALL_1 → WAIT_ACK_1 → WAIT_CALL_0 → WAIT_ACK_0 → WAIT_CALL_1 → WAIT_ACK_1 → WAIT_CALL_0 → WAIT_ACK_0 → WAIT_CALL_1 → WAIT_ACK_1 → WAIT_CALL_0 → WAIT_ACK_0 → WAIT_CALL_1 → WAIT_ACK_1 → WAIT_CALL_0 → WAIT_ACK_0 → WAIT_CALL_1 → WAIT_ACK_1 → WAIT_CALL_0
- `B/receiver`: WAIT_0 → WAIT_1 → WAIT_0 → WAIT_1 → WAIT_0 → WAIT_1 → WAIT_0 → WAIT_1 → WAIT_0 → WAIT_1 → WAIT_0

| Métrica | rdt-3.0 |
|---|---|
| mensagens oferecidas | 10 |
| entregues (únicas) | 10 |
| perdidas para a aplicação | 0 |
| duplicadas entregues à aplicação | 0 |
| entregues em ordem | 10 |
| entregues fora de ordem | 0 |
| segmentos enviados | 22 |
| retransmissões | 2 |
| retransmissões rápidas | 0 |
| timeouts | 1 |
| perdidos no canal | 1 |
| corrompidos no canal | 0 |
| duplicados pelo canal | 0 |
| descartados pelo receptor | 0 |
| bytes no fio | 792 |
| tempo de transferência: 1ª oferta → última entrega (ms) | 729.329 |
| tempo total desde t=0, com handshake (ms) | 731.329 |
| economia da retransmissão rápida (ms) | 0 |
