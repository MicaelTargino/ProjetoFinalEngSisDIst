# Plataforma de Negociação de Ativos

Projeto Final de Engenharia de Sistemas Distribuídos — 2026.2

Plataforma de negociação de ativos (no estilo das exchanges de criptomoedas) em que **cada negociação atualiza o preço do ativo**. O sistema precisa manter um preço correto mesmo recebendo ordens simultâneas do mesmo ativo em servidores diferentes, e explora na prática o dilema **consistência x disponibilidade** (teorema CAP).

> Status: **Projeto 01 — definição do grupo e tema** (09/10/2026)

## Problema

Com vários servidores processando ordens do mesmo ativo ao mesmo tempo, surgem:

- **Atualização perdida:** duas negociações paralelas leem o mesmo preço e uma sobrescreve a outra.
- **Preços divergentes:** clientes em servidores diferentes veem valores diferentes.
- **Saldo gasto duas vezes:** ordens simultâneas usam o mesmo saldo.

## Decisão central: consistência x disponibilidade

| Caminho | Escolha | Como |
| --- | --- | --- |
| Execução de ordens e preço | Consistência (CP) | Cada ativo tem um único motor dono, que processa as ordens em sequência |
| Saldo das carteiras | Consistência (ACID) | Reserva de saldo em transação no PostgreSQL |
| Consulta de cotações | Disponibilidade (AP) | Cache em Redis atualizado por eventos |

Custo aceito: se o motor dono de um ativo cair, **só aquele ativo** fica sem negociação por alguns segundos, até o Kafka redistribuir a partição.

## Arquitetura (visão inicial)

```
Cliente → API Gateway (JWT + rate limit) → Serviço de Ordens (reserva saldo, PostgreSQL + outbox)
                                                   │
                                                   ▼
                          Kafka: ordens particionadas por ativo      ┐
                            │              │              │          │ CP
                        Motor 1        Motor 2        Motor 3        │
                            └──────────────┼──────────────┘          ┘
                                           ▼
            Kafka: negociações (Event Sourcing) → Serviço de Cotações (Redis) → Clientes (WebSocket)   AP
```

O diagrama C4 (níveis 1 e 2) será entregue no Projeto 02.

## Padrões arquiteturais previstos

| Padrão | Área |
| --- | --- |
| Database Sharding e Consistent Hashing | Escalabilidade |
| Strong vs Eventual Consistency | Confiabilidade |
| Event Sourcing | Confiabilidade |
| CQRS e Materialized View | Desempenho |
| Transactional Outbox | Confiabilidade |
| Queues, PubSub e Messaging Fanout | Escalabilidade |
| API Gateway (JWT) e Rate Limit | Segurança |

## Stack prevista

- **Serviços:** Go ou Java (Spring Boot), a definir no Projeto 02
- **API Gateway:** Kong ou Traefik
- **Eventos e particionamento:** Apache Kafka
- **Dados:** PostgreSQL (carteiras) e Redis (cotações)
- **Tempo real:** WebSocket
- **Observabilidade:** Prometheus, Grafana e Jaeger
- **Testes:** k6 (carga) e Toxiproxy (falhas de rede)
- **Infra:** Docker Compose e GitHub Actions

## Critérios de sucesso

| Critério | Meta |
| --- | --- |
| Zero atualizações perdidas | Preço final igual ao recalculado a partir do log de eventos |
| Comparação com versão ingênua | Versão sem coordenação perde atualizações; a proposta, zero |
| Saldo nunca negativo | 0 carteiras negativas com ordens simultâneas |
| Latência de execução | p95 < 100 ms |
| Vazão | 2.000 ordens/s em 10 ativos |
| Atraso da cotação | p95 < 500 ms até o cliente |
| Custo da consistência | Ativo volta a negociar em < 15 s após queda do seu motor; demais não são afetados |
| Partição de rede | Cotações continuam disponíveis; ordens afetadas são recusadas, nunca divergentes |

## Equipe

| Integrante | GitHub | Responsabilidade |
| --- | --- | --- |
| Nome 1 | @usuario | API Gateway, autenticação e carteiras |
| Nome 2 | @usuario | Motor de negociação (order book) |
| Nome 3 | @usuario | Particionamento por ativo e failover |
| Nome 4 | @usuario | Serviço de cotações, cache e WebSocket |
| Nome 5 | @usuario | Observabilidade, CI e testes de carga |

## Como executar

Em construção. O `docker-compose.yml` e as instruções completas serão adicionados no Projeto 02.

## Roadmap

- [x] **Projeto 01 (09/10):** grupo, tema e README inicial
- [ ] **Projeto 02 (06/11):** C4 níveis 1 e 2, ADRs iniciais (incluindo a decisão CP x AP), stack, Docker Compose e CI básico
- [ ] **Projeto 03 (11/12):** código completo, testes de carga/resiliência/integração e documentação final

## Ferramentas de IA utilizadas

| Ferramenta | Onde atuou | Como foi orientada |
| --- | --- | --- |
| Claude (Anthropic) | Rascunho da proposta do tema e deste README | Recebeu o documento de definição do projeto e a descrição do tema (preço atualizado a cada negociação, requisições concorrentes em servidores diferentes, consistência x disponibilidade) |

**Avaliação:** o rascunho foi revisado pela equipe. _Descrever aqui o que foi mantido, corrigido ou descartado._ Esta seção será atualizada a cada entrega.
