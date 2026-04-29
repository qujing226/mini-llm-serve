TARGET ?= http://127.0.0.1:8800
METRICS_URL ?= http://127.0.0.1:8801/metrics
CONCURRENCY ?= 100
REQUESTS ?= 1000
TIMEOUT_MS ?= 10000
DOCKER ?= docker
SERVER_IMAGE ?= mini-llm-server:local
EXECUTOR_IMAGE ?= mini-llm-executor:local

run:
	go run ./cmd/server/. --conf="server.toml"

run-no-batching:
	go run ./cmd/server/. --conf="./config/server-no-batching.toml"

run-dynamic-default:
	go run ./cmd/server/. --conf="./config/server-dynamic-default.toml"

run-dynamic-fastflush:
	go run ./cmd/server/. --conf="./config/server-dynamic-fastflush.toml"

bench-smoke:
	go run ./cmd/bench --mode smoke --target $(TARGET) --metrics-url $(METRICS_URL) --requests $(REQUESTS) --concurrency $(CONCURRENCY) --timeout-ms $(TIMEOUT_MS)

bench-no-batching:
	go run ./cmd/bench --mode baseline_no_batching --target $(TARGET) --metrics-url $(METRICS_URL) --requests $(REQUESTS) --concurrency $(CONCURRENCY) --timeout-ms $(TIMEOUT_MS)

bench-dynamic-default:
	go run ./cmd/bench --mode dynamic_default --target $(TARGET) --metrics-url $(METRICS_URL) --requests $(REQUESTS) --concurrency $(CONCURRENCY) --timeout-ms $(TIMEOUT_MS)

bench-dynamic-fastflush:
	go run ./cmd/bench --mode dynamic_fastflush --target $(TARGET) --metrics-url $(METRICS_URL) --requests $(REQUESTS) --concurrency $(CONCURRENCY) --timeout-ms $(TIMEOUT_MS)

docker-build-server:
	$(DOCKER) build -f docker/server.Dockerfile -t $(SERVER_IMAGE) .

docker-build-executor:
	$(DOCKER) build -f docker/executor.Dockerfile -t $(EXECUTOR_IMAGE) .

docker-build: docker-build-server docker-build-executor

docker-run-executor:
	$(DOCKER) run --rm -p 19991:19991 $(EXECUTOR_IMAGE)

docker-run-server:
	$(DOCKER) run --rm --network host -v "$(PWD)/server.toml:/etc/mini-llm/server.toml:ro" $(SERVER_IMAGE) --conf=/etc/mini-llm/server.toml
