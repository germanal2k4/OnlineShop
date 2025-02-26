.PHONY: deps build run lint install-tools all

deps:
	@go mod tidy

install-tools:
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

mlint: install-tools
	golangci-lint run

build: deps
	go build -o myapp ./cmd

run: build
	./myapp

all: lint build run
