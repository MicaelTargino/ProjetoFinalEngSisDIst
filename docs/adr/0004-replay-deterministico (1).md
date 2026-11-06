# ADR-004: Estado do motor reconstruído por replay determinístico (Event Sourcing)

- **Status:** aceito
- **Data:** 06/11/2026

## Contexto

O livro de ofertas vive na memória do motor. Quando uma instância cai e outra assume a partição (ADR-001), a nova dona precisa do livro exatamente como estava. Persistir o livro a cada ordem num banco recolocaria um sistema externo no caminho crítico.

## Decisão

- O tópico `orders` é o **log de eventos** e a fonte da verdade do motor. Ele tem retenção infinita (`retention.ms=-1`).
- O motor é **determinístico**: o resultado depende só da sequência de ordens. O ID de cada negociação é `p<partição>-o<offset>-<n>`, e o `seq` é um contador por ativo.
- Ao receber uma partição, a instância obtém o offset confirmado pelo dono anterior e relê a partição desde o início:
  - antes desse offset: **replay silencioso**, que reconstrói o livro sem publicar nada;
  - a partir dele: processamento normal.
- O offset só é confirmado **depois** que as negociações foram publicadas. Se o processo cair entre publicar e confirmar, a nova dona reprocessa a ordem e publica negociações **com os mesmos IDs**. Os consumidores descartam essas duplicatas (`settled_trades` e o `seq` no Redis).

O resultado é um efeito *exactly-once* com um mecanismo *at-least-once*: determinismo mais consumidores idempotentes.

## Alternativas consideradas

| Alternativa | Por que foi descartada |
| --- | --- |
| Persistir o livro no PostgreSQL ou Redis a cada ordem | Latência e um ponto de contenção no caminho crítico. |
| Transações do Kafka (*exactly-once semantics*) | Viável, mas aumenta o acoplamento com a configuração do broker. Idempotência nos consumidores é mais simples de testar. |
| Snapshots periódicos | Necessários a longo prazo (ver consequências), mas não para o volume desta entrega. |

## Consequências

**Positivas**

- Failover sem perda e sem divergência, demonstrável por teste unitário.
- Auditoria completa: qualquer preço pode ser recalculado a partir do log.

**Negativas**

- O tempo de replay cresce com o tamanho do log. Na Entrega 3 avaliaremos snapshots do livro por partição (offset + estado) para limitar esse tempo.
- O conjunto de IDs de ordens já vistas (deduplicação) também cresce com o log; os snapshots resolvem os dois problemas.
- Qualquer não-determinismo no motor (relógio, mapa iterado sem ordem, número aleatório) quebraria a garantia. O código evita isso e o teste de replay protege contra regressões.

## Como será validado

`TestReplayProducesIdenticalTrades`, `TestFailoverContinuesWhereOldOwnerStopped`, `TestSeqHasNoGapsPerSymbol` e `TestDuplicateOrderIsIgnored`. Na Entrega 3, `make chaos-engine` mede o tempo de failover com carga.
