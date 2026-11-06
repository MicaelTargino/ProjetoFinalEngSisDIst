# ADR-001: Um único dono por ativo para executar ordens (consistência na escrita)

- **Status:** aceito
- **Data:** 06/11/2026
- **Decisores:** equipe completa

## Contexto

O preço de um ativo é o preço da última negociação. Cada ordem casada muda o estado que a próxima ordem vai ler (o livro de ofertas e o preço). Com várias instâncias recebendo ordens do mesmo ativo ao mesmo tempo, qualquer esquema de "ler, calcular e gravar" sem coordenação perde atualizações. A equipe vai medir esse efeito com um grupo de controle (`naive-service`) que atualiza o preço sem coordenação.

Pelo teorema CAP, quando há partição de rede o sistema precisa escolher entre recusar operações (consistência) ou aceitá-las com risco de divergência (disponibilidade). Para a execução de ordens, divergência significa dois preços diferentes para o mesmo ativo, ou o mesmo saldo gasto duas vezes. Isso é inaceitável numa plataforma financeira.

## Decisão

A execução de ordens é **CP**: cada ativo tem exatamente uma instância do motor responsável por ele, em qualquer momento.

- O tópico `orders` do Kafka é particionado pelo código do ativo (chave da mensagem). Todas as ordens de um ativo caem na mesma partição, venham de qual réplica do `order-service` vierem.
- O `matching-engine` roda como *consumer group*. O Kafka garante que cada partição tem um único consumidor ativo no grupo.
- Dentro da instância, cada partição é processada sequencialmente por uma única goroutine. O livro de ofertas não tem locks: a ordem de chegada no log **é** a ordem de execução.
- A paralelização acontece entre ativos: 6 partições e 3 instâncias permitem processar ativos diferentes em paralelo.

## Alternativas consideradas

| Alternativa | Por que foi descartada |
| --- | --- |
| Lock distribuído por ativo (Redis/Redlock) | Cada ordem pagaria uma ida e volta ao lock; Redlock não é seguro sob pausas de GC e falhas de relógio; um lock perdido volta ao problema de atualização perdida. |
| `UPDATE ... SET price = price + x` no banco (modo `atomic` do experimento) | Correto, mas transforma o banco no gargalo de todas as escritas de todos os ativos e não suporta a lógica do livro de ofertas (casamento de várias ordens). |
| Consenso por ativo (Raft) | Garantias fortes, mas complexidade alta para o escopo do projeto; o Kafka já entrega a ordenação total por partição. |
| Aceitar ordens em qualquer réplica e reconciliar depois (AP) | Exigiria desfazer negociações já confirmadas ao cliente. Inaceitável para dinheiro. |

## Consequências

**Positivas**

- Zero atualizações perdidas por construção, sem locks no caminho crítico.
- O processamento de um ativo é determinístico, o que viabiliza o failover por replay (ADR-004).
- Escala horizontalmente por número de ativos (partições).

**Negativas e custos aceitos**

- **Disponibilidade:** se o dono de uma partição cai, os ativos daquela partição ficam sem execução até o rebalance do grupo (meta: menos de 15 s). As ordens não se perdem: ficam no log e são executadas depois.
- **Ativo "quente":** um único ativo não escala além de uma goroutine. Meta de 2.000 ordens/s é distribuída em 10 ativos.
- O número de partições limita o paralelismo e é difícil de mudar depois (muda o mapeamento ativo → partição). Escolhemos 6 com folga para 3 instâncias.
- Se o Kafka inteiro fica inacessível, o `order-service` acumula ordens no outbox e, acima de `MAX_OUTBOX_BACKLOG`, passa a recusar novas ordens com 503 (*load shedding*), em vez de prometer uma execução que não sabe quando acontecerá.

## Como será validado

- Teste unitário `TestFailoverContinuesWhereOldOwnerStopped` (determinismo).
- Teste de integração `TestConcurrentOrdersKeepInvariants`: conservação de saldo e sequência sem lacunas.
- Experimento comparativo `make experiment` (naive x atomic x arquitetura proposta).
- Teste de resiliência `make chaos-engine`: tempo até todas as partições terem dono de novo.
