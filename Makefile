.DEFAULT_GOAL := binary
.PHONY: go-pb

PROTO_DIR := pb/
#PROTO_FILES := $(wildcard $(PROTO_DIR)*.proto)
PROTO_FILES := pb/enrichedflow.proto pb/legacyenrichedflow.proto

go-pb:
	protoc --go_out=. --go_opt=paths=source_relative $(PROTO_FILES)

binary:
	go build .

test:
	go test ./... -cover

debian-bullseye-binary:
	podman run --rm -v .:/src:Z golang:bullseye bash -c 'apt update; apt install -y libpcap-dev; cd /src; make'

bench:
	@go test -bench=. -benchtime=1ns ./segments/pass | grep "cpu:"
	@echo "results:"
	@go test -bench=. -run=Bench ./... | grep -E "^Bench" | awk '{fps = 1/(($$3)/1e9); sub(/Benchmark/, "", $$1); sub(/-.*/, "", $$1); printf("%15s: %8s ns/flow, %7.0f flows/s\n", tolower($$1), $$3, fps)}'
	@rm segments/output/sqlite/bench.sqlite

#pb/enrichedflow.pb.go: pb/enrichedflow.proto
#	protoc --go_out=. --go_opt=paths=source_relative pb/enrichedflow.proto