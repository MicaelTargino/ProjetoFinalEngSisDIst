.PHONY: up up-app down logs ps test fuzz lint topics

up:              ## sobe infraestrutura, gateway e observabilidade
	@test -f .env || cp .env.example .env
	docker compose up -d

up-app:          ## inclui os serviços da aplicação (quando o código existir)
	@test -f .env || cp .env.example .env
	docker compose --profile app up -d --build

down:            ## derruba o ambiente e apaga os volumes
	docker compose --profile app down -v

logs:
	docker compose logs -f --tail=100

ps:
	docker compose ps

topics:          ## lista os tópicos do Kafka
	docker compose exec kafka /opt/kafka/bin/kafka-topics.sh --bootstrap-server localhost:9092 --describe

test:            ## testes unitários
	go test -race -cover ./...

fuzz:
	go test -run=^$$ -fuzz=FuzzBookNeverCrosses -fuzztime=60s ./internal/orderbook

lint:
	gofmt -l . && go vet ./...
