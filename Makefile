.PHONY: deps build run lint install-tools all

GOBIN := $(CURDIR)/bin
export GOBIN

deps:
	@echo "Обновление зависимостей..."
	go mod tidy

install-tools:
	@echo "Установка golangci-lint..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

lint: install-tools
	@echo "Запуск линтеров..."
	@$(GOBIN)/golangci-lint run

build: deps
	@echo "Сборка проекта..."
	@go build -o $(GOBIN)/myapp ./cmd

run: build
	@echo "Запуск приложения..."
	@$(GOBIN)/myapp

all: lint build run
