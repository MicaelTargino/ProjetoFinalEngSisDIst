# ADR-003: Reserva de saldo ACID e publicação com Transactional Outbox

- **Status:** aceito
- **Data:** 06/11/2026

## Contexto

Antes de uma ordem entrar no motor, o saldo precisa estar garantido. Duas ordens simultâneas do
mesmo usuário, recebidas por réplicas diferentes, não podem gastar o mesmo dinheiro. Além disso,
gravar a ordem no banco e publicá-la no Kafka são duas operações em sistemas diferentes: se o
processo cair entre elas, ou a ordem existe sem ter sido publicada (dinheiro bloqueado para sempre),
ou foi publicada sem existir.

## Decisão

1. **Reserva atômica no PostgreSQL**, sem `SELECT ... FOR UPDATE`:
   `UPDATE wallets SET available = available - x, locked = locked + x WHERE user_id = $1 AND asset = $2 AND available >= x`.
   Se nenhuma linha for afetada, a ordem é recusada com 422. A restrição `CHECK (available >= 0)`
   é a última linha de defesa.
2. **Transactional Outbox:** a ordem, a reserva e o evento (tabela `outbox`) são gravados na mesma
   transação. Um relay em cada réplica lê o outbox com `FOR UPDATE SKIP LOCKED`, publica no Kafka
   e apaga as linhas publicadas.
3. **Idempotência na entrada:** o cliente envia `Idempotency-Key`. A restrição
   `UNIQUE (user_id, client_order_id)` faz uma requisição repetida devolver a ordem original (200)
   sem reservar de novo.
4. **Idempotência na saída:** o relay é at-least-once. O motor ignora ordens com ID já visto, e a
   liquidação ignora negociações já registradas em `settled_trades`.

## Alternativas consideradas

| Alternativa | Por que foi descartada |
|---|---|
| Publicar no Kafka e depois gravar no banco | Falha entre os dois passos gera ordem fantasma ou dinheiro bloqueado. |
| Transação distribuída (2PC) | Kafka não participa de XA; alto acoplamento e latência. |
| CDC com Debezium lendo o outbox | Mais robusto em produção, mas adiciona Kafka Connect ao ambiente. Fica como evolução. |
| SAGA com compensação | Útil quando há vários serviços com estado; aqui a reserva e o registro estão no mesmo banco, então uma transação local basta. |

## Consequências

### Positivas

- Impossível gastar o mesmo saldo duas vezes, mesmo com várias réplicas (validado com 200
  requisições simultâneas).
- Nenhuma ordem aceita se perde: se o Kafka estiver fora, ela espera no outbox.

### Negativas

- A execução é assíncrona: o cliente recebe `202 Accepted` com status `NEW` e acompanha o
  resultado por `GET /orders/{id}`.
- A liquidação também é assíncrona (consistência eventual das carteiras após a negociação), mas
  é idempotente e nunca diverge.
- O relay faz polling a cada 20 ms, o que soma até 20 ms de latência e algumas consultas por
  segundo ao banco.

## Validação

Testes de integração com PostgreSQL real (executados): `TestConcurrentReservationsNeverOverspend`,
`TestIdempotencyKeyReservesOnce` e `TestSettlementIsCorrectAndIdempotent`.
