GOCACHE ?= $(CURDIR)/.cache/go-build

.PHONY: test vet run list watch run-multilang

test:
	GOCACHE=$(GOCACHE) go test ./...

vet:
	GOCACHE=$(GOCACHE) go vet ./...

run:
	GOCACHE=$(GOCACHE) go run ./cmd/plugin-executor -plugins ./plugins -input-file ./examples/inputs/default.json

list:
	GOCACHE=$(GOCACHE) go run ./cmd/plugin-executor -plugins ./plugins -list

watch:
	GOCACHE=$(GOCACHE) go run ./cmd/plugin-executor -plugins ./plugins -watch -interval 2s

run-multilang:
	GOCACHE=$(GOCACHE) go run ./cmd/plugin-executor -plugins ./plugins -disable go.slow -enable python.uppercase,js.reverse -input-file ./examples/inputs/multilingual.json
