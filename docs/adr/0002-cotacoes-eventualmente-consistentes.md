# ADR-002: Cotações com consistência eventual via CQRS (disponibilidade na leitura)

- **Status:** aceito
- **Data:** 06/11/2026

## Contexto

A leitura de cotações tem um perfil oposto ao da execução: muito mais leituras do que escritas, muitos clientes conectados ao mesmo tempo, e tolerância a alguns milissegundos de atraso. Se as leituras dependessem do motor (CP), a queda de uma instância do motor também derrubaria as telas de todos os traders.

## Decisão

Separar escrita e leitura (**CQRS**) e tratar a leitura como **AP**:

- O motor publica cada negociação no tópico `trades`, com um `seq` por ativo que cresce sem lacunas.
- O `quote-service` consome `trades` e mantém uma **visão materializada** no Redis (`quote:<ativo>`: preço, última quantidade, volume, seq).
- A atualização é feita por um script Lua atômico que só aplica a negociação se `seq` for maior que o atual. Isso torna a escrita idempotente e imune a mensagens repetidas ou fora de ordem.
- O mesmo script publica a cotação no canal Pub/Sub `quotes`. Todas as réplicas do `quote-service` recebem e repassam para os clientes WebSocket (*fan-out*).
- Cada réplica mantém também um cache em memória. Se o Redis cair, `GET /quotes/{ativo}` responde com o último valor conhecido e os cabeçalhos `X-Quote-Source: local-cache` e `X-Quote-Stale: true`.

## Alternativas consideradas

| Alternativa | Por que foi descartada |
| --- | --- |
| Clientes leem o preço direto do motor | Acopla leitura à disponibilidade do motor e coloca carga de leitura no caminho crítico. |
| Ler do PostgreSQL (tabela `settled_trades`) | Consistente, mas concorre com as escritas transacionais e não oferece Pub/Sub de baixa latência. |
| Cache-aside com TTL | Não garante ordem: um valor antigo pode sobrescrever um novo. A checagem de `seq` resolve isso. |

## Consequências

**Positivas**

- Cotações continuam disponíveis com motor, Kafka ou Redis fora (com o último valor conhecido e sinalização de desatualização).
- Escala de leitura independente da escrita: basta adicionar réplicas do `quote-service`.

**Negativas**

- Consistência eventual: um cliente pode ver a cotação alguns milissegundos depois da negociação. Meta: p95 abaixo de 500 ms (métrica `quote_propagation_seconds`).
- Clientes WebSocket lentos são desconectados quando o buffer de 64 mensagens enche (*backpressure*), para não atrasar os demais.
- Redis sem persistência: após reinício, a visão é reconstruída à medida que novas negociações chegam. Na Entrega 3 avaliaremos reconstruir a partir do tópico `trades`.

## Como será validado

- Teste `TestCacheIgnoresStaleAndDuplicateQuotes` e `TestApplyScriptIsMonotonicAndPublishes` (Redis real).
- Teste manual: com o Redis desligado, a API deve responder 200 com `X-Quote-Stale: true`.
- Teste de partição com Toxiproxy (Entrega 3).
