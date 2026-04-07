TARGET ?= http://127.0.0.1:8800
METRICS_URL ?= http://127.0.0.1:8801/metrics
CONCURRENCY ?= 100
REQUESTS ?= 1000
TIMEOUT_MS ?= 10000

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
