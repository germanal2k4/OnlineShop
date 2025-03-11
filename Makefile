.PHONY: install-tools migrate-up migrate-down build run

DSN ?= "host=localhost user=postgres password=postgres dbname=pickups sslmode=disable"
MIGRATIONS_DIR := migrations

install-tools:
	@echo "Устанавливаем goose..."
	go install github.com/pressly/goose/v3/cmd/goose@latest

migrate-up:
	@echo "Накатываем миграции..."
	goose -dir=$(MIGRATIONS_DIR) postgres $(DSN) up

migrate-down:
	@echo "Откатываем миграции..."
	goose -dir=$(MIGRATIONS_DIR) postgres $(DSN) down

build:
	go build -o myapp ./cmd

test:
	go test ./... -v

run: build
	./myapp
