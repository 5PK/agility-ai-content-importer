APP_NAME := agility-ai-content-importer
BIN_DIR := dist
BIN := $(BIN_DIR)/$(APP_NAME)
PORT ?= 8080
OLLAMA_URL ?= http://localhost:11434
OLLAMA_MODEL ?= llama3.1
GO_FILES := $(shell find . -name '*.go' ! -name '*_templ.go')

.PHONY: help dev generate fmt test check build clean

help:
	@printf "Targets:\n"
	@printf "  make dev       Generate templ files and run the app on PORT=%s\n" "$(PORT)"
	@printf "  make generate  Generate Go code from templ templates\n"
	@printf "  make fmt       Format Go files\n"
	@printf "  make test      Run Go tests\n"
	@printf "  make check     Generate, format, and test\n"
	@printf "  make build     Generate and build the app binary\n"
	@printf "  make clean     Remove build output\n"

dev: generate
	PORT=$(PORT) OLLAMA_URL=$(OLLAMA_URL) OLLAMA_MODEL=$(OLLAMA_MODEL) go run .

generate:
	templ generate

fmt:
	gofmt -w $(GO_FILES)

test:
	go test ./...

check: generate fmt test

build: generate
	mkdir -p $(BIN_DIR)
	go build -o $(BIN) .

clean:
	rm -rf $(BIN_DIR)
