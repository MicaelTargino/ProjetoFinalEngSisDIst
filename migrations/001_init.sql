-- Esquema do order-service (carteiras, ordens, outbox, liquidação) e do
-- experimento naive. Executado pelo Postgres na primeira inicialização.

CREATE TABLE users (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Invariante de ACID: nenhuma carteira fica negativa, garantido pelo banco.
CREATE TABLE wallets (
    user_id   TEXT   NOT NULL REFERENCES users (id),
    asset     TEXT   NOT NULL,
    available BIGINT NOT NULL CHECK (available >= 0),
    locked    BIGINT NOT NULL DEFAULT 0 CHECK (locked >= 0),
    PRIMARY KEY (user_id, asset)
);

CREATE TABLE orders (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         TEXT   NOT NULL REFERENCES users (id),
    client_order_id TEXT   NOT NULL,                       -- chave de idempotência
    symbol          TEXT   NOT NULL,
    side            TEXT   NOT NULL CHECK (side IN ('BUY', 'SELL')),
    price           BIGINT NOT NULL CHECK (price > 0),     -- centavos
    qty             BIGINT NOT NULL CHECK (qty > 0),
    filled_qty      BIGINT NOT NULL DEFAULT 0 CHECK (filled_qty <= qty),
    status          TEXT   NOT NULL DEFAULT 'NEW',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, client_order_id)
);
CREATE INDEX orders_user_created ON orders (user_id, created_at DESC);

-- Transactional Outbox: gravado na mesma transação da ordem.
CREATE TABLE outbox (
    id         BIGSERIAL PRIMARY KEY,
    topic      TEXT NOT NULL,
    key        TEXT NOT NULL,
    payload    JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Deduplicação da liquidação: cada negociação é aplicada uma única vez.
CREATE TABLE settled_trades (
    trade_id    TEXT PRIMARY KEY,
    symbol      TEXT   NOT NULL,
    seq         BIGINT NOT NULL,
    price       BIGINT NOT NULL,
    qty         BIGINT NOT NULL,
    buyer_id    TEXT   NOT NULL,
    seller_id   TEXT   NOT NULL,
    executed_at TIMESTAMPTZ NOT NULL,
    settled_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (symbol, seq)
);

-- Grupo de controle do experimento (sem coordenação).
CREATE TABLE naive_assets (
    symbol        TEXT PRIMARY KEY,
    price         BIGINT NOT NULL,
    initial_price BIGINT NOT NULL,
    trade_count   BIGINT NOT NULL DEFAULT 0
);

-- ---------- Dados de demonstração ----------
INSERT INTO users (id, name) VALUES
    ('alice', 'Alice'), ('bob', 'Bob'), ('carol', 'Carol'), ('dave', 'Dave');

-- Cada usuário começa com R$ 1.000.000,00 e 1.000 unidades de cada ativo.
INSERT INTO wallets (user_id, asset, available)
SELECT u.id, a.asset, a.amount
FROM users u
CROSS JOIN (VALUES ('BRL', 100000000), ('BTC', 1000), ('ETH', 1000), ('SOL', 1000)) AS a (asset, amount);

-- Usuários dos testes de carga, com saldo alto para não esgotar durante o k6.
INSERT INTO users (id, name)
SELECT format('load%s', lpad(n::text, 2, '0')), format('Carga %s', n) FROM generate_series(1, 20) AS n;

INSERT INTO wallets (user_id, asset, available)
SELECT u.id, a.asset, a.amount
FROM users u
CROSS JOIN (VALUES ('BRL', 1000000000000), ('BTC', 1000000000), ('ETH', 1000000000), ('SOL', 1000000000)) AS a (asset, amount)
WHERE u.id LIKE 'load%';

INSERT INTO naive_assets (symbol, price, initial_price) VALUES
    ('BTC', 35000000, 35000000), ('ETH', 1800000, 1800000);