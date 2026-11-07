# ADR-005: Stack tecnológico (Go, Kafka, PostgreSQL, Redis, Kong)

- **Status:** aceito
- **Data:** 06/11/2026

## Contexto

A proposta da Entrega 1 deixou em aberto Go ou Java e Kong ou Traefik. A escolha precisa favorecer baixa latência, binários pequenos para containers, facilidade de testar concorrência e o tempo de aprendizado de uma equipe de 5 pessoas.

## Decisão

Adotar o stack da seção 3: Go 1.24, Apache Kafka 3.8 (KRaft) com o cliente franz-go, PostgreSQL 16, Redis 7, Kong 3.8 (DB-less), Prometheus, Grafana e Jaeger, testes com go test, k6 e Toxiproxy, e entrega com Docker Compose e GitHub Actions. A justificativa de cada escolha está na tabela daquela seção.

## Consequências


- Parte da equipe precisa aprender Go. Mitigação: pares misturando quem conhece com quem está aprendendo, e o esqueleto inicial do repositório serve de base.
- Um único broker Kafka no Compose é um ponto único de falha. É aceitável para desenvolvimento; na Entrega 3 avaliaremos 3 brokers com `min.insync.replicas=2` para o teste de resiliência.
- **Limitação de segurança assumida:** na primeira versão, o endpoint `/api/auth/token` emitirá tokens para usuários de demonstração sem senha. O segredo JWT de desenvolvimento está no `.env.example` e no `kong.yml`. Na Entrega 3: provedor OIDC (ex.: Keycloak) e segredo vindo de um cofre.
